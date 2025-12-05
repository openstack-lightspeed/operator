FROM registry.redhat.io/ubi9/ubi-minimal:9.7 as builder
ARG RELATED_IMAGE_FILE=related_images.json
ARG CSV_FILE=bundle/manifests/openstack-lightspeed-operator.clusterserviceversion.yaml
ARG OPERATOR_IMAGE_ORIGINAL=quay.io/openstack-lightspeed/operator:latest

RUN microdnf install -y jq

COPY ${CSV_FILE} /manifests/openstack-lightspeed-operator.clusterserviceversion.yaml
COPY ${RELATED_IMAGE_FILE} /${RELATED_IMAGE_FILE}

RUN OPERATOR_IMAGE=$(jq -r '.[] | select(.name == "openstack-lightspeed-operator") | .image' /${RELATED_IMAGE_FILE}) && sed -i "s|${OPERATOR_IMAGE_ORIGINAL}|${OPERATOR_IMAGE}|g" /manifests/openstack-lightspeed-operator.clusterserviceversion.yaml

FROM registry.redhat.io/ubi9/ubi-minimal:9.7

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=openstack-lightspeed-operator
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.bundle.channel.default.v1=alpha
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.38.0
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v4

# Labels for testing.
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/

# Copy files to locations specified by labels.
COPY bundle/manifests /manifests/
COPY bundle/metadata /metadata/
COPY bundle/tests/scorecard /tests/scorecard/

# Copy the CSV file with replaced image references
COPY --from=builder /manifests/openstack-lightspeed-operator.clusterserviceversion.yaml /manifests/openstack-lightspeed-operator.clusterserviceversion.yaml
