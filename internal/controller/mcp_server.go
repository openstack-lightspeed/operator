/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	common_deployment "github.com/openstack-k8s-operators/lib-common/modules/common/deployment"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// ---------------------------------------------------------------------------
// Builders: MCP Server Deployment
// ---------------------------------------------------------------------------

const (
	// CloudsYAMLConfigMapName is the name of the ConfigMap, located in the operator's namespace,
	// that contains the clouds.yaml configuration file.
	CloudsYAMLConfigMapName string = "openstack-config"

	// SecureYAMLSecretName is the name of the Secret, located in the operator's namespace,
	// that contains the secrets.yaml configuration file for the MCP server.
	SecureYAMLSecretName string = "openstack-config-secret"

	// CombinedCABundleSecretName is the name of the Secret, located in the operator's namespace,
	// that contains the combined TLS CA bundle required by the MCP server.
	CombinedCABundleSecretName string = "combined-ca-bundle"

	// MCPConfigYAMLConfigMapName is the name of the ConfigMap, located in the operator's namespace,
	// that contains the config.yaml file for the MCP server.
	MCPConfigYAMLConfigMapName string = "mcp-config"

	// MCPServerPort - Port on which the OpenStack Lightspeed MCP server listens
	MCPServerPort = 8080

	// MCPServiceName specifies the Service name for the OpenStack Lightspeed MCP server
	MCPServiceName = "mcp-server-service"
)

// MCPServerConfig - stores the config file for the MCP server
//
//go:embed mcp_server_config.yaml
var MCPServerConfig string

// MCPDeploymentLabels are the labels applied to the MCP server deployment.
var MCPDeploymentLabels = map[string]string{
	"app": "openstack-lightspeed-mcp-server",
}

// BuildMCPServerDeployment creates a Kubernetes Deployment resource for the MCP server.
// The Deployment expects that the following resources exists:
//   - ConfigMap with `CloudsYAMLConfigMapName` name containing clouds.yaml
//   - ConfigMap with `MCPConfigYAMLConfigMapName` name containing config.yaml
//   - Secret with `SecretsYAMLSecretName` name containing secure.yaml
//   - Secret with `CombinedCABundleSecretName` name containing tls-ca-bundle.pem
func BuildMCPServerDeployment(
	instance *apiv1beta1.OpenStackLightspeed,
) appsv1.Deployment {
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcp-server",
			Namespace: instance.Namespace,
			Labels:    MCPDeploymentLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: MCPDeploymentLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: MCPDeploymentLabels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: SecureYAMLSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: SecureYAMLSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  "secure.yaml",
											Path: "secure.yaml",
										},
									},
								},
							},
						},
						{
							Name: CloudsYAMLConfigMapName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: CloudsYAMLConfigMapName,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "clouds.yaml",
											Path: "clouds.yaml",
										},
									},
								},
							},
						},
						{
							Name: CombinedCABundleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: CombinedCABundleSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  "tls-ca-bundle.pem",
											Path: "tls-ca-bundle.pem",
										},
									},
								},
							},
						},
						{
							Name: MCPConfigYAMLConfigMapName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: MCPConfigYAMLConfigMapName,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "config.yaml",
											Path: "config.yaml",
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{{
						Name:  "mcp-server-container",
						Image: apiv1beta1.OpenStackLightspeedDefaultValues.MCPServerImageURL,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      SecureYAMLSecretName,
								MountPath: "/app/secure.yaml",
								SubPath:   "secure.yaml",
							},
							{
								Name:      CloudsYAMLConfigMapName,
								MountPath: "/app/clouds.yaml",
								SubPath:   "clouds.yaml",
							},
							{
								Name:      CombinedCABundleSecretName,
								MountPath: "/app/tls-ca-bundle.pem",
								SubPath:   "tls-ca-bundle.pem",
								ReadOnly:  true,
							},
							{
								Name:      MCPConfigYAMLConfigMapName,
								MountPath: "/app/config.yaml",
								SubPath:   "config.yaml",
							},
						},
					}},
				},
			},
		},
	}

	deployment.SetOwnerReferences([]metav1.OwnerReference{
		CreateOwnerReference(instance),
	})

	return deployment
}

func BuildMCPServerService(
	instance *apiv1beta1.OpenStackLightspeed,
) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MCPServiceName,
			Namespace: instance.Namespace,
			Labels:    MCPDeploymentLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: MCPDeploymentLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       MCPServerPort,
					TargetPort: intstr.FromInt(MCPServerPort),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

func BuildMCPServerConfigMap(
	instance *apiv1beta1.OpenStackLightspeed,
) corev1.ConfigMap {
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MCPConfigYAMLConfigMapName,
			Namespace: instance.Namespace,
		},
		Data: map[string]string{
			"config.yaml": MCPServerConfig,
		},
	}

	configMap.SetOwnerReferences([]metav1.OwnerReference{
		CreateOwnerReference(instance),
	})

	return configMap
}

func GetMCPServerURL() string {
	return fmt.Sprintf("http://%s:%d", MCPServiceName, MCPServerPort)
}

// ---------------------------------------------------------------------------
// End Builders
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Reconcile: MCP deployment
// ---------------------------------------------------------------------------

// ReconcileMCPServer performs the reconciliation of the OpenStack Lightspeed
// MCP server.
func (r *OpenStackLightspeedReconciler) ReconcileMCPServer(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (ctrl.Result, error) {
	var openStackControlPlaneInstance openstackv1.OpenStackControlPlane
	ready, err := IsDynamicCRDReady(helper, r.DynamicWatchCRD, &openStackControlPlaneInstance)
	if err != nil {
		return ctrl.Result{}, err
	} else if !ready {
		helper.GetLogger().Info("OpenStackControlPlane CRD not available, deleting MCP server")
		return r.ReconcileMCPServerDelete(ctx, helper, instance)
	}

	openStackControlPlaneList := &openstackv1.OpenStackControlPlaneList{}
	err = r.List(ctx, openStackControlPlaneList)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	switch openStackControlPlaneListLen := len(openStackControlPlaneList.Items); openStackControlPlaneListLen {

	case 0:
		helper.GetLogger().Info("No OpenStackControlPlane found, deleting MCP server")
		return r.ReconcileMCPServerDelete(ctx, helper, instance)

	case 1:
		openStackControlPlaneInstance = openStackControlPlaneList.Items[0]
		return r.ReconcileMCPServerCreate(ctx, helper, instance, &openStackControlPlaneInstance)

	default:
		err = errors.New("more than one OpenStackControlPlane found")
		return ctrl.Result{}, err
	}
}

// ReconcileMCPServerCreate ensures the MCP Server is deployed and configured.
// It copies required resources from the openstack namespace, creates
// the MCP Server ConfigMap, and makes sure the deployment uses the latest resources.
// The OpenStackLightspeed instance status is updated on success; on error,
// the caller must update status.
func (r *OpenStackLightspeedReconciler) ReconcileMCPServerCreate(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
	openStackControlPlaneInstance *openstackv1.OpenStackControlPlane,
) (ctrl.Result, error) {
	ok, err := CheckRequiredOpenStackControlPlaneFields(helper, openStackControlPlaneInstance)
	if err != nil {
		return ctrl.Result{}, err
	} else if !ok {
		return ctrl.Result{}, nil
	}

	copiedObjects, err := CopyObjectsToOpenStackLightspeedNamespace(
		ctx,
		helper,
		instance,
		openStackControlPlaneInstance,
	)
	if err != nil && k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	ConfigYAMLConfigMap := BuildMCPServerConfigMap(instance)
	err = helper.GetClient().Create(ctx, &ConfigYAMLConfigMap)
	if err != nil && !k8s_errors.IsAlreadyExists(err) {
		return ctrl.Result{}, err
	}

	err = DeployMCPServer(ctx, helper, instance, openStackControlPlaneInstance, copiedObjects)
	if err != nil {
		return ctrl.Result{}, err
	}

	// A set of resources we want to be owned by the MCP Server deployment.
	resourcesToOwn := []client.Object{
		copiedObjects[CloudsYAMLConfigMapName],
		copiedObjects[SecureYAMLSecretName],
		copiedObjects[CombinedCABundleSecretName],
		&ConfigYAMLConfigMap,
	}
	err = MCPServerDeploymentOwnResources(ctx, helper, instance, resourcesToOwn...)
	if err != nil {
		return ctrl.Result{}, err
	}

	mcpServerDeploymentReady, err := IsMCPServerDeploymentReady(ctx, helper, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mcpServerDeploymentReady {
		instance.Status.Conditions.MarkTrue(
			apiv1beta1.OpenStackLightspeedMCPServerReadyCondition,
			apiv1beta1.OpenStackLightspeedMCPServerDeployed,
		)
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackLightspeedReconciler) ReconcileMCPServerDelete(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (ctrl.Result, error) {
	deployment := BuildMCPServerDeployment(instance)
	err := helper.GetClient().Delete(ctx, &deployment)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	instance.Status.Conditions.MarkTrue(
		apiv1beta1.OpenStackLightspeedMCPServerReadyCondition,
		apiv1beta1.OpenStackLightspeedMCPServerNoDeployment,
	)

	return ctrl.Result{}, nil
}

// CheckRequiredOpenStackControlPlaneFields checks whether the required fields are present
// in the OpenStackControlPlane instance for the MCP server. The following fields are validated:
//   - .Spec.OpenStackClient.Template.OpenStackConfigSecret:
//     The name of the Secret containing secrets.yaml. This field is required.
//   - .Spec.OpenStackClient.Template.OpenStackConfigMap:
//     The name of the ConfigMap containing clouds.yaml. This field is required.
//   - .Status.TLS.CaBundleSecretName:
//     The name of the Secret containing CA certificates. This gets populated during
//     the deployment.
func CheckRequiredOpenStackControlPlaneFields(
	helper *common_helper.Helper,
	openStackControlPlaneInstance *openstackv1.OpenStackControlPlane,
) (bool, error) {
	if openStackControlPlaneInstance.Spec.OpenStackClient.Template.OpenStackConfigSecret == nil {
		err := errors.New("OpenStackClient.Template.OpenStackConfigSecret is missing value")
		return false, err
	} else if openStackControlPlaneInstance.Spec.OpenStackClient.Template.OpenStackConfigMap == nil {
		err := errors.New("OpenStackControlPlane.OpenStackClient.Template.OpenStackConfigMap is missing value")
		return false, err
	} else if openStackControlPlaneInstance.Status.TLS.CaBundleSecretName == "" {
		helper.GetLogger().Info("Waiting for OpenStackControlPlaneInstance.Status.TLS.CaBundleSecretName value")
		return false, nil
	}

	return true, nil
}

// CopyObjectsToOpenStackLightspeedNamespace copies the required ConfigMaps and
// Secrets from the OpenStackControlPlane's namespace to the OpenStack Lightspeed
// deployment namespace, making these resources available to the MCP server.
// All necessary fields in the OpenStackControlPlane instance must be non-nil
// and valid. It is recommended  to call CheckRequiredOpenStackControlPlaneFields
// to ensure this.
func CopyObjectsToOpenStackLightspeedNamespace(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
	openStackControlPlaneInstance *openstackv1.OpenStackControlPlane,
) (map[string]client.Object, error) {
	objectsToCopy := map[string]client.Object{
		SecureYAMLSecretName: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      *openStackControlPlaneInstance.Spec.OpenStackClient.Template.OpenStackConfigSecret,
				Namespace: openStackControlPlaneInstance.Namespace,
			},
		},
		CloudsYAMLConfigMapName: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      *openStackControlPlaneInstance.Spec.OpenStackClient.Template.OpenStackConfigMap,
				Namespace: openStackControlPlaneInstance.Namespace,
			},
		},
		CombinedCABundleSecretName: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openStackControlPlaneInstance.Status.TLS.CaBundleSecretName,
				Namespace: openStackControlPlaneInstance.Namespace,
			},
		},
	}

	ownerReference := []metav1.OwnerReference{CreateOwnerReference(instance)}
	copiedObjects := make(map[string]client.Object)
	var err error

	for resourceName, sourceObject := range objectsToCopy {
		targetObject := sourceObject.DeepCopyObject().(client.Object)
		targetObject.SetNamespace(instance.Namespace)
		targetObject.SetName(resourceName)

		copiedObjects[resourceName], err = CopyResource(ctx, helper, sourceObject, targetObject, ownerReference)
		if err != nil && k8s_errors.IsNotFound(err) {
			helper.GetLogger().Info(
				"Resource %s not found in namespace %s, waiting for it to be created",
				sourceObject.GetName(),
				sourceObject.GetNamespace(),
			)
			return nil, err
		} else if err != nil {
			return nil, err
		} else if copiedObjects[resourceName] == nil {
			return nil, errors.New("the internal representatnion of the copied object is nil")
		}
	}

	return copiedObjects, nil
}

// DeployMCPServer ensures that the MCP Server Deployment is up-to-date and
// properly configured. It creates or patches the Deployment by injecting the
// names of the necessary copied resources (ConfigMaps and Secrets) from the
// OpenStack related namespace. Additionally, it sets annotations on
// the Deployment with the checksums of these resources. Any change in resource
// content will trigger a restart of the pods, ensuring consistent configuration.
func DeployMCPServer(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
	openStackControlPlaneInstance *openstackv1.OpenStackControlPlane,
	requiredResources map[string]client.Object,
) error {
	deployment := BuildMCPServerDeployment(instance)
	_, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), &deployment, func() error {
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}

		var volumeSections []*corev1.Volume
		volumeSections = append(volumeSections, GetDeploymentVolumeSection(deployment, CloudsYAMLConfigMapName))
		volumeSections = append(volumeSections, GetDeploymentVolumeSection(deployment, SecureYAMLSecretName))
		volumeSections = append(volumeSections, GetDeploymentVolumeSection(deployment, CombinedCABundleSecretName))

		var deploymentChecksum string
		const checksumPrefixLen = 10
		for _, volumeSection := range volumeSections {
			if volumeSection == nil {
				return errors.New("missing volume section in MCP Server deployment")
			}

			checksum := GetChecksumAnnotation(requiredResources[volumeSection.Name])
			if len(checksum) == 0 {
				return fmt.Errorf("missing checksum annotation for: %s", volumeSection.Name)
			}

			deploymentChecksum += checksum[:checksumPrefixLen]
		}

		// Setting the checksum annotation ensures the MCP server pod is automatically redeployed
		// whenever any associated ConfigMap or Secret changes.
		deployment.Spec.Template.Annotations[OpenStackLightspeedChecksumAnnotation] = deploymentChecksum

		return nil
	})
	if err != nil {
		return err
	}

	service := BuildMCPServerService(instance)
	_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), &service, func() error {
		return nil
	})

	return err
}

// GetMCPServerDeployment returns (true, nil) if the MCPServer deployment is ready.
// Otherwise, it returns (false, err) or (false, nil) when the deployment is not found.
func GetMCPServerDeployment(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (*appsv1.Deployment, error) {
	deployment := BuildMCPServerDeployment(instance)

	latestDeployment := &appsv1.Deployment{}
	err := helper.GetClient().Get(ctx, client.ObjectKey{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, latestDeployment)
	if err != nil && k8s_errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return latestDeployment, nil
}

// MCPServerDeploymentOwnResources sets the specified resources in mcpServerResources to be owned
// by the MCP Server Deployment by assigning an OwnerReference to each resource. This ensures that
// these resources are automatically cleaned up when the deployment is deleted.
func MCPServerDeploymentOwnResources(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
	mcpServerResources ...client.Object,
) error {
	MCPServerDeployemnt, err := GetMCPServerDeployment(ctx, helper, instance)
	if err != nil {
		return err
	}

	if MCPServerDeployemnt != nil {
		deploymentOwnerRefs := []metav1.OwnerReference{
			{
				APIVersion: MCPServerDeployemnt.APIVersion,
				Kind:       MCPServerDeployemnt.Kind,
				Name:       MCPServerDeployemnt.Name,
				UID:        MCPServerDeployemnt.UID,
			},
		}
		for _, obj := range mcpServerResources {
			obj.SetOwnerReferences(deploymentOwnerRefs)
			err = helper.GetClient().Update(ctx, obj)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// IsMCPServerDeploymentReady returns true if the MCP server deployment is fully
// ready and available for use.
func IsMCPServerDeploymentReady(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	MCPServerDeployemnt, err := GetMCPServerDeployment(ctx, helper, instance)
	if err != nil {
		return false, err
	}

	return common_deployment.IsReady(*MCPServerDeployemnt), nil
}

// ---------------------------------------------------------------------------
// End Reconcile
// ---------------------------------------------------------------------------
