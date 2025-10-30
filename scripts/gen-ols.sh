#!/bin/bash
# Deploy OLS
# Optionally pass the CSV version to use as an argument to use other than the
# latest stable version.
set -ex

if [ -z "${OUTPUT_DIR}" ]; then
  echo "Please set OPERATOR_DIR"
  exit 1
fi

if [ -n "$1" ]; then
  CSV_VERSION="$1"
else
  CSV_VERSION=$(oc get packagemanifest lightspeed-operator -o go-template="{{range .status.channels}}{{if eq .name \"stable\"}}{{.currentCSV}}{{\"\n\"}}{{end}}{{end}}")
fi

DEST_DIR="${OUTPUT_DIR}/ols"
mkdir -p "${DEST_DIR}"

cat > "${DEST_DIR}/namespace.yaml" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-lightspeed
  labels:
    kubernetes.io/metadata.name: openshift-lightspeed
EOF

cat > "${DEST_DIR}/operator_group.yaml" <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: lightspeed-operator-group
  namespace: openshift-lightspeed
spec:
  targetNamespaces:
  - openshift-lightspeed
EOF

cat > "${DEST_DIR}/subscription.yaml" <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  labels:
    operators.coreos.com/lightspeed-operator.openshift-lightspeed: ""
  name: lightspeed-operator
  namespace: openshift-lightspeed
spec:
  channel: stable
  installPlanApproval: Automatic
  name: lightspeed-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  startingCSV: ${CSV_VERSION}
EOF

