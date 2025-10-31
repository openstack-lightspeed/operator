# Openstack Lightspeed Operator

OpenStack Lightspeed Operator is a generative AI-based virtual assistant for
Red Hat OpenStack Services on OpenShift (RHOSO) users.

## Images

| Type | Quay.io Repository |
|------|--------------------|
| Operator | [quay.io/openstack-lightspeed/operator](https://quay.io/repository/openstack-lightspeed/operator?tab=tags) |
| Bundle | [quay.io/openstack-lightspeed/operator-bundle](https://quay.io/repository/openstack-lightspeed/operator-bundle?tab=tags) |
| Catalog | [quay.io/openstack-lightspeed/operator-catalog](https://quay.io/repository/openstack-lightspeed/operator-catalog?tab=tags) |


## Running Locally

You can run the operator locally (outside the cluster) using the Operator SDK make targets:

```bash
make install run
```

This will:

1. Install the CRDs into your cluster.
2. Run the operator locally, connected to your cluster.

Use this for quick development and testing.

## Deployment on OpenShift

You can also deploy the operator by creating a **CatalogSource** that references the catalog image.

```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: openstack-lightspeed-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: quay.io/openstack-lightspeed/operator-catalog:latest
  displayName: OpenStack Lightspeed Operator
  publisher: Red Hat
EOF
```

Once applied the catalog becomes visible in OperatorHub.
