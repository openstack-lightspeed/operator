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
	_ "embed"
	"fmt"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	"sigs.k8s.io/yaml"
)

// systemPrompt - system prompt tailored to the needs of OpenStack Lightspeed.
//
//go:embed assets/system_prompt.txt
var systemPrompt string

// getSystemPrompt returns the OpenStackLightspeed system prompt
func getSystemPrompt() string {
	return systemPrompt
}

// lcoreProvider represents an LLM provider configuration.
type lcoreProvider struct {
	Name                string
	URL                 string
	Type                string
	CredentialsSecret   string
	Models              []lcoreModel
	AzureDeploymentName string
	APIVersion          string
	WatsonProjectID     string
}

// lcoreModel represents a model configuration.
type lcoreModel struct {
	Name                 string
	MaxTokensForResponse int
}

// lcoreRAG represents RAG configuration.
type lcoreRAG struct {
	Image     string
	IndexPath string
	IndexID   string
}

// buildProvider creates an lcoreProvider from an OpenStackLightspeed instance.
func buildProvider(instance *apiv1beta1.OpenStackLightspeed) lcoreProvider {
	return lcoreProvider{
		Name:              OpenStackLightspeedDefaultProvider,
		URL:               instance.Spec.LLMEndpoint,
		Type:              instance.Spec.LLMEndpointType,
		CredentialsSecret: instance.Spec.LLMCredentials,
		Models: []lcoreModel{
			{
				Name:                 instance.Spec.ModelName,
				MaxTokensForResponse: instance.Spec.MaxTokensForResponse,
			},
		},
		AzureDeploymentName: instance.Spec.LLMDeploymentName,
		APIVersion:          instance.Spec.LLMAPIVersion,
		WatsonProjectID:     instance.Spec.LLMProjectID,
	}
}

// buildLCoreRAGConfigs builds the RAG configuration from an OpenStackLightspeed instance.
func buildLCoreRAGConfigs(instance *apiv1beta1.OpenStackLightspeed, ocpVersion string) []lcoreRAG {
	rags := []lcoreRAG{
		{
			Image:     instance.Spec.RAGImage,
			IndexPath: OpenStackLightspeedVectorDBPath,
		},
	}

	if ocpVersion != "" {
		rags = append(rags, lcoreRAG{
			Image:     instance.Spec.RAGImage,
			IndexPath: GetOCPVectorDBPath(ocpVersion),
			IndexID:   GetOCPIndexName(ocpVersion),
		})
	}

	return rags
}

func buildLCoreServiceConfig(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"host":         "0.0.0.0",
		"port":         OpenStackLightspeedAppServerContainerPort,
		"auth_enabled": true,
		"workers":      1,
		"color_log":    false,
		"access_log":   true,
		"tls_config": map[string]interface{}{
			"tls_certificate_path": "/etc/certs/lightspeed-tls/tls.crt",
			"tls_key_path":         "/etc/certs/lightspeed-tls/tls.key",
		},
	}
}

func buildLCoreLlamaStackConfig() map[string]interface{} {
	llamaStackConfig := map[string]interface{}{
		"use_as_library_client": false,
		"url":                   fmt.Sprintf("http://localhost:%d", LlamaStackContainerPort),
	}

	return llamaStackConfig
}

func buildLCoreUserDataCollectionConfig(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	feedbackEnabled := !instance.Spec.FeedbackDisabled
	transcriptsEnabled := !instance.Spec.TranscriptsDisabled

	return map[string]interface{}{
		"feedback_enabled":    feedbackEnabled,
		"feedback_storage":    LCoreUserDataMountPath + "/feedback",
		"transcripts_enabled": transcriptsEnabled,
		"transcripts_storage": LCoreUserDataMountPath + "/transcripts",
	}
}

func buildLCoreAuthenticationConfig(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"module": "k8s",
	}
}

func buildLCoreInferenceConfig(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"default_provider": OpenStackLightspeedDefaultProvider,
		"default_model":    instance.Spec.ModelName,
	}
}

// buildLCoreDatabaseConfig configures persistent database storage (PostgreSQL)
func buildLCoreDatabaseConfig(h *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"postgres": map[string]interface{}{
			"host":         PostgresServiceName + "." + h.GetBeforeObject().GetNamespace() + ".svc",
			"port":         PostgresServicePort,
			"db":           PostgresDefaultDbName,
			"user":         PostgresDefaultUser,
			"ssl_mode":     PostgresDefaultSSLMode,
			"gss_encmode":  "disable",
			"ca_cert_path": "/etc/certs/postgres-ca/service-ca.crt",

			// Environment variable substitution via llama_stack.core.stack.replace_env_vars
			"password": "${env.POSTGRES_PASSWORD}",

			// Separate schema for LCore to avoid conflicts with App Server
			"namespace": "lcore",
		},
	}
}

// buildLCoreCustomizationConfig configures system prompt customization
// Uses config field if set, otherwise falls back to default
func buildLCoreCustomizationConfig() map[string]interface{} {
	return map[string]interface{}{
		"system_prompt": getSystemPrompt(),
		// Prevent users from overriding via API
		"disable_query_system_prompt": true,
	}
}

// buildLCoreConversationCacheConfig configures chat history caching (PostgreSQL)
func buildLCoreConversationCacheConfig(h *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"type": "postgres",
		"postgres": map[string]interface{}{
			"host":         PostgresServiceName + "." + h.GetBeforeObject().GetNamespace() + ".svc",
			"port":         PostgresServicePort,
			"db":           PostgresDefaultDbName,
			"user":         PostgresDefaultUser,
			"password":     "${env.POSTGRES_PASSWORD}",
			"ssl_mode":     PostgresDefaultSSLMode,
			"gss_encmode":  "disable",
			"ca_cert_path": "/etc/certs/postgres-ca/service-ca.crt",
			"namespace":    "conversation_cache",
		},
	}
}

// buildLCoreConfigYAML assembles the complete Lightspeed Core Service configuration and converts to YAML.
// NOTE: MCP servers, quota handlers, and tools approval features are disabled for OpenStack Lightspeed.
func buildLCoreConfigYAML(h *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) (string, error) {
	// Build the complete config as a map
	config := map[string]interface{}{
		"name":                 "Lightspeed Core Service (LCS)",
		"service":              buildLCoreServiceConfig(h, instance),
		"llama_stack":          buildLCoreLlamaStackConfig(),
		"user_data_collection": buildLCoreUserDataCollectionConfig(h, instance),
		"authentication":       buildLCoreAuthenticationConfig(h, instance),
		"inference":            buildLCoreInferenceConfig(h, instance),
		"database":             buildLCoreDatabaseConfig(h, instance),
		"customization":        buildLCoreCustomizationConfig(),
		"conversation_cache":   buildLCoreConversationCacheConfig(h, instance),
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LCore config to YAML: %w", err)
	}

	return string(yamlBytes), nil
}
