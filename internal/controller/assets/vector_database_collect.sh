#!/bin/sh

# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# vector_database_collect.sh
#
# Collects vector database data and embedding models from a container image and
# copies them to a target directory.
#
# IMPORTANT: The vector database data must strictly adhere to the expected
# directory structure described below. Otherwise the script will fail.
#
# Expected Container Image Structure (Input):
# /rag/
# ├── vector_db/
# │   ├── vector-db-data-1/
# │   │   ├── faiss_store.db
# │   │   └── llama-stack.yaml
# │   └── vector-db-data-N/
# │       ├── faiss_store.db
# │       └── llama-stack.yaml
# ├── ocp_vector_db/
# │   ├── ocp_X.YZ/
# │   │   ├── faiss_store.db
# │   │   └── llama-stack.yaml
# │   └── ocp_latest/
# │       ├── faiss_store.db
# │       └── llama-stack.yaml
# └── embeddings_model/
#     └── <model_files>
#
# Output Structure:
# <target-path>/            (specified via --vector-db-path)
# └── <random-tmp-dir>/
#     ├── vector_db/
#     │   ├── vector-db-data-1/
#     │   ├── vector-db-data-N/
#     │   └── ocp_X.YZ/     (if --enable-ocp-rag true and --ocp-version X.YZ)
#     └── embeddings_model/
#         └── <model_files>
#
# Arguments:
#  --vector-db-path PATH    Target directory for collected data (required)
#  --enable-ocp-rag BOOL    Enable OCP vector DB collection: true/false (required)
#  --ocp-version VERSION    OCP version to collect, e.g., "X.YZ" (required)

set -euo pipefail

# -- vector_database_collect.sh script parameters ----------------------------

# VECTOR_DB_VOLUME_MOUNT_PATH is location where content of a vector database
# image is mounted. Populated via parse_arguments_and_init.
VECTOR_DB_VOLUME_MOUNT_PATH=""

# ENABLE_OCP_RAG specifies whether this script should collect vector database
# content related to OCP (expected to be found under OCP_VECTOR_DB_DIR).
# Must be set to "true" for the collection to be enabled. Populated via
# parse_arguments_and_init.
ENABLE_OCP_RAG=""

# OCP_VERSION specifies what version of OCP content should be collected from
# the vector database image -> ${OCP_VECTOR_DB_DIR}/ocp_${OCP_VERSION}. Populated
# via parse_arguments_and_init.
OCP_VERSION=""
# ----------------------------------------------------------------------------

# -- Global vars -------------------------------------------------------------

# COLLECT_DIR is location within user provided VECTOR_DB_VOLUME_MOUNT_PATH where
# data collected from a single vector db image should be stored (populated
# via parse_arguments_and_init)
COLLECT_DIR=""

# VECTOR_DB_DATA_COLLECT_DIR is location within COLLECT_DIR where vector db
# related data should be stored (faiss_store.db, ogx_config.yaml). Populated
# via parse_arguments_and_init.
VECTOR_DB_DATA_COLLECT_DIR=""

# EMBEDDINGS_MODEL_DATA_COLLECT_DIR is location within COLLECT_DIR where
# embeddings model should be stored. Populated via parse_arguments_and_init.
EMBEDDINGS_MODEL_DATA_COLLECT_DIR=""

# OCP_VECTOR_DB_DIR specifies the directory within the vector DB container image
# where OCP-specific vector DB data must reside. This script expects to find the
# OCP data exclusively at this location.
OCP_VECTOR_DB_DIR="/rag/ocp_vector_db"

# OCP_VECTOR_DB_DIR_FALLBACK specifies the directory within the vector DB container
# image that contains the default OCP vector DB data. This path is used when no data
# matching the version specified via --ocp-version is found.
OCP_VECTOR_DB_DIR_FALLBACK="/rag/ocp_vector_db/ocp_latest"

# VECTOR_DB_DIR specifies the directory within the vector DB container image
# where general vector DB data must reside.
VECTOR_DB_DIR="/rag/vector_db"

# EMBEDDINGS_MODEL_DIR specifies the directory within the vector DB container image
# where embeddings model must reside.
EMBEDDINGS_MODEL_DIR="/rag/embeddings_model"

# OGX_CONFIG_FILE_NAME is the name of the OGX config file associated with a
# single vector database.
OGX_CONFIG_FILE_NAME="llama-stack.yaml"

# VECTOR_DB_FILE_NAME is the name of the file containing vector database data
# for a single vector database.
VECTOR_DB_FILE_NAME="faiss_store.db"
# ----------------------------------------------------------------------------

parse_arguments_and_init() {
    while [ $# -gt 0 ]; do
        case $1 in
            --vector-db-path)
                VECTOR_DB_VOLUME_MOUNT_PATH="$2"
                shift 2
                ;;
            --enable-ocp-rag)
                ENABLE_OCP_RAG="$2"
                shift 2
                ;;
            --ocp-version)
                OCP_VERSION="$2"
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 --vector-db-path PATH --enable-ocp-rag BOOL --ocp-version VERSION"
                echo ""
                echo "Arguments:"
                echo "  --vector-db-path     Target path for vector DB data collection"
                echo "  --enable-ocp-rag     Enable OCP RAG collection (true/false)"
                echo "  --ocp-version        OCP version to collect (e.g., 4.16)"
                echo "  -h, --help           Show this help message"
                exit 0
                ;;
            *)
                echo "Unknown argument: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done

    if [ -z "${VECTOR_DB_VOLUME_MOUNT_PATH:-}" ]; then
        echo "ERROR: --vector-db-path is required"
        exit 1
    fi

    if [ -z "${ENABLE_OCP_RAG:-}" ]; then
        echo "ERROR: --enable-ocp-rag is required"
        exit 1
    fi

    if [ -z "${OCP_VERSION:-}" ]; then
        echo "ERROR: --ocp-version is required"
        exit 1
    fi

    COLLECT_DIR=$(mktemp -d "${VECTOR_DB_VOLUME_MOUNT_PATH}/XXXXXXXXXX")
    VECTOR_DB_DATA_COLLECT_DIR="${COLLECT_DIR}/vector_db/"
    EMBEDDINGS_MODEL_DATA_COLLECT_DIR="${COLLECT_DIR}/embeddings_model"
}

validate_vector_db_dir() {
    local vector_db_dir="$1"

    if [ ! -d "${vector_db_dir}" ]; then
        echo "ERROR: ${vector_db_dir} is not a directory"
        exit 1
    fi

    if [ ! -f "${vector_db_dir}/${VECTOR_DB_FILE_NAME}" ]; then
        echo "ERROR: faiss_store.db is missing in ${vector_db_dir}"
        exit 1
    fi

    if [ ! -f "${vector_db_dir}/${OGX_CONFIG_FILE_NAME}" ]; then
        echo "ERROR: llama-stack.yaml is missing in ${vector_db_dir}"
        exit 1
    fi
}

collect_ocp_vector_db_data() {
    if [ "${ENABLE_OCP_RAG}" != "true" ]; then
        echo "Collecting of OCP vector db data is DISABLED => Skipping"
        return
    fi

    echo "Collecting OCP vector DB data ..."
    mkdir -p "${VECTOR_DB_DATA_COLLECT_DIR}"

    local ocp_dir="${OCP_VECTOR_DB_DIR}/ocp_${OCP_VERSION}"
    if [ ! -d "${ocp_dir}" ]; then
        echo "Data for OCP version ${OCP_VERSION} not found. Using: ${OCP_VECTOR_DB_DIR_FALLBACK}"
        ocp_dir=${OCP_VECTOR_DB_DIR_FALLBACK}
    fi

    validate_vector_db_dir "${ocp_dir}"
    cp -rL "${ocp_dir}" "${VECTOR_DB_DATA_COLLECT_DIR}"
    echo "Discovered and collected OCP vector DB data from ${ocp_dir}"
}

collect_vector_db_data() {
    echo "Collecting vector DB data ..."
    mkdir -p "${VECTOR_DB_DATA_COLLECT_DIR}"
    local vector_db_data_collected="false"
    for dir in ${VECTOR_DB_DIR}/*/; do
        [ ! -d "$dir" ] && continue

        validate_vector_db_dir "$dir"
        cp -rL "${dir}" "${VECTOR_DB_DATA_COLLECT_DIR}"
        vector_db_data_collected="true"
        echo "Discovered and collected vector DB data from ${dir}"
    done

    if [ "${ENABLE_OCP_RAG}" != "true" ] && [ ${vector_db_data_collected} != "true" ]; then
        echo "ERROR: ENABLE_OCP_RAG='${ENABLE_OCP_RAG}' and no generic vector db data found."
        exit 1
    fi
}

# TODO(lpiwowar): When introducing BYOK, ensure that the same embeddings model is not
# copied multiple times from different vector database images. Implement logic to check
# if the model already exists in the collection directory before copying; consider using
# symlinks or a similar mechanism to avoid redundant copies. These models can be large
# (e.g., 500MB), so minimizing duplication is important.
collect_embeddings_model() {
    echo "Collecting embeddings model ..."
    if [ ! -d "${EMBEDDINGS_MODEL_DIR}" ]; then
        echo "ERROR: Embeddings model dir not found under ${EMBEDDINGS_MODEL_DIR}."
        exit 1
    fi

    cp -rL "${EMBEDDINGS_MODEL_DIR}" "${EMBEDDINGS_MODEL_DATA_COLLECT_DIR}"
    echo "Discovered and collected embeddings model data from ${EMBEDDINGS_MODEL_DIR}"
}

main() {
    # NOTE: parse_arguments_and_init must be called first to ensure all global
    # variables are initialized before proceeding.
    parse_arguments_and_init "$@"
    collect_vector_db_data
    collect_ocp_vector_db_data
    collect_embeddings_model
}

main "$@"
