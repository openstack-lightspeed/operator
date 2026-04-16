/*
Copyright 2025.

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
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	_ "embed"

	appsv1 "k8s.io/api/apps/v1"

	common_cm "github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// OpenStackLightspeedDefaultProvider - contains default name for the provider created in OLSConfig
	// by openstack-operator.
	OpenStackLightspeedDefaultProvider = "openstack-lightspeed-provider"

	// OpenStackLightspeedChecksumAnnotation - is the annotation key used to store the checksum of resources
	OpenStackLightspeedChecksumAnnotation = "openstack.org/checksum"

	// OpenStackLightspeedOwnerIDLabel - name of a label that contains ID of OpenStackLightspeed instance
	// that manages the OLSConfig.
	OpenStackLightspeedOwnerIDLabel = "openstack.org/lightspeed-owner-id"

	// OpenStackLightspeedVectorDBPath - path inside of the container image where the vector DB are
	// located
	OpenStackLightspeedVectorDBPath = "/rag/vector_db/os_product_docs"

	// OpenStackLightspeedJobName - name of the pod that is used to discover environment variables inside of the RAG
	// container image
	OpenStackLightspeedJobName = "openstack-lightspeed"

	// OLSConfigName - OLS forbids other name for OLSConfig instance than OLSConfigName
	OLSConfigName = "cluster"
)

// systemPrompt - system prompt tailored to the needs of OpenStack Lightspeed. It overwrites the default OLS prompt.
//
//go:embed system_prompt.txt
var systemPrompt string

// GetSystemPrompt returns the OpenStackLightspeed system prompt
func GetSystemPrompt() string {
	return systemPrompt
}

// RemoveOLSConfig attempts to remove the OLSConfig custom resource if it exists
// and is managed by the given OpenStackLightspeed instance. It first fetches the OLSConfig,
// checks whether the current OpenStackLightspeed instance is the owner (via label check),
// and if so, removes the finalizer and deletes the OLSConfig resource.
// Returns (true, nil) if the OLSConfig is not found (indicating it has already been deleted).
// Returns (true, nil) if the resource was deleted successfully, or (false, error) if any error occurs.
func RemoveOLSConfig(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	olsConfig, err := GetOLSConfig(ctx, helper)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return false, err
	} else if err != nil && k8s_errors.IsNotFound(err) {
		return true, nil
	}

	_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), &olsConfig, func() error {
		ownerLabel := olsConfig.GetLabels()[OpenStackLightspeedOwnerIDLabel]
		isInstanceOwnedOLSConfig := ownerLabel == string(instance.GetObjectMeta().GetUID())

		if ownerLabel == "" || !isInstanceOwnedOLSConfig {
			helper.GetLogger().Info("Skipping OLSConfig deletion as it is not managed by the OpenStackLightspeed instance")
			return nil
		}

		if ok := controllerutil.RemoveFinalizer(&olsConfig, helper.GetFinalizer()); !ok {
			return fmt.Errorf("remove finalizer failed")
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	err = helper.GetClient().Delete(ctx, &olsConfig)
	if err != nil {
		return false, err
	}

	_, err = GetOLSConfig(ctx, helper)
	if err != nil && k8s_errors.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return false, nil
}

// GetOLSConfig returns OLSConfig if there is one present in the cluster.
func GetOLSConfig(ctx context.Context, helper *common_helper.Helper) (uns.Unstructured, error) {
	OLSConfigGVR := schema.GroupVersionResource{
		Group:    "ols.openshift.io",
		Version:  "v1alpha1",
		Resource: "olsconfigs",
	}

	OLSConfigList := &uns.UnstructuredList{}
	OLSConfigList.SetGroupVersionKind(OLSConfigGVR.GroupVersion().WithKind("OLSConfig"))
	err := helper.GetClient().List(ctx, OLSConfigList)
	if err != nil {
		return uns.Unstructured{}, err
	}

	if len(OLSConfigList.Items) > 0 {
		return OLSConfigList.Items[0], nil
	}

	return uns.Unstructured{}, k8s_errors.NewNotFound(
		schema.GroupResource{Group: "ols.openshifg.io", Resource: "olsconfigs"},
		"OLSConfig")
}

// BuildRAGConfigs builds the RAG configuration array.
// OpenStack RAG is always included first.
// OCP RAG is added if ocpVersion is provided.
func BuildRAGConfigs(instance *apiv1beta1.OpenStackLightspeed, ocpVersion string) []interface{} {
	rags := []interface{}{
		// OpenStack RAG
		map[string]interface{}{
			"image":     instance.Spec.RAGImage,
			"indexPath": OpenStackLightspeedVectorDBPath,
		},
	}

	// Add OCP RAG if enabled
	if ocpVersion != "" {
		rags = append(rags, map[string]interface{}{
			"image":     instance.Spec.RAGImage,
			"indexPath": GetOCPVectorDBPath(ocpVersion),
			"indexID":   GetOCPIndexName(ocpVersion),
		})
	}

	return rags
}

// PatchOLSConfig patches OLSConfig with information from OpenStackLightspeed instance.
func PatchOLSConfig(
	ctx context.Context,
	helper *common_helper.Helper,
	scheme *runtime.Scheme,
	instance *apiv1beta1.OpenStackLightspeed,
	olsConfig *uns.Unstructured,
	dynamicWatchCRD *DynamicWatchCRD,
) error {
	// Patch the Providers section
	providersPatch := []interface{}{
		map[string]interface{}{
			"credentialsSecretRef": map[string]interface{}{
				"name": instance.Spec.LLMCredentials,
			},
			"models": []interface{}{
				map[string]interface{}{
					"name": instance.Spec.ModelName,
					"parameters": map[string]interface{}{
						"maxTokensForResponse": float64(instance.Spec.MaxTokensForResponse), // unstructured JSON numbers default to float64
					},
				},
			},
			"name": OpenStackLightspeedDefaultProvider,
			"type": instance.Spec.LLMEndpointType,
			"url":  instance.Spec.LLMEndpoint,
		},
	}

	provider := providersPatch[0].(map[string]interface{})
	if instance.Spec.LLMProjectID != "" {
		if err := uns.SetNestedField(provider, instance.Spec.LLMProjectID, "projectID"); err != nil {
			return err
		}
	}

	if instance.Spec.LLMDeploymentName != "" {
		if err := uns.SetNestedField(provider, instance.Spec.LLMDeploymentName, "deploymentName"); err != nil {
			return err
		}
	}

	if instance.Spec.LLMAPIVersion != "" {
		if err := uns.SetNestedField(provider, instance.Spec.LLMAPIVersion, "apiVersion"); err != nil {
			return err
		}
	}

	if err := uns.SetNestedSlice(olsConfig.Object, providersPatch, "spec", "llm", "providers"); err != nil {
		return err
	}

	// Patch the RAG section
	// Build RAG array with priorities using BuildRAGConfigs
	ragConfigs := BuildRAGConfigs(instance, instance.Status.ActiveOCPRAGVersion)

	if err := uns.SetNestedSlice(olsConfig.Object, ragConfigs, "spec", "ols", "rag"); err != nil {
		return err
	}

	if instance.Spec.TLSCACertBundle != "" {
		tlsCaCertBundle := instance.Spec.TLSCACertBundle
		err := uns.SetNestedField(olsConfig.Object, tlsCaCertBundle, "spec", "ols", "additionalCAConfigMapRef", "name")
		if err != nil {
			return err
		}
	}

	modelName := instance.Spec.ModelName
	err := uns.SetNestedField(olsConfig.Object, modelName, "spec", "ols", "defaultModel")
	if err != nil {
		return err
	}

	err = uns.SetNestedField(olsConfig.Object, OpenStackLightspeedDefaultProvider, "spec", "ols", "defaultProvider")
	if err != nil {
		return err
	}

	// Disable the OCP RAG
	// TODO(lucasagomes): Remove this once we have a "query router" that can
	// handle multiple RAGs nicely
	err = uns.SetNestedField(olsConfig.Object, true, "spec", "ols", "byokRAGOnly")
	if err != nil {
		return err
	}

	// Disable or enable feedback collection
	err = uns.SetNestedField(olsConfig.Object, instance.Spec.FeedbackDisabled, "spec", "ols", "userDataCollection", "feedbackDisabled")
	if err != nil {
		return err
	}

	// Disable or enable transcripts collection
	err = uns.SetNestedField(olsConfig.Object, instance.Spec.TranscriptsDisabled, "spec", "ols", "userDataCollection", "transcriptsDisabled")
	if err != nil {
		return err
	}

	err = uns.SetNestedField(olsConfig.Object, GetSystemPrompt(), "spec", "ols", "querySystemPrompt")
	if err != nil {
		return err
	}

	// Add info which OpenStackLightspeed instance owns the OLSConfig
	labels := olsConfig.GetLabels()
	updatedLabels := map[string]interface{}{
		OpenStackLightspeedOwnerIDLabel: string(instance.GetUID()),
	}
	for k, v := range labels {
		updatedLabels[k] = v
	}

	err = uns.SetNestedField(olsConfig.Object, updatedLabels, "metadata", "labels")
	if err != nil {
		return err
	}

	available, err := IsDynamicCRDReady(
		helper,
		*dynamicWatchCRD,
		&openstackv1.OpenStackControlPlane{},
	)
	if err != nil {
		return err
	}

	if available {
		OpenShiftMCPServerConfig := map[string]interface{}{
			"name": "openshift-lightspeed-mcp",
			"streamableHTTP": map[string]interface{}{
				"url": fmt.Sprintf("%s/openshift", GetMCPServerURL()),
				"headers": map[string]interface{}{
					"OCP_TOKEN": "kubernetes",
				},
			},
		}

		OpenStackMCPServerConfig := map[string]interface{}{
			"name": "openstack-lightspeed-mcp",
			"streamableHTTP": map[string]interface{}{
				"url": fmt.Sprintf("%s/openstack", GetMCPServerURL()),
			},
		}

		MCPServersConfig := []interface{}{OpenShiftMCPServerConfig, OpenStackMCPServerConfig}
		err = uns.SetNestedSlice(olsConfig.Object, MCPServersConfig, "spec", "mcpServers")
		if err != nil {
			return err
		}

		// Add featureGates to enable "MCPServe"
		err = uns.SetNestedSlice(olsConfig.Object, []interface{}{"MCPServer"}, "spec", "featureGates")
		if err != nil {
			return err
		}
	} else {
		err = uns.SetNestedSlice(olsConfig.Object, []interface{}{}, "spec", "mcpServers")
		if err != nil {
			return err
		}
		err = uns.SetNestedSlice(olsConfig.Object, []interface{}{}, "spec", "featureGates")
		if err != nil {
			return err
		}
	}

	// Add OpenStack finalizers
	if !controllerutil.AddFinalizer(olsConfig, helper.GetFinalizer()) && instance.Status.Conditions == nil {
		return fmt.Errorf("cannot add finalizer")
	}

	return nil
}

// IsOLSConfigReady returns true if OLSConfig's overallStatus is Ready
func IsOLSConfigReady(ctx context.Context, helper *common_helper.Helper) (bool, error) {
	olsConfig, err := GetOLSConfig(ctx, helper)
	if err != nil {
		return false, err
	}

	overallStatus, found, err := uns.NestedString(olsConfig.Object, "status", "overallStatus")
	if err != nil {
		return false, err
	}

	if !found || overallStatus != "Ready" {
		return false, OLSConfigPing(ctx, helper)
	}

	return true, nil
}

// IsOwnedBy returns true if 'object' is owned by 'owner' based on OwnerReference UID.
func IsOwnedBy(object metav1.Object, owner metav1.Object) bool {
	for _, ref := range object.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

// GetRawClient returns a raw client that is not restricted to WATCH_NAMESPACE.
// This is useful for operations that need to query resources across all namespaces
// cluster wide.
func GetRawClient(helper *common_helper.Helper) (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	rawClient, err := client.New(cfg, client.Options{Scheme: helper.GetScheme()})
	if err != nil {
		return nil, err
	}

	return rawClient, nil
}

// OLSConfigPing adds a random label to the OLSConfig to trigger a reconciliation
// by the OpenShift Lightspeed operator. This causes the operator to update the Status field.
// Note: This is a workaround for a current limitation—when the OLS operator is installed
// in the openstack-lightspeed namespace, it does not automatically update the OLSConfig
// status as expected.
func OLSConfigPing(ctx context.Context, helper *common_helper.Helper) error {
	const randomLabelKey = "openstack-lightspeed/ping"

	olsConfig, err := GetOLSConfig(ctx, helper)
	if err != nil {
		return err
	}

	labels := olsConfig.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[randomLabelKey] = strconv.Itoa(rand.Int())
	olsConfig.SetLabels(labels)

	if err := helper.GetClient().Update(ctx, &olsConfig); err != nil {
		return err
	}
	return nil
}

// GetCRDName returns the name of the CustomResourceDefinition (CRD) for a given
// GroupVersionKind (GVK). The CRD name is constructed as "<Kind>s.<Group>" string.
//
// NOTE(lpiwowar): This approach is NOT perfect but it is sufficient for
// OpenStackControlPlane use cases and potentially other CRDs. For broader use,
// we should consider implementing a more robust transformation from GroupVersionKind
// to CRD name.
func GetCRDName(gvk schema.GroupVersionKind) string {
	return fmt.Sprintf("%ss.%s", strings.ToLower(gvk.Kind), gvk.Group)
}

// IsCRDEstablished checks if a CRD exists and is in "Established" state (ready for use)
// It returns the following values:
//   - (true, nil) if the CRD exists and is established
//   - (false, nil) if the CRD doesn't exist
//   - (false, error) for other errors
func IsCRDEstablished(ctx context.Context, helper *common_helper.Helper, gvk schema.GroupVersionKind) (bool, error) {
	crdName := GetCRDName(gvk)
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := helper.GetClient().Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err != nil && k8s_errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}

// CreateOwnerReference creates an owner reference for the given OpenStackLightspeed instance.
func CreateOwnerReference(instance *apiv1beta1.OpenStackLightspeed) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         instance.APIVersion,
		Kind:               instance.Kind,
		Name:               instance.Name,
		UID:                instance.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}
}

// CopyResource copies a resources (supported: Secret, ConfigMap) from one namespace
// to another, applying the specified owner references and computing checksums.
// It performs the following steps:
//
//  1. Fetches the source object identified by its namespace and name.
//  2. Creates or patches the target object in the target namespace with the same data and relevant metadata.
//  3. Sets the given owner references on the copied object.
//  4. Recomputes and sets checksum annotation using the lib-common Hash methods.
//
// Returns the copied object if successful, or an error if the operation fails
// or the type is unsupported.
func CopyResource(
	ctx context.Context,
	helper *common_helper.Helper,
	sourceObject client.Object,
	targetObject client.Object,
	ownerReference []metav1.OwnerReference,
) (client.Object, error) {
	objectKey := types.NamespacedName{
		Namespace: sourceObject.GetNamespace(),
		Name:      sourceObject.GetName(),
	}

	err := helper.GetClient().Get(ctx, objectKey, sourceObject)
	if err != nil {
		return nil, err
	}

	var copyObject client.Object

	switch object := sourceObject.(type) {
	case *corev1.Secret:
		copySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      targetObject.GetName(),
				Namespace: targetObject.GetNamespace(),
			},
		}

		_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), copySecret, func() error {
			copySecret.Data = object.Data
			copySecret.StringData = object.StringData
			copySecret.Type = object.Type
			copySecret.SetOwnerReferences(ownerReference)

			checksum, err := common_secret.Hash(copySecret)
			if err != nil {
				return err
			}

			SetChecksumAnnotation(copySecret, checksum)

			return nil
		})

		copyObject = copySecret
	case *corev1.ConfigMap:
		copyConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      targetObject.GetName(),
				Namespace: targetObject.GetNamespace(),
			},
		}

		_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), copyConfigMap, func() error {
			copyConfigMap.Data = object.Data
			copyConfigMap.BinaryData = object.BinaryData
			copyConfigMap.SetOwnerReferences(ownerReference)

			checksum, err := common_cm.Hash(copyConfigMap)
			if err != nil {
				return err
			}

			SetChecksumAnnotation(copyConfigMap, checksum)

			return nil
		})

		copyObject = copyConfigMap
	default:
		return nil, errors.New("cannot copy resource (invalid type)")
	}

	if err != nil {
		return nil, err
	}

	return copyObject, nil
}

// SetChecksumAnnotation sets or updates the checksum annotation on the provided
// object.This function adds or overwrites only the OpenStackLightspeedChecksumAnnotation
// key, preserving any existing annotations.
func SetChecksumAnnotation(object client.Object, checksum string) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[OpenStackLightspeedChecksumAnnotation] = checksum
	object.SetAnnotations(annotations)
}

// GetChecksumAnnotation retrieves the checksum annotation from the given object.
// If the annotation is not found, it returns an empty string.
func GetChecksumAnnotation(object client.Object) string {
	annotations := object.GetAnnotations()
	if annotations == nil {
		return ""
	}

	checksum, ok := annotations[OpenStackLightspeedChecksumAnnotation]
	if !ok {
		return ""
	}

	return checksum
}

// GetDeploymentVolumeSection returns a pointer to the Volume in the Deployment's PodSpec
// whose name matches the given volumeSectionName. If no such volume is found, it returns nil.
// This is useful for patching or inspecting a named volume within a Deployment's specification.
func GetDeploymentVolumeSection(deployment appsv1.Deployment, volumeSectionName string) *corev1.Volume {
	for i, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == volumeSectionName {
			return &deployment.Spec.Template.Spec.Volumes[i]
		}
	}

	return nil
}

// GetObjectGVKs retrieves the GroupVersionKinds for an clientObject using the
// provided runtime.Scheme. It returns the list of GVKs and any error encountered.
func GetObjectGVKs(helper *common_helper.Helper, object client.Object) ([]schema.GroupVersionKind, error) {
	gvks, _, err := helper.GetScheme().ObjectKinds(object)
	if err != nil {
		return nil, err
	}

	return gvks, nil
}

// IsDynamicCRDReady checks whether all GroupVersionKinds (GVKs) associated with
// the given object are being watched and have been observed as ready by the
// dynamic watch. It returns true only if all relevant GVKs are present and marked as seen.
func IsDynamicCRDReady(
	helper *common_helper.Helper,
	dynamicWatchCRD DynamicWatchCRD,
	object client.Object,
) (bool, error) {
	gvks, err := GetObjectGVKs(helper, object)
	if err != nil {
		return false, err
	}

	for _, gvk := range gvks {
		seen, exists := dynamicWatchCRD[gvk]
		if !exists {
			return false, fmt.Errorf("GVK %v not found in DynamicWatchCRD map", gvk)
		}

		if !seen.Load() {
			return false, nil
		}
	}

	return true, nil
}
