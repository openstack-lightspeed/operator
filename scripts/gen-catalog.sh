#!/bin/bash
# Create a catalog for openstack-lightspeed
# Pass the image location as the first argument
# Optionally pass the catalog name as the second argument
set -ex

if [ -z "${OUTPUT_DIR}" ]; then
  echo "Please set OPERATOR_DIR"
  exit 1
fi

IMAGE_LOCATION=$1
CATALOG_NAME=${2:-openstack-lightspeed-catalog}

DEST_DIR="${OUTPUT_DIR}/catalog"
mkdir -p "${DEST_DIR}"

# oc delete --ignore-not-found=true -n openshift-marketplace CatalogSource "${CATALOG_NAME}"

cat > "${DEST_DIR}/catalogsource.yaml" <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${CATALOG_NAME}
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${IMAGE_LOCATION}
  displayName: OpenStack Lightspeed Operator
  publisher: Red Hat
EOF
