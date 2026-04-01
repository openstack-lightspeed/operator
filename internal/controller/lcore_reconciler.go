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
	"fmt"
	"time"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ReconcileLCoreResources reconciles Phase 1 resources: service accounts, roles,
// config maps, secrets, and network policies. Uses a continue-on-error pattern
// so that all tasks are attempted even if some fail.
func ReconcileLCoreResources(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	tasks := []ReconcileTask{
		{Name: "ServiceAccount", Task: reconcileServiceAccount},
		{Name: "SARRole", Task: reconcileSARRole},
		{Name: "SARRoleBinding", Task: reconcileSARRoleBinding},
		{Name: "LlamaStackConfigMap", Task: reconcileLlamaStackConfigMap},
		{Name: "LcoreConfigMap", Task: reconcileLcoreConfigMap},
		{Name: "OpenStackLightspeedAdditionalCAConfigMap", Task: reconcileOpenStackLightspeedAdditionalCAConfigMap},
		{Name: "ProxyCAConfigMap", Task: reconcileProxyCAConfigMap},
		{Name: "NetworkPolicy", Task: reconcileNetworkPolicy},
	}

	return ReconcileTasks(h, ctx, instance, tasks)
}

// ReconcileLCoreDeployment reconciles Phase 2 resources: deployment, service,
// TLS secret, service monitor, and prometheus rule. Uses a fail-fast pattern
// where the first error stops execution.
func ReconcileLCoreDeployment(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	tasks := []ReconcileTask{
		{Name: "Deployment", Task: reconcileDeployment},
		{Name: "Service", Task: reconcileService},
		{Name: "TLSSecret", Task: reconcileTLSSecret},
	}

	return ReconcileTasksFailFast(h, ctx, instance, tasks)
}

// reconcileServiceAccount ensures the OpenStack Lightspeed app server service account exists.
func reconcileServiceAccount(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OpenStackLightspeedAppServerServiceAccountName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), sa, func() error {
		// ServiceAccount has no spec to set, just ensure owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), sa, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateAPIServiceAccount, err)
	}

	logger.Info("ServiceAccount reconciled", "name", sa.Name, "result", result)
	return nil
}

// reconcileSARRole ensures the SAR cluster role exists.
func reconcileSARRole(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   OpenStackLightspeedAppServerSARRoleName,
			Labels: generateAppServerSelectorLabels(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), role, func() error {
		// Set the Rules spec
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"authorization.k8s.io"},
				Resources: []string{"subjectaccessreviews"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"authentication.k8s.io"},
				Resources: []string{"tokenreviews"},
				Verbs:     []string{"create"},
			},
		}
		// Note: ClusterRole is cluster-scoped, no owner reference needed
		return nil
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateSARClusterRole, err)
	}

	logger.Info("SAR ClusterRole reconciled", "name", role.Name, "result", result)
	return nil
}

// reconcileSARRoleBinding ensures the SAR cluster role binding exists.
func reconcileSARRoleBinding(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	rb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   OpenStackLightspeedAppServerSARRoleBindingName,
			Labels: generateAppServerSelectorLabels(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), rb, func() error {
		// Set Subjects and RoleRef
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      OpenStackLightspeedAppServerServiceAccountName,
				Namespace: h.GetBeforeObject().GetNamespace(),
			},
		}
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     OpenStackLightspeedAppServerSARRoleName,
		}
		// Note: ClusterRoleBinding is cluster-scoped, no owner reference needed
		return nil
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateSARClusterRoleBinding, err)
	}

	logger.Info("SAR ClusterRoleBinding reconciled", "name", rb.Name, "result", result)
	return nil
}

// reconcileLlamaStackConfigMap ensures the Llama Stack config map exists and is up to date.
func reconcileLlamaStackConfigMap(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	// Build the YAML data
	yamlData, err := buildLlamaStackYAML(h, ctx, instance)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGenerateLlamaStackConfigMap, err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LlamaStackConfigCmName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), cm, func() error {
		// Set Data (same as current selective update)
		cm.Data = map[string]string{
			LlamaStackConfigFilename: yamlData,
		}
		// Set owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), cm, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateLlamaStackConfigMap, err)
	}

	logger.Info("Llama Stack ConfigMap reconciled", "name", cm.Name, "result", result)
	return nil
}

// reconcileLcoreConfigMap ensures the LCore config map exists and is up to date.
func reconcileLcoreConfigMap(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	// Build the YAML data
	yamlData, err := buildLCoreConfigYAML(h, instance)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGenerateAPIConfigmap, err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LCoreConfigCmName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), cm, func() error {
		// Set Data (same as current selective update)
		cm.Data = map[string]string{
			LCoreConfigFilename: yamlData,
		}
		// Set owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), cm, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateAPIConfigmap, err)
	}

	logger.Info("LCore ConfigMap reconciled", "name", cm.Name, "result", result)
	return nil
}

// reconcileOpenStackLightspeedAdditionalCAConfigMap verifies that the additional CA config map
// exists if one is specified in the configuration.
func reconcileOpenStackLightspeedAdditionalCAConfigMap(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	if instance.Spec.TLSCACertBundle == "" {
		logger.Info("no additional CA configmap configured, skipping")
		return nil
	}

	existing := &corev1.ConfigMap{}
	err := h.GetClient().Get(ctx, client.ObjectKey{
		Name:      instance.Spec.TLSCACertBundle,
		Namespace: h.GetBeforeObject().GetNamespace(),
	}, existing)
	if err != nil {
		return fmt.Errorf("%w %q: %v", ErrGetAdditionalCACM, instance.Spec.TLSCACertBundle, err)
	}

	logger.Info("additional CA configmap found", "name", instance.Spec.TLSCACertBundle)
	return nil
}

// reconcileProxyCAConfigMap is a no-op for the minimal mapping (no proxy config).
func reconcileProxyCAConfigMap(h *common_helper.Helper, _ context.Context, _ *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()
	logger.Info("proxy CA configmap reconciliation skipped (no proxy config in minimal mapping)")
	return nil
}

// reconcileNetworkPolicy ensures the app server network policy exists and is up to date.
func reconcileNetworkPolicy(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OpenStackLightspeedAppServerNetworkPolicyName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), np, func() error {
		// Set Spec (wholesale replacement, same as before)
		np.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: generateAppServerSelectorLabels(),
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: toPtr(corev1.ProtocolTCP),
							Port:     toPtr(intstr.FromInt32(OpenStackLightspeedAppServerContainerPort)),
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		}
		// Set owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), np, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateAppServerNetworkPolicy, err)
	}

	logger.Info("App server NetworkPolicy reconciled", "name", np.Name, "result", result)
	return nil
}

// reconcileDeployment ensures the LCore deployment exists and is up to date.
func reconcileDeployment(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LCoreDeploymentName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), deployment, func() error {
		// Build the desired pod template spec
		podTemplateSpec, err := buildLCorePodTemplateSpec(h, ctx, instance)
		if err != nil {
			return err
		}

		// Selective field updates (avoid update loops)
		replicas := int32(1)
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: generateAppServerSelectorLabels(),
		}
		deployment.Spec.Template = podTemplateSpec

		// Set owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), deployment, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateAPIDeployment, err)
	}

	logger.Info("LCore Deployment reconciled", "name", deployment.Name, "result", result)
	return nil
}

// reconcileService ensures the OpenStack Lightspeed app server service exists and is up to date.
// Always uses the service-ca annotation for TLS certificate provisioning.
func reconcileService(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OpenStackLightspeedAppServerServiceName,
			Namespace: h.GetBeforeObject().GetNamespace(),
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), svc, func() error {
		// Selective field updates (preserves ClusterIP, ClusterIPs, etc.)
		svc.Spec.Selector = generateAppServerSelectorLabels()
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "https",
				Port:       OpenStackLightspeedAppServerServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(OpenStackLightspeedAppServerContainerPort),
			},
		}
		svc.Spec.Type = corev1.ServiceTypeClusterIP

		// Set service-ca annotation for TLS certificate provisioning
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		svc.Annotations[ServingCertSecretAnnotationKey] = OpenStackLightspeedCertsSecretName

		// Set owner reference
		return controllerutil.SetControllerReference(h.GetBeforeObject(), svc, h.GetScheme())
	})

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateAPIService, err)
	}

	logger.Info("App server Service reconciled", "name", svc.Name, "result", result)
	return nil
}

// reconcileTLSSecret waits for the TLS secret to be populated by the service-ca
// operator with tls.key and tls.crt data.
func reconcileTLSSecret(h *common_helper.Helper, ctx context.Context, _ *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()
	logger.Info("waiting for TLS secret to be populated", "name", OpenStackLightspeedCertsSecretName)

	secretKey := client.ObjectKey{
		Name:      OpenStackLightspeedCertsSecretName,
		Namespace: h.GetBeforeObject().GetNamespace(),
	}

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, ResourceCreationTimeout, true, func(ctx context.Context) (bool, error) {
		secret := &corev1.Secret{}
		if err := h.GetClient().Get(ctx, secretKey, secret); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		_, hasKey := secret.Data["tls.key"]
		_, hasCert := secret.Data["tls.crt"]
		return hasKey && hasCert, nil
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGetTLSSecret, err)
	}

	logger.Info("TLS secret is ready", "name", OpenStackLightspeedCertsSecretName)
	return nil
}

// reconcileDeleteClusterRoleBindingByLabels deletes ClusterRoleBinding resources by labels.
func reconcileDeleteClusterRoleBindingByLabels(h *common_helper.Helper, ctx context.Context, _ *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	labelSelector := labels.Set(generateAppServerSelectorLabels()).AsSelector()
	matchingLabels := client.MatchingLabelsSelector{Selector: labelSelector}
	deleteOptions := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: matchingLabels,
		},
	}

	if err := h.GetClient().DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, deleteOptions); err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteSARClusterRoleBinding, err)
	}

	logger.Info("SAR ClusterRoleBinding deleted successfully")
	return nil
}

// reconcileDeleteClusterRoleByLabels deletes ClusterRole resources by labels.
func reconcileDeleteClusterRoleByLabels(h *common_helper.Helper, ctx context.Context, _ *apiv1beta1.OpenStackLightspeed) error {
	logger := h.GetLogger()

	labelSelector := labels.Set(generateAppServerSelectorLabels()).AsSelector()
	matchingLabels := client.MatchingLabelsSelector{Selector: labelSelector}
	deleteOptions := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: matchingLabels,
		},
	}

	if err := h.GetClient().DeleteAllOf(ctx, &rbacv1.ClusterRole{}, deleteOptions); err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteSARClusterRole, err)
	}

	logger.Info("SAR ClusterRole deleted successfully")
	return nil
}
