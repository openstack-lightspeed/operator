#!/bin/bash
export RELATED_IMAGE_LCORE_IMAGE_URL_DEFAULT="quay.io/lightspeed-core/lightspeed-stack:dev-latest"
export RELATED_IMAGE_EXPORTER_IMAGE_URL_DEFAULT="quay.io/lightspeed-core/lightspeed-to-dataverse-exporter:latest"
export RELATED_IMAGE_POSTGRES_IMAGE_URL_DEFAULT="registry.redhat.io/rhel9/postgresql-16:latest"
# TODO(lpiwowar): Replace this with a stable (non-alpha) image version once
# the automated pipeline for building OGX-compatible vector database images
# is ready.
export RELATED_IMAGE_OPENSTACK_LIGHTSPEED_IMAGE_URL_DEFAULT="quay.io/openstack-lightspeed/rag-content:alpha-ogx-os-docs-2025.2"
export WATCH_NAMESPACE="openstack-lightspeed"
