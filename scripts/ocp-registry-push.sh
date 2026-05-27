#!/bin/bash
# Push images to the OpenShift internal image registry.
#
# Usage: ocp-registry-push.sh <container-tool> <namespace> <image1> [image2 ...]
#
# Each <imageN> must use the in-cluster registry address, e.g.:
#   image-registry.openshift-image-registry.svc:5000/my-ns/my-image:tag
set -euo pipefail

CONTAINER_TOOL="${1:?Usage: $0 <container-tool> <namespace> <image1> [image2 ...]}"
NAMESPACE="${2:?Usage: $0 <container-tool> <namespace> <image1> [image2 ...]}"
shift 2

if [ $# -eq 0 ]; then
  echo "Error: at least one image must be specified"
  exit 1
fi

INTERNAL_PREFIX="image-registry.openshift-image-registry.svc:5000"

echo "==> Ensuring the OpenShift image registry default route is exposed..."
oc patch configs.imageregistry.operator.openshift.io/cluster \
  --type merge -p '{"spec":{"defaultRoute":true}}'

echo "==> Obtaining the external registry route..."
ROUTE_HOST="$(oc get route default-route -n openshift-image-registry \
  -o jsonpath='{.spec.host}')"
if [ -z "${ROUTE_HOST}" ]; then
  echo "Error: could not obtain the registry route host."
  exit 1
fi
echo "    External route: ${ROUTE_HOST}"

echo "==> Ensuring namespace '${NAMESPACE}' exists..."
oc create namespace "${NAMESPACE}" --dry-run=client -o yaml | oc apply -f -

echo "==> Logging in to the registry..."
${CONTAINER_TOOL} login "${ROUTE_HOST}" -u "$(oc whoami)" -p "$(oc whoami -t)" --tls-verify=false

for IMAGE in "$@"; do
  PUSH_IMAGE="${IMAGE/${INTERNAL_PREFIX}/${ROUTE_HOST}}"
  if [ "${PUSH_IMAGE}" = "${IMAGE}" ]; then
    echo "Warning: image '${IMAGE}' does not start with '${INTERNAL_PREFIX}', pushing as-is."
  fi

  echo "==> Pushing ${IMAGE} -> ${PUSH_IMAGE}"
  ${CONTAINER_TOOL} tag "${IMAGE}" "${PUSH_IMAGE}"
  ${CONTAINER_TOOL} push "${PUSH_IMAGE}" --tls-verify=false
  ${CONTAINER_TOOL} rmi "${PUSH_IMAGE}" 2>/dev/null || true
done

echo "==> All images pushed successfully."
