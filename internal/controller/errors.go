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

import "errors"

var (
	// Lcore Errors
	ErrCreateAPIConfigmap           = errors.New("failed to create OpenStack Lightspeed configmap")
	ErrCreateAPIDeployment          = errors.New("failed to create OpenStack Lightspeed deployment")
	ErrCreateAPIService             = errors.New("failed to create OpenStack Lightspeed service")
	ErrCreateAPIServiceAccount      = errors.New("failed to create OpenStack Lightspeed service account")
	ErrCreateAppServerNetworkPolicy = errors.New("failed to create AppServer network policy")
	ErrCreateSARClusterRole         = errors.New("failed to create SAR cluster role")
	ErrCreateSARClusterRoleBinding  = errors.New("failed to create SAR cluster role binding")
	ErrDeleteSARClusterRole         = errors.New("failed to delete SAR cluster role")
	ErrDeleteSARClusterRoleBinding  = errors.New("failed to delete SAR cluster role binding")
	ErrGenerateAPIConfigmap         = errors.New("failed to generate OpenStack Lightspeed configmap")
	ErrGetAdditionalCACM            = errors.New("failed to get additional CA configmap")
	ErrGetProxyCACM                 = errors.New("failed to get proxy CA configmap")
	ErrGetTLSSecret                 = errors.New("failed to get TLS secret")
	ErrCreateLlamaStackConfigMap    = errors.New("failed to create Llama Stack configmap")
	ErrGenerateLlamaStackConfigMap  = errors.New("failed to generate Llama Stack configmap")

	// Postgres Errors
	ErrCreatePostgresDeployment      = errors.New("failed to create Postgres deployment")
	ErrCreatePostgresService         = errors.New("failed to create Postgres service")
	ErrGeneratePostgresSecret        = errors.New("failed to generate Postgres secret")
	ErrCreatePostgresSecret          = errors.New("failed to create Postgres secret")
	ErrGetPostgresSecret             = errors.New("failed to get Postgres secret")
	ErrCreatePostgresBootstrapSecret = errors.New("failed to create Postgres bootstrap secret")
	ErrCreatePostgresConfigMap       = errors.New("failed to create Postgres configmap")
	ErrGetPostgresConfigMap          = errors.New("failed to get Postgres configmap")
	ErrCreatePostgresNetworkPolicy   = errors.New("failed to create Postgres network policy")
)
