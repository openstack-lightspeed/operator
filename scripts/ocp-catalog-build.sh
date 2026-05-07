#!/bin/bash
# Build a catalog image for use with the OpenShift internal registry.
#
# OPM cannot pull bundle images from the internal registry (unreachable from dev machine),
# so this script:
# 1. Pushes the bundle to the internal registry via the external route
# 2. Uses OPM to build the catalog index from the external route (pullable)
# 3. Fixes the bundlepath in the database to use the internal registry address
# 4. Builds the final catalog image
#
# Usage: ocp-catalog-build.sh <container-tool> <bundle-img> <catalog-img> <opm>
set -euo pipefail

CONTAINER_TOOL="${1:?Usage: $0 <container-tool> <bundle-img> <catalog-img> <opm>}"
BUNDLE_IMG="${2:?Usage: $0 <container-tool> <bundle-img> <catalog-img> <opm>}"
CATALOG_IMG="${3:?Usage: $0 <container-tool> <bundle-img> <catalog-img> <opm>}"
OPM="${4:?Usage: $0 <container-tool> <bundle-img> <catalog-img> <opm>}"

INTERNAL_PREFIX="image-registry.openshift-image-registry.svc:5000"

echo "==> Ensuring the OpenShift image registry default route is exposed..."
oc patch configs.imageregistry.operator.openshift.io/cluster \
  --type merge -p '{"spec":{"defaultRoute":true}}'

ROUTE_HOST="$(oc get route default-route -n openshift-image-registry \
  -o jsonpath='{.spec.host}')"
if [ -z "${ROUTE_HOST}" ]; then
  echo "Error: could not obtain the registry route host."
  exit 1
fi

ROUTE_BUNDLE="${BUNDLE_IMG/${INTERNAL_PREFIX}/${ROUTE_HOST}}"

echo "==> Logging in to the registry..."
${CONTAINER_TOOL} login "${ROUTE_HOST}" -u "$(oc whoami)" -p "$(oc whoami -t)" --tls-verify=false

echo "==> Pushing bundle image to internal registry..."
${CONTAINER_TOOL} tag "${BUNDLE_IMG}" "${ROUTE_BUNDLE}"
${CONTAINER_TOOL} push "${ROUTE_BUNDLE}" --tls-verify=false
${CONTAINER_TOOL} rmi "${ROUTE_BUNDLE}" 2>/dev/null || true

echo "==> Building catalog index from external route..."
WORKDIR=$(mktemp -d)
# shellcheck disable=SC2064
trap "rm -rf ${WORKDIR}" EXIT

${OPM} index add \
  --build-tool "${CONTAINER_TOOL}" --pull-tool none --skip-tls-verify \
  --generate --mode semver \
  --out-dockerfile "${WORKDIR}/index.Dockerfile" \
  --bundles "${ROUTE_BUNDLE}" \
  --tag "${CATALOG_IMG}"
mv database "${WORKDIR}/"

echo "==> Fixing bundle path in catalog database..."
sqlite3 "${WORKDIR}/database/index.db" \
  "UPDATE operatorbundle SET bundlepath = REPLACE(bundlepath, '${ROUTE_HOST}', '${INTERNAL_PREFIX}');"

echo "==> Building catalog image..."
${CONTAINER_TOOL} build -f "${WORKDIR}/index.Dockerfile" -t "${CATALOG_IMG}" "${WORKDIR}"

echo "==> Catalog image built successfully: ${CATALOG_IMG}"
