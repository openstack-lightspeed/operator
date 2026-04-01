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

package v1beta1

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OpenStackLightspeedContainerImage is the fall-back container image for OpenStackLightspeed
	OpenStackLightspeedContainerImage = "quay.io/openstack-lightspeed/rag-content:os-docs-2025.2"

	// LCoreContainerImage is the fall-back container image for LCore
	LCoreContainerImage = "quay.io/lightspeed-core/lightspeed-stack:latest"

	// ExporterContainerImage is the fall-back container image for the Dataverse Exporter
	ExporterContainerImage = "quay.io/lightspeed-core/lightspeed-to-dataverse-exporter:latest"

	// PostgresContainerImage is the fall-back container image for PostgreSQL
	PostgresContainerImage = "registry.redhat.io/rhel9/postgresql-16:latest"

	// MaxTokensForResponseDefault is the default maximum number of tokens that should be used for response
	MaxTokensForResponseDefault = 2048
)

// OpenStackLightspeedSpec defines the desired state of OpenStackLightspeed
type OpenStackLightspeedSpec struct {
	OpenStackLightspeedCore `json:",inline"`

	// +kubebuilder:validation:Optional
	// ContainerImage for the OpenStack Lightspeed RAG container (will be set to environmental default if empty)
	RAGImage string `json:"ragImage"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Enables automatic OCP documentation based on cluster version
	EnableOCPRAG bool `json:"enableOCPRAG,omitempty"`

	// +kubebuilder:validation:Optional
	// Allows forcing a specific OCP version instead of auto-detection.
	// Format should be like "4.15", "4.16", etc.
	OCPRAGVersionOverride string `json:"ocpVersionOverride,omitempty"`
}

// OpenStackLightspeedCore defines the desired state of OpenStackLightspeed
type OpenStackLightspeedCore struct {
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LLM Endpoint"
	// URL pointing to the LLM
	LLMEndpoint string `json:"llmEndpoint"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=azure_openai;bam;openai;watsonx;rhoai_vllm;rhelai_vllm;fake_provider
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Provider Type"
	// Type of the provider serving the LLM
	LLMEndpointType string `json:"llmEndpointType"`

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Name"
	// Name of the model to use at the API endpoint provided in LLMEndpoint
	ModelName string `json:"modelName"`

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LLM Credentials Secret"
	// Secret name containing API token for the LLMEndpoint. The secret must contain
	// a field named "apitoken" which holds the token value.
	LLMCredentials string `json:"llmCredentials"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS CA Certificate Bundle"
	// Configmap name containing a CA Certificates bundle
	TLSCACertBundle string `json:"tlsCACertBundle"`

	// +kubebuilder:validation:Optional
	// MaxTokensForResponse defines the maximum number of tokens to be used for the response generation
	MaxTokensForResponse int `json:"maxTokensForResponse,omitempty"`

	// +kubebuilder:validation:Optional
	// Project ID for LLM providers that require it (e.g., WatsonX)
	LLMProjectID string `json:"llmProjectID,omitempty"`

	// +kubebuilder:validation:Optional
	// Deployment name for LLM providers that require it (e.g., Microsoft Azure OpenAI)
	LLMDeploymentName string `json:"llmDeploymentName,omitempty"`

	// +kubebuilder:validation:Optional
	// LLM API Version for LLM providers that require it (e.g., Microsoft Azure OpenAI)
	LLMAPIVersion string `json:"llmAPIVersion,omitempty"`

	// +kubebuilder:validation:Optional
	// Disable feedback collection
	FeedbackDisabled bool `json:"feedbackDisabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Disable conversation transcripts collection
	TranscriptsDisabled bool `json:"transcriptsDisabled,omitempty"`
}

// OpenStackLightspeedStatus defines the observed state of OpenStackLightspeed
type OpenStackLightspeedStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	// ActiveOCPRAGVersion contains the OCP version being used for RAG configuration
	// Will be one of: "4.16", "4.18", "latest", or empty if OCP RAG is disabled
	ActiveOCPRAGVersion string `json:"activeOCPRAGVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"
// +operator-sdk:csv:customresourcedefinitions:resources={{Deployment,v1,lightspeed-stack-deployment}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Deployment,v1,lightspeed-postgres-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Service,v1,lightspeed-app-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Service,v1,lightspeed-postgres-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,llama-stack-config}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,lightspeed-stack-config}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,lightspeed-postgres-conf}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,lightspeed-postgres-secret}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,lightspeed-postgres-bootstrap}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,metrics-reader-token}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,lightspeed-tls}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,lightspeed-postgres-certs}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ServiceAccount,v1,lightspeed-app-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{NetworkPolicy,v1,lightspeed-app-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{NetworkPolicy,v1,lightspeed-postgres-server}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ClusterRole,v1,lightspeed-app-server-sar-role}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ClusterRoleBinding,v1,lightspeed-app-server-sar-role-binding}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Subscription,v1alpha1}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ClusterServiceVersion,v1alpha1}}
// +operator-sdk:csv:customresourcedefinitions:resources={{InstallPlan,v1alpha1}}

// OpenStackLightspeed is the Schema for the openstacklightspeeds API
type OpenStackLightspeed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackLightspeedSpec   `json:"spec,omitempty"`
	Status OpenStackLightspeedStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackLightspeedList contains a list of OpenStackLightspeed
type OpenStackLightspeedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackLightspeed `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackLightspeed{}, &OpenStackLightspeedList{})
}

// IsReady - returns true if OpenStackLightspeed is reconciled successfully
func (instance OpenStackLightspeed) IsReady() bool {
	return instance.Status.Conditions.IsTrue(OpenStackLightspeedReadyCondition)
}

type OpenStackLightspeedDefaults struct {
	RAGImageURL          string
	LCoreImageURL        string
	ExporterImageURL     string
	PostgresImageURL     string
	MaxTokensForResponse int
}

var OpenStackLightspeedDefaultValues OpenStackLightspeedDefaults

// SetupDefaults - initializes OpenStackLightspeedDefaultValues with default values from env vars
func SetupDefaults() {
	// Acquire environmental defaults and initialize OpenStackLightspeed defaults with them
	openStackLightspeedDefaults := OpenStackLightspeedDefaults{
		RAGImageURL: util.GetEnvVar(
			"RELATED_IMAGE_OPENSTACK_LIGHTSPEED_IMAGE_URL_DEFAULT", OpenStackLightspeedContainerImage),
		LCoreImageURL: util.GetEnvVar(
			"RELATED_IMAGE_LCORE_IMAGE_URL_DEFAULT", LCoreContainerImage),
		ExporterImageURL: util.GetEnvVar(
			"RELATED_IMAGE_EXPORTER_IMAGE_URL_DEFAULT", ExporterContainerImage),
		PostgresImageURL: util.GetEnvVar(
			"RELATED_IMAGE_POSTGRES_IMAGE_URL_DEFAULT", PostgresContainerImage),
		MaxTokensForResponse: MaxTokensForResponseDefault,
	}

	OpenStackLightspeedDefaultValues = openStackLightspeedDefaults
}
