# Openstack Lightspeed Operator

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
$ git clone https://github.com/openstack-k8s-operators/install_yamls.git
$ cd install_yamls/devsetup
$ make download_tools
```

Get pull credentials (pull secret) from `https://cloud.redhat.com/openshift/create/local`
and save it in `pull-secret.txt` of the current path, or you can save it anywhere
and use the `PULL_SECRET` env var to point to it like in the next example.

Deploy OpenShift and OpenStack operator:
```bash
$ PULL_SECRET=~/work/pull-secret CRC_MONITORING_ENABLED=true CPUS=12 MEMORY=25600 DISK=100 make crc
$ make crc_attach_default_interface
$ eval $(crc oc-env)
$ cd ../..
```

### Get the operator repository

```bash
$ git clone https://github.com/openstack-lightspeed/operator.git
$ cd operator
```

### Deploy operators

We need to deploy the OpenShift Lightspeed and the OpenStack Lightspeed
operators:

```bash
$ make ols-deploy
$ make catalog-deploy
```

Now check that they are running:

```bash
$ oc get -n openshift-lightspeed pods
NAME                                                      READY   STATUS    RESTARTS   AGE
lightspeed-operator-controller-manager-7f4698b55c-8w8vn   1/1     Running   0          81s

$ oc get -n openstack-lightspeed pods
NAME                                                              READY   STATUS    RESTARTS   AGE
openstack-lightspeed-operator-controller-manager-76df7fbfb5wggr   1/1     Running   0          72s
```

### Create LLM credentials

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
```
$ LLM_ENDPOINT=https://generativelanguage.googleapis.com/v1beta/openai
$ LLM_MODEL=gemini-2.5-pro
```

Create the LLM API key secret:
```
$ oc apply -f - <<EOF
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

Not needed for Gemini, but here's an example of the optional certificate:
```
$ export CERT_SECRET_NAME=openstack-lightspeed-certs
$ oc apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
type: Opaque
metadata:
  name: $CERT_SECRET_NAME
  namespace: openshift-lightspeed
data:
  cert: |
    -----BEGIN CERTIFICATE-----
    MIIE5 <...>
    ------END CERTIFICATE-----
EOF
```

### Deploy

Create the `openstack` namespace if we haven't deployed openstack yet:

```
$ oc create namespace openstack
```

Deploy OpenStack-Lightspeed, a configuration would look like this (for actual
examples look in following sections):
```
$ oc apply -f - <<EOF
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
$ oc describe -n openstack openstacklightspeed
$ oc describe -n openshift-lightspeed olsconfig
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
