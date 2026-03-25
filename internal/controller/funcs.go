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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	_ "embed"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
