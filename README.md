# OpenStack Lightspeed Operator

OpenStack Lightspeed Operator is a generative AI-based virtual assistant for
Red Hat OpenStack Services on OpenShift (RHOSO) users.

## Images

| Type | Quay.io Repository |
|------|--------------------|
| Operator | [quay.io/openstack-lightspeed/operator](https://quay.io/repository/openstack-lightspeed/operator?tab=tags) |
| Bundle | [quay.io/openstack-lightspeed/operator-bundle](https://quay.io/repository/openstack-lightspeed/operator-bundle?tab=tags) |
| Catalog | [quay.io/openstack-lightspeed/operator-catalog](https://quay.io/repository/openstack-lightspeed/operator-catalog?tab=tags) |

## Quickstart

### Run OpenShift Cluster

We'll use CRC and deploy it using the development tools from `install_yamls`.

Get install-yamls:
```bash
git clone https://github.com/openstack-k8s-operators/install_yamls.git
cd install_yamls/devsetup
make download_tools
```

Get pull credentials (pull secret) from `https://cloud.redhat.com/openshift/create/local`
and save it in `pull-secret.txt` of the current path, or you can save it anywhere
and use the `PULL_SECRET` env var to point to it like in the next example.

Deploy OpenShift CRC and attach the libvirt default interface to CRC:

```bash
PULL_SECRET=~/work/pull-secret CRC_MONITORING_ENABLED=true CPUS=12 MEMORY=25600 DISK=100 make crc
make crc_attach_default_interface
eval $(crc oc-env)
cd ../..
```

### Deploy OpenStack Lightspeed Operator

Get the operator repository:

```bash
git clone https://github.com/openstack-lightspeed/operator.git
cd operator
```

First, deploy OpenStack Lightspeed Operator:

```bash
make openstack-lightspeed-deploy
```

Next, verify that the OpenStack Lightspeed Operator pod is running:

```bash
$ oc get -n openstack-lightspeed pods
NAME                                                              READY   STATUS    RESTARTS   AGE
openstack-lightspeed-operator-controller-manager-76df7fbfb5wggr   1/1     Running   0          72s
```

### Set up the LLM endpoint along with its credentials

To access the LLM we need:
- An API Key (eg: in `LLM_KEY`)
- An URL for the server (eg: in `LLM_ENDPOINT`)
- A model (eg: in `LLM_MODEL`)
- Optionally a certificate to access the LLM endpoint (name stored in
  `CERT_SECRET_NAME`)

The API key will be stored in a `Secret`, the certificate in a `ConfigMap` and
the other 2 together with the references to the first 2 will be passed in the
`OpenStackLightspeeed` resource that triggerrs the deployment.

Define the URL and model env vars, for example para Gemini:

```bash
LLM_ENDPOINT=https://generativelanguage.googleapis.com/v1beta/openai
LLM_MODEL=gemini-2.5-pro
LLM_KEY=<API TOKEN>
```

Create the LLM API key secret:

```bash
oc apply -f - <<EOF
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: openstack-lightspeed-apitoken
  namespace: openshift-lightspeed
stringData:
  apitoken: $LLM_KEY
EOF
```

Not required for Gemini, but here is an example of an optional certificate:

```bash
CERT_SECRET_NAME=openstack-lightspeed-certs
CERT_FILE=/path/to/cert.crt
```
```bash
oc apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
type: Opaque
metadata:
  name: $CERT_SECRET_NAME
  namespace: openshift-lightspeed
data:
  cert: |
$(sed 's/^/    /' "$CERT_FILE")
EOF
```

### Deploy

Create the `openstack` namespace if we haven't deployed openstack yet:

```bash
oc create namespace openstack
```

Deploy OpenStack-Lightspeed, a configuration would look like this (for actual
examples look in following sections):

```bash
oc apply -f - <<EOF
apiVersion: lightspeed.openstack.org/v1beta1
kind: OpenStackLightspeed
metadata:
  name: openstack-lightspeed
  namespace: openstack
spec:
$(if [ -n "$RHOS_LS_IMAGE" ]; then
  echo "  ragImage: $RHOS_LS_IMAGE"
fi)
  llmEndpoint: $LLM_ENDPOINT
  llmEndpointType: openai
  llmCredentials: openstack-lightspeed-apitoken
  modelName: $LLM_MODEL
$(if [ -n "$CERT_SECRET_NAME" ]; then
  echo "  tlsCACertBundle: $CERT_SECRET_NAME"
fi)
EOF
```

### Check deployment

Confirm the conditions are met

```bash
oc describe -n openstack openstacklightspeed
oc describe -n openshift-lightspeed olsconfig
```

### Use
Now you can go to the [OpenShift web console](https://console-openshift-console.apps-crc.testing) using the `kubeadmin` username and `12345678` password and use the OpenShift Lightspeed console widget that should appear at the lower right corner.
You may need to click on `refresh` console link that appears on a message.

If you are running CRC on a different machine you can use `sshuttle` to connect to the remote system:
- Edit your local system's `/etc/hosts` (where you use the browser) and add this line verbatim (don't change the IP): `192.168.130.11 api.crc.testing canary-openshift-ingress-canary.apps-crc.testing console-openshift-console.apps-crc.testing default-route-openshift-image-registry.apps-crc.testing downloads-openshift-console.apps-crc.testing oauth-openshift.apps-crc.testing`
- In your local system run `sshuttle -r $remote_username@$remote_server 192.168.130.0/24`.
- Now the console should be accessible in your browser.

## Development

If you are making changes to the operator you can run the operator locally
(outside the cluster) using the Operator SDK make targets:

```bash
make install run
```

This will:

1. Install the CRDs into your cluster.
2. Run the operator locally, connected to your cluster.

Use this for quick development and testing.

*Attention*: In this mode RBACs are ignored, so when changing those please run
the operator in the OpenShift cluster with an image.

## Quickstart

### Run OpenShift Cluster

We'll use CRC and deploy it using the development tools from `install_yamls`.

Get install-yamls:
```bash
git clone https://github.com/openstack-k8s-operators/install_yamls.git
cd install_yamls/devsetup
make download_tools
```

Get pull credentials (pull secret) from `https://cloud.redhat.com/openshift/create/local`
and save it in `pull-secret.txt` of the current path, or you can save it anywhere
and use the `PULL_SECRET` env var to point to it like in the next example.

## Development

### Running Pre-Commit Hooks

To ensure code quality and consistency, run pre-commit hooks locally before
submitting a pull request.

Install hooks:

```bash
pre-commit install
```

Run all hooks manually:

```bash
pre-commit run --all-files
```

### Running KUTTL Tests

KUTTL (KUbernetes Test TooL) tests validate the operator's behavior in a real
OpenShift environment.

Before running the tests ensure that:
- `oc` CLI tool is available in your PATH and you can access an OpenShift cluster
(e.g., deployed with `crc`) with it
- The `openshift-lightspeed` namespace is empty or non-existing to prevent collisions

Once you are ready you can run the KUTTL tests using:

```bash
make kuttl-test-run
```

**Important Notes:**
- The tests use the `openshift-lightspeed` namespace to test in the exact namespace
where the OLS operator is expected to operate.
- The correct behavior of the OLS operator is not guaranteed outside of the
`openshift-lightspeed` namespace.
- Ensure the namespace is clean before running tests to avoid resource conflicts
or test failures.
