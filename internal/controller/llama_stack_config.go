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
	"strings"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	"sigs.k8s.io/yaml"
)

func buildLlamaStackCoreConfig(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"version": "2",

		// image_name is a semantic identifier for the llama-stack configuration
		// Note: Does NOT affect PostgreSQL database name (llama-stack uses hardcoded "llamastack")
		"image_name": "openstack-lightspeed-configuration",

		// Minimal APIs for RAG + MCP: agents (for MCP), files, inference, safety (required by agents),
		// telemetry, tool_runtime, vector_io.
		"apis": []string{
			"agents",
			"files",
			"inference",
			"safety",
			"tool_runtime",
			"vector_io",
		},
		"benchmarks":             []interface{}{},
		"container_image":        nil,
		"datasets":               []interface{}{},
		"external_providers_dir": nil,
		"inference_store": map[string]interface{}{
			"db_path": ".llama/distributions/ollama/inference_store.db",
			"type":    "sqlite",
		},
		"logging": nil,
		"metadata_store": map[string]interface{}{
			"db_path":   "/tmp/llama-stack/registry.db",
			"namespace": nil,
			"type":      "sqlite",
		},
	}
}

func buildLlamaStackFileProviders(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"provider_id":   "localfs",
			"provider_type": "inline::localfs",
			"config": map[string]interface{}{
				"storage_dir": "/tmp/llama-stack-files",
				"metadata_store": map[string]interface{}{
					"backend":    "sql_default",
					"namespace":  "files_metadata",
					"table_name": "files_metadata",
				},
			},
		},
	}
}

func buildLlamaStackAgentProviders(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"provider_id":   "meta-reference",
			"provider_type": "inline::meta-reference",
			"config": map[string]interface{}{
				"persistence": map[string]interface{}{
					"agent_state": map[string]interface{}{
						"backend":    "kv_default",
						"table_name": "agent_state",
						"namespace":  "agent_state",
					},
					"responses": map[string]interface{}{
						"backend":    "sql_default",
						"table_name": "agent_responses",
						"namespace":  "agent_responses",
					},
				},
			},
		},
	}
}

func buildLlamaStackInferenceProviders(_ *common_helper.Helper, _ context.Context, instance *apiv1beta1.OpenStackLightspeed) ([]interface{}, error) {
	// Always include sentence-transformers for embeddings
	providers := []interface{}{
		map[string]interface{}{
			"provider_id":   "sentence-transformers",
			"provider_type": "inline::sentence-transformers",
			"config":        map[string]interface{}{},
		},
	}

	// Add the LLM provider from the instance spec
	{
		provider := buildProvider(instance)
		providerConfig := map[string]interface{}{
			"provider_id": provider.Name,
		}

		// Convert provider name to valid environment variable name
		envVarName := providerNameToEnvVarName(provider.Name)

		// Map provider types to Llama Stack provider types
		switch provider.Type {
		case "openai", "rhoai_vllm", "rhelai_vllm":
			config := map[string]interface{}{}
			// Determine the appropriate Llama Stack provider type:
			//  - OpenAI uses remote::openai
			//  - vLLM uses remote::vllm
			var apiKeyField string
			if provider.Type == "openai" {
				providerConfig["provider_type"] = "remote::openai"
				apiKeyField = "api_key"
			} else {
				providerConfig["provider_type"] = "remote::vllm"
				apiKeyField = "api_token"
			}
			// Llama Stack will substitute ${env.VAR_NAME} with the actual env var value
			config[apiKeyField] = fmt.Sprintf("${env.%s%s}", envVarName, EnvVarSuffixAPIKey)

			// Add custom URL if specified
			if provider.URL != "" {
				config["base_url"] = provider.URL
			}

			providerConfig["config"] = config

		case "azure_openai":
			providerConfig["provider_type"] = "remote::azure"
			config := map[string]interface{}{}

			// Azure supports both API key and client credentials authentication
			// Always include api_key (required by LiteLLM's Pydantic validation)
			config["api_key"] = fmt.Sprintf("${env.%s_API_KEY}", envVarName)

			// Also include client credentials fields (will be empty if not using client credentials)
			config["client_id"] = fmt.Sprintf("${env.%s_CLIENT_ID:=}", envVarName)
			config["tenant_id"] = fmt.Sprintf("${env.%s_TENANT_ID:=}", envVarName)
			config["client_secret"] = fmt.Sprintf("${env.%s_CLIENT_SECRET:=}", envVarName)

			// Azure-specific fields
			if provider.AzureDeploymentName != "" {
				config["deployment_name"] = provider.AzureDeploymentName
			}
			if provider.APIVersion != "" {
				config["api_version"] = provider.APIVersion
			}
			if provider.URL != "" {
				config["api_base"] = provider.URL
			}
			providerConfig["config"] = config

		case "watsonx", "bam":
			// These providers are not supported by Llama Stack
			// They are handled directly by lightspeed-stack (LCS), not Llama Stack
			return nil, fmt.Errorf("provider type '%s' (provider '%s') is not currently supported by Llama Stack. Supported types: openai, azure_openai, rhoai_vllm, rhelai_vllm", provider.Type, provider.Name)

		default:
			// Unknown provider type
			return nil, fmt.Errorf("unknown provider type '%s' (provider '%s'). Supported types: openai, azure_openai, rhoai_vllm, rhelai_vllm", provider.Type, provider.Name)
		}

		providers = append(providers, providerConfig)
	}

	return providers, nil
}

// Safety API - Required by agents provider (for MCP)
func buildLlamaStackSafety(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"provider_id":   "llama-guard",
			"provider_type": "inline::llama-guard",
			"config": map[string]interface{}{
				"excluded_categories": []interface{}{},
			},
		},
	}
}

func buildLlamaStackToolRuntime(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"provider_id":   "model-context-protocol",
			"provider_type": "remote::model-context-protocol",
			"config":        map[string]interface{}{},
		},
		map[string]interface{}{
			"provider_id":   "rag-runtime",
			"provider_type": "inline::rag-runtime",
			"config":        map[string]interface{}{},
		},
	}
}

func buildLlamaStackVectorDB(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"provider_id":   "faiss",
			"provider_type": "inline::faiss",
			"config": map[string]interface{}{
				"kvstore": map[string]interface{}{
					"backend":    "sql_default",
					"table_name": "vector_store",
				},
				"persistence": map[string]interface{}{
					"backend":   "rag_backend",
					"namespace": "vector_io:faiss",
				},
			},
		},
	}
}

func buildLlamaStackServerConfig(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"auth":         nil,
		"host":         "0.0.0.0", // Listen on all interfaces so lightspeed-stack container can connect
		"port":         LlamaStackContainerPort,
		"quota":        nil,
		"tls_cafile":   nil,
		"tls_certfile": nil,
		"tls_keyfile":  nil,
	}
}

// buildLlamaStackStorage configures persistent storage for Llama Stack
func buildLlamaStackStorage(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	// Define storage backends - SQL only
	backends := map[string]interface{}{
		"sql_default": map[string]interface{}{
			"type":    "sql_sqlite",
			"db_path": "/tmp/llama-stack/sql_store.db",
		},
		"kv_default": map[string]interface{}{
			"type":    "kv_sqlite",
			"db_path": "/tmp/llama-stack/kv_store.db",
		},
		"postgres_backend": map[string]interface{}{
			"type":     "sql_postgres",
			"host":     fmt.Sprintf("lightspeed-postgres-server.%s.svc", instance.GetNamespace()),
			"port":     PostgresServicePort,
			"user":     "postgres",
			"password": "${env.POSTGRES_PASSWORD}",
			// Note: Database name is HARDCODED to "llamastack" in llama-stack's postgres adapter
			// Not configurable - llama-stack ignores image_name for database selection
			"ssl_mode":     "require",
			"ca_cert_path": "/etc/certs/postgres-ca/service-ca.crt",
			"gss_encmode":  "disable",
		},
		"rag_backend": map[string]interface{}{
			"type":    "kv_sqlite",
			"db_path": "/rag/rag-0/vector_db/os_product_docs/faiss_store.db",
		},
	}

	// Map data stores to backends - all use SQL with table_name
	stores := map[string]interface{}{
		"metadata": map[string]interface{}{
			"namespace": "registry",
			"backend":   "kv_default",
		},
		"inference": map[string]interface{}{
			"table_name": "inference_store",
			"backend":    "sql_default",
		},
		"conversations": map[string]interface{}{
			"table_name": "openai_conversations", // Required by config schema but ignored - llama-stack uses hardcoded names
			"backend":    "postgres_backend",
		},
	}

	return map[string]interface{}{
		"backends": backends,
		"stores":   stores,
	}
}

func buildLlamaStackVectorStores(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	var vectorDBs map[string]interface{}

	// Use RAG configuration from instance if available
	rags := buildLCoreRAGConfigs(instance, instance.Status.ActiveOCPRAGVersion)
	if len(rags) > 0 {
		vectorDBs = map[string]interface{}{
			"default_embedding_model": map[string]interface{}{
				"provider_id": "sentence-transformers",
				// "model_id":    "all-mpnet-base-v2",
				"model_id": "/rag/rag-0/embeddings_model",
			},

			// "embedding_dimension": 768,
			"default_provider_id": "faiss",
			// "index_path":          rag.IndexPath,
		}

		vectorDBs["vector_store_id"] = getVectorStoreID()

	} else {
		// Default fallback if no RAG configured
		vectorDBs = map[string]interface{}{
			"vector_db_id":        "my_knowledge_base",
			"embedding_model":     "sentence-transformers/all-mpnet-base-v2",
			"embedding_dimension": 768,
			"provider_id":         "faiss",
		}
	}

	return vectorDBs
}

func buildLlamaStackModels(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) []interface{} {
	models := []interface{}{
		// Always include sentence-transformers embedding model for RAG
		map[string]interface{}{
			"model_id":          "sentence-transformers/all-mpnet-base-v2",
			"model_type":        "embedding",
			"provider_id":       "sentence-transformers",
			"provider_model_id": "/rag/rag-0/embeddings_model",
			"metadata": map[string]interface{}{
				"embedding_dimension": 768,
			},
		},
	}

	// Add LLM models from the instance spec
	{
		provider := buildProvider(instance)
		for _, model := range provider.Models {
			modelConfig := map[string]interface{}{
				"model_id":          model.Name,
				"model_type":        "llm",
				"provider_id":       provider.Name,
				"provider_model_id": model.Name,
			}

			// Add model-specific metadata if available
			metadata := map[string]interface{}{}
			if model.MaxTokensForResponse > 0 {
				metadata["max_tokens"] = model.MaxTokensForResponse
			}
			if len(metadata) > 0 {
				modelConfig["metadata"] = metadata
			}

			models = append(models, modelConfig)
		}
	}

	return models
}

func buildLlamaStackToolGroups(_ *common_helper.Helper, _ *apiv1beta1.OpenStackLightspeed) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"toolgroup_id": "builtin::rag",
			"provider_id":  "rag-runtime",
		},
	}
}

func buildLlamaStackRegisteredResources(_ *common_helper.Helper, instance *apiv1beta1.OpenStackLightspeed) map[string]interface{} {
	return map[string]interface{}{
		"models": []interface{}{
			map[string]interface{}{
				"model_id":   "sentence-transformers/all-mpnet-base-v2",
				"model_type": "embedding",
				// "provider_model_id": "all-mpnet-base-v2",
				"metadata": map[string]interface{}{
					"embedding_dimension": 768,
				},

				"provider_id": "sentence-transformers",
				// "provider_model_id": "all-mpnet-base-v2",
				"provider_model_id": "/rag/rag-0/embeddings_model",
			},

			map[string]interface{}{
				"model_id":          instance.Spec.ModelName,
				"model_type":        "llm",
				"provider_id":       "openstack-lightspeed-provider",
				"provider_model_id": instance.Spec.ModelName,
				"metadata": map[string]interface{}{
					"max_tokens": 2048,
				},
			},
		},
		"vector_stores": []interface{}{
			map[string]interface{}{
				"vector_store_id": getVectorStoreID(),
				// "model_id":        "sentence-transformers/all-mpnet-base-v2",
				"provider_id": "faiss",
				// "embedding_model":     "all-mpnet-base-v2",
				// "embedding_model":     "sentence-transformers/all-mpnet-base-v2",
				"embedding_model":     "sentence-transformers//rag/rag-0/embeddings_model",
				"embedding_dimension": 768,
				// "provider_model_id":   "/rag/rag-0/embeddings_model",
			},
		},
	}
}

// buildLlamaStackYAML assembles the complete Llama Stack configuration and converts to YAML
func buildLlamaStackYAML(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) (string, error) {
	// Build the complete config as a map
	config := buildLlamaStackCoreConfig(h, instance)

	// Build inference providers with error handling
	inferenceProviders, err := buildLlamaStackInferenceProviders(h, ctx, instance)
	if err != nil {
		return "", fmt.Errorf("failed to build inference providers: %w", err)
	}

	// Build providers map - only include providers for enabled APIs
	config["providers"] = map[string]interface{}{
		"files":        buildLlamaStackFileProviders(h, instance),
		"agents":       buildLlamaStackAgentProviders(h, instance),
		"inference":    inferenceProviders,
		"safety":       buildLlamaStackSafety(h, instance),
		"tool_runtime": buildLlamaStackToolRuntime(h, instance),
		"vector_io":    buildLlamaStackVectorDB(h, instance),
	}
	// Add top-level fields
	config["scoring_fns"] = []interface{}{}
	config["server"] = buildLlamaStackServerConfig(h, instance)
	config["storage"] = buildLlamaStackStorage(h, instance)
	config["vector_stores"] = buildLlamaStackVectorStores(h, instance)
	config["registered_resources"] = buildLlamaStackRegisteredResources(h, instance)
	config["models"] = buildLlamaStackModels(h, instance)
	config["tool_groups"] = buildLlamaStackToolGroups(h, instance)
	config["telemetry"] = map[string]interface{}{
		"enabled": false,
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Llama Stack config to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// sanitizeID creates a valid ID from an image name. It extracts just the image name without
// registry/tag (e.g., "quay.io/my-org/my-rag:latest" -> "my-rag")
func sanitizeID(image string) string {
	parts := strings.Split(image, "/")
	name := parts[len(parts)-1]
	name = strings.Split(name, ":")[0]

	// Replace invalid characters with underscores
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)

	return name
}
