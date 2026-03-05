#!/bin/bash
set -e
echo "Validating checksum annotations match between resources and deployment..."

# Get checksum from ConfigMap
CM_CHECKSUM=$(kubectl get configmap openstack-config \
  -n openshift-lightspeed \
  -o jsonpath='{.metadata.annotations.openstack\.org/checksum}')

# Get checksum from Secret (openstack-config-secret)
SECRET_CHECKSUM=$(kubectl get secret openstack-config-secret \
  -n openshift-lightspeed \
  -o jsonpath='{.metadata.annotations.openstack\.org/checksum}')

# Get checksum from CA Bundle Secret
CA_CHECKSUM=$(kubectl get secret combined-ca-bundle \
  -n openshift-lightspeed \
  -o jsonpath='{.metadata.annotations.openstack\.org/checksum}')

# Get checksum from Deployment pod template for secrets.yaml
DEP_CHECKSUM=$(kubectl get deployment mcp-server \
  -n openshift-lightspeed \
  -o jsonpath='{.spec.template.metadata.annotations.openstack\.org/checksum}')


# Validate all checksums match
FAILED=0
COMBINED_CHECKSUM="${CM_CHECKSUM:0:10}${SECRET_CHECKSUM:0:10}${CA_CHECKSUM:0:10}"
if [ "$COMBINED_CHECKSUM" = "$DEP_CHECKSUM" ]; then
  echo "✓ Combined checksum matches: $COMBINED_CHECKSUM"
else
  echo "✗ Combined checksum mismatch!"
  echo "  Combined checksum: $COMBINED_CHECKSUM"
  echo "  Deployment checksum: $DEP_CHECKSUM"
  FAILED=1
fi

if [ $FAILED -eq 1 ]; then
  echo "Checksum validation failed!"
  exit 1
fi

echo "All checksums match successfully!"
exit 0
