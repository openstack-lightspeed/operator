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
	"time"
)

const (
	// Volume Permissions
	VolumeDefaultMode    = int32(420)
	VolumeRestrictedMode = int32(0600)

	// Operator Settings
	ResourceCreationTimeout = 60 * time.Second

	// Application Server
	OpenStackLightspeedAppServerServiceAccountName = "lightspeed-app-server"
	OpenStackLightspeedAppServerSARRoleName        = OpenStackLightspeedAppServerServiceAccountName + "-sar-role"
	OpenStackLightspeedAppServerSARRoleBindingName = OpenStackLightspeedAppServerSARRoleName + "-binding"
	OpenStackLightspeedAppServerContainerPort      = 8443
	OpenStackLightspeedAppServerServicePort        = 8443
	OpenStackLightspeedAppServerServiceName        = "lightspeed-app-server"
	OpenStackLightspeedAppServerNetworkPolicyName  = "lightspeed-app-server"
	OpenStackLightspeedCertsSecretName             = "lightspeed-tls"
	OpenStackLightspeedDefaultProvider             = "openstack-lightspeed-provider"
	OpenStackLightspeedVectorDBPath                = "/rag/vector_db/os_product_docs"

	ServingCertSecretAnnotationKey = "service.beta.openshift.io/serving-cert-secret-name"

	// Monitoring
	MetricsReaderServiceAccountTokenSecretName = "metrics-reader-token"
	MetricsReaderServiceAccountName            = "lightspeed-operator-metrics-reader"

	// Cert / CA
	OpenStackLightspeedAppCertsMountRoot = "/etc/certs"
	OpenStackLightspeedCAConfigMap       = "openshift-service-ca.crt"
	OpenShiftCAVolumeName                = "openshift-ca"
	AdditionalCAVolumeName               = "additional-ca"
	AdditionalCACertFile                 = "cert.crt"

	// Postgres
	PostgresCAVolume                             = "cm-olspostgresca"
	PostgresDeploymentName                       = "lightspeed-postgres-server"
	PostgresServiceName                          = "lightspeed-postgres-server"
	PostgresSecretName                           = "lightspeed-postgres-secret"
	PostgresCertsSecretName                      = "lightspeed-postgres-certs"
	PostgresBootstrapSecretName                  = "lightspeed-postgres-bootstrap"
	PostgresConfigMapName                        = "lightspeed-postgres-conf"
	PostgresNetworkPolicyName                    = "lightspeed-postgres-server"
	PostgresServicePort                          = int32(5432)
	PostgresDefaultUser                          = "postgres"
	PostgresDefaultDbName                        = "postgres"
	PostgresDefaultSSLMode                       = "require"
	PostgresSharedBuffers                        = "256MB"
	PostgresMaxConnections                       = 100
	OpenStackLightspeedComponentPasswordFileName = "password"
	PostgresExtensionScript                      = "create-extensions.sh"
	PostgresConfigKey                            = "postgresql.conf.sample"
	PostgresBootstrapVolumeMountPath             = "/usr/share/container-scripts/postgresql/start/create-extensions.sh"
	PostgresConfigVolumeMountPath                = "/usr/share/pgsql/postgresql.conf.sample"
	PostgresDataVolume                           = "postgres-data"
	PostgresDataVolumeMountPath                  = "/var/lib/pgsql"
	PostgresVarRunVolumeName                     = "lightspeed-postgres-var-run"
	PostgresVarRunVolumeMountPath                = "/var/run/postgresql"
	TmpVolumeName                                = "tmp-writable-volume"
	TmpVolumeMountPath                           = "/tmp"
	PostgresConfigMapResourceVersionAnnotation   = "ols.openshift.io/postgres-configmap-version"

	// LCore specific
	LlamaStackContainerPort                      = int32(8321)
	LlamaStackConfigCmName                       = "llama-stack-config"
	LCoreConfigCmName                            = "lightspeed-stack-config"
	LCoreDeploymentName                          = "lightspeed-stack-deployment"
	LlamaStackConfigMountPath                    = "/app-root/run.yaml"
	LCoreConfigMountPath                         = "/app-root/lightspeed-stack.yaml"
	LlamaStackConfigFilename                     = "run.yaml"
	LCoreConfigFilename                          = "lightspeed-stack.yaml"
	LCoreConfigMapResourceVersionAnnotation      = "ols.openshift.io/lcore-configmap-version"
	LlamaStackConfigMapResourceVersionAnnotation = "ols.openshift.io/llamastack-configmap-version"
	LCoreUserDataMountPath                       = "/tmp/data"
	ForceReloadAnnotationKey                     = "ols.openshift.io/force-reload"

	// Data Exporter
	ExporterConfigVolumeName    = "exporter-config"
	ExporterConfigMountPath     = "/etc/config"
	ExporterConfigFilename      = "config.yaml"
	RHOSOLightspeedOwnerIDLabel = "openstack.org/lightspeed-owner-id"
	ServiceIDRHOSO              = "rhos-lightspeed"

	// Azure
	AzureOpenAIType = "azure_openai"

	// EnvVarSuffixAPIKey is the environment variable suffix for API key credentials
	EnvVarSuffixAPIKey = "_API_KEY"
)

// PostgreSQL Bootstrap Script - creates database, extensions, and schemas
//
//go:embed assets/postgres_bootstrap.sh
var PostgresBootStrapScriptContent string

// PostgreSQL Configuration - SSL and TLS settings
//
//go:embed assets/postgres.conf
var PostgresConfigMapContent string
