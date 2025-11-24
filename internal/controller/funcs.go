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
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"

	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// OpenStackLightspeedDefaultProvider - contains default name for the provider created in OLSConfig
	// by openstack-operator.
	OpenStackLightspeedDefaultProvider = "openstack-lightspeed-provider"

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

	return true, nil
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

// PatchOLSConfig patches OLSConfig with information from OpenStackLightspeed instance.
func PatchOLSConfig(
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
	olsConfig *uns.Unstructured,
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
	if err := uns.SetNestedSlice(olsConfig.Object, providersPatch, "spec", "llm", "providers"); err != nil {
		return err
	}

	// Patch the RAG section
	// NOTE(lucasagomes): We don't need indexID here because the tag on our RAG images
	// already matches the indexID that the Vector DB used when it was built. OLS leverages
	// that to set the right index.
	openstackRAG := []interface{}{
		map[string]interface{}{
			"image":     instance.Spec.RAGImage,
			"indexPath": OpenStackLightspeedVectorDBPath,
		},
	}

	if err := uns.SetNestedSlice(olsConfig.Object, openstackRAG, "spec", "ols", "rag"); err != nil {
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

	// Add OpenStack finalizers
	if !controllerutil.AddFinalizer(olsConfig, helper.GetFinalizer()) && instance.Status.Conditions == nil {
		return fmt.Errorf("cannot add finalizer")
	}

	return nil
}

// IsOLSConfigReady returns true if required conditions are true for OLSConfig
func IsOLSConfigReady(ctx context.Context, helper *common_helper.Helper) (bool, error) {
	olsConfig, err := GetOLSConfig(ctx, helper)
	if err != nil {
		return false, err
	}

	olsConfigStatusList, found, err := uns.NestedSlice(olsConfig.Object, "status", "conditions")
	if !found {
		return false, err
	}

	jsonData, err := json.Marshal(olsConfigStatusList)
	if err != nil {
		return false, fmt.Errorf("failed to marshal OLSConfig status: %w", err)
	}

	var OLSConfigConditions []metav1.Condition
	err = json.Unmarshal(jsonData, &OLSConfigConditions)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal JSON containing condition.Conditions: %w", err)
	}

	requiredConditionTypes := []string{"ConsolePluginReady", "CacheReady", "ApiReady", "Reconciled"}
	for _, OLSConfigCondition := range OLSConfigConditions {
		for _, requiredConditionType := range requiredConditionTypes {
			if OLSConfigCondition.Type == requiredConditionType && OLSConfigCondition.Status != metav1.ConditionTrue {
				return false, OLSConfigPing(ctx, helper)
			}
		}
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
