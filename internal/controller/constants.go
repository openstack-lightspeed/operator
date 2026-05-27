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
	PostgresDataPVCName                          = "openstack-lightspeed-data"
	PostgresDataPVCDefaultSize                   = "1Gi"
	PostgresVarRunVolumeName                     = "lightspeed-postgres-var-run"
	PostgresVarRunVolumeMountPath                = "/var/run/postgresql"
	TmpVolumeName                                = "tmp-writable-volume"
	TmpVolumeMountPath                           = "/tmp"

	// LCore specific
	LlamaStackContainerPort  = int32(8321)
	LlamaStackConfigCmName   = "llama-stack-config"
	LCoreConfigCmName        = "lightspeed-stack-config"
	LCoreDeploymentName      = "lightspeed-stack-deployment"
	LCoreConfigMountPath     = "/app-root/lightspeed-stack.yaml"
	LCoreUserDataMountPath   = "/tmp/data"
	ForceReloadAnnotationKey = "ols.openshift.io/force-reload"

	// Data Exporter
	ExporterConfigVolumeName       = "exporter-config"
	ExporterConfigMountPath        = "/etc/config"
	ExporterConfigFilename         = "config.yaml"
	ExporterConfigCmName           = "lightspeed-exporter-config"
	DataverseExporterContainerName = "lightspeed-to-dataverse-exporter"
	UserDataVolumeName             = "ols-user-data"
	RHOSOLightspeedOwnerIDLabel    = "openstack.org/lightspeed-owner-id"
	ServiceIDRHOSO                 = "rhos-lightspeed"

	// Console Plugin
	ConsoleUIConfigMapName         = "lightspeed-console-plugin"
	ConsoleUIServiceCertSecretName = "lightspeed-console-plugin-cert"
	ConsoleUIServiceName           = "lightspeed-console-plugin"
	ConsoleUIDeploymentName        = "lightspeed-console-plugin"
	ConsoleUIHTTPSPort             = int32(9443)
	ConsoleUIPluginName            = "lightspeed-console-plugin"
	ConsoleUIServiceAccountName    = "lightspeed-console-plugin"
	ConsoleCRName                  = "cluster"
	ConsoleProxyAlias              = "ols"
	ConsoleUINetworkPolicyName     = "lightspeed-console-plugin"

	// Azure
	AzureOpenAIType = "azure_openai"

	// EnvVarSuffixAPIKey is the environment variable suffix for API key credentials
	EnvVarSuffixAPIKey = "_API_KEY"

	// VectorDBVolumeName is the name of the volume used by init containers to
	// store discovered values from vector DB images.
	VectorDBVolumeName = "vector-db-discovered-values"

	// VectorDBVolumeMountPath specifies the mount path for the volume that stores
	// discovered values from vector database images.
	VectorDBVolumeMountPath = "/vector-db-discovered-values"

	// VectorDBVolumeOGXConfigPath specifies the path within the `VectorDBVolumeName` volume
	// where the final OGX configuration file (ogx_config.yaml) is stored. This file is
	// generated by the init container responsible for assembling the final OGX config.
	VectorDBVolumeOGXConfigPath = VectorDBVolumeMountPath + "/ogx_config.yaml"

	// VectorDBVolumeLightspeedStackConfigPath specifies the path within the
	// `VectorDBVolumeName` volume where the final Lightspeed Stack configuration
	// file (lightspeed-stack.yaml) is stored. This file is generated by the
	// init container responsible for assembling the final Lightspeed Stack config.
	VectorDBVolumeLightspeedStackConfigPath = VectorDBVolumeMountPath + "/lightspeed-stack.yaml"

	// OGXConfigInitContainerMountPath specifies the path where the operator-generated
	// OGX config file is mounted in the init container responsible for assembling
	// the final OGX configuration, which includes information about RAG.
	OGXConfigInitContainerMountPath = "/ogx_config.yaml"

	// LightspeedStackInitContainerMountPath specifies the path where the
	// operator-generated Lightspeed Stack config file is mounted in the init
	// container responsible for assembling the final Lightspeed Stack configuration,
	// which includes information about RAG.
	LightspeedStackInitContainerMountPath = "/lightspeed-stack.yaml"

	// OGXConfigVolumeName specifies the name of the volume holding config file for OGX
	// (generated by the operator and passed to init containers)
	OGXConfigVolumeName = "ogx-config"

	// LightspeedStackConfig specifies the name of the volume holding config file for
	// Lightspeed Stack (generated by the operator and passed to init containers)
	LightspeedStackConfig = "lightspeed-stack-config"

	// OGXConfigCMKey is the key in the ConfigMap under which the OGX configuration
	// is stored.
	OGXConfigCMKey = "ogx_config.yaml"

	// LightspeedStackConfigCMKey is the key in the ConfigMap under which the Lightspeed Stack
	// configuration is stored.
	LightspeedStackConfigCMKey = "lightspeed-stack.yaml"

	// VectorDBScriptsConfigMapName is the name of the ConfigMap that contains the
	// initialization scripts used by init containers to collect and build vector database data
	VectorDBScriptsConfigMapName = "vector-db-scripts"

	// VectorDBScriptsVolumeName is the name of the volume that mounts the ConfigMap containing
	// vector database initialization scripts for use by init containers
	VectorDBScriptsVolumeName = "vector-db-scripts"

	// VectorDBScriptsMountPath specifies the path where vector database init scripts
	// should be mounted within the init containers.
	VectorDBScriptsMountPath = "/scripts"

	// VectorDBCollectScriptKey is the ConfigMap key under which the vector_database_collect.sh
	// script is stored in the ConfigMap containing vector database init scripts.
	VectorDBCollectScriptKey = "vector_database_collect.sh"

	// VectorDBBuildScriptKey is the ConfigMap key under which the vector_database_build.py
	// script is stored in the ConfigMap containing vector database init scripts.
	VectorDBBuildScriptKey = "vector_database_build.py"

	// Resource Version Annotation
	// These constants define annotation keys used to track the resource versions of specific ConfigMaps.
	// By recording the resource version of a ConfigMap in a Deployment, StatefulSet, or similar resource,
	// changes to the referenced ConfigMaps can be detected and trigger rollouts or reconciliation in the operator.
	PostgresConfigMapResourceVersionAnnotation   = "ols.openshift.io/postgres-configmap-version"
	VectorDBScriptsConfigMapVersionAnnotation    = "ols.openshift.io/vector-db-scripts-configmap-version"
	LlamaStackConfigMapResourceVersionAnnotation = "ols.openshift.io/llamastack-configmap-version"
	LCoreConfigMapResourceVersionAnnotation      = "ols.openshift.io/lcore-configmap-version"

	// Volume Permissions
	// These constants define file permissions for volumes mounted in containers.
	VolumeDefaultMode    = int32(420)
	VolumeRestrictedMode = int32(0600)
	VolumeExecutableMode = int32(0755)
)

// PostgreSQL Bootstrap Script - creates database, extensions, and schemas
//
//go:embed assets/postgres_bootstrap.sh
var PostgresBootStrapScriptContent string

// PostgreSQL Configuration - SSL and TLS settings
//
//go:embed assets/postgres.conf
var PostgresConfigMapContent string

// vectorDatabaseCollectScript embeds the contents of the vector_database_collect.sh script
// found in the assets directory. This script is used during the initialization of the
// vector database, run as an init container in the deployment. Read
// assets/vector_database_collect.sh for more comprehensive explanation.
//
//go:embed assets/vector_database_collect.sh
var vectorDatabaseCollectScript string

// vectorDatabaseBuildScript embeds the contents of the vector_database_build.py script
// found in the assets directory. This script is responsible for building or processing
// the vector database and is used by an init container during deployment initialization.
// Read assets/vector_database_build.py for more comprehensive explanation.
//
//go:embed assets/vector_database_build.py
var vectorDatabaseBuildScript string

//go:embed assets/console_nginx.conf.tmpl
var consoleNginxConfigTemplate string

// consoleLocalesRewriteAwk is the awk script that performs case-preserving
// OpenShift -> OpenStack replacement only in JSON values (after the first `": `).
//
//go:embed assets/console_locales_rewrite.awk
var consoleLocalesRewriteAwk string
