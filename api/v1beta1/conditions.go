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
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// OpenStackLightspeed Condition Types used by API objects.
const (
	// OpenStackLightspeedReadyCondition Status=True condition which indicates if OpenStackLightspeedReadyCondition
	// is configured and operational
	OpenStackLightspeedReadyCondition condition.Type = "OpenStackLightspeedReady"

	// OpenShift Lightspeed Operator Status=True condition which indicates if OpenShift Lightspeed is installed and
	// operational and it can be used by OpenStack Lightspeed operator.
	OpenShiftLightspeedOperatorReadyCondition condition.Type = "OpenShiftLightspeedOperatorReady"

	// OCPRAGCondition Status=True condition which indicates the OCP RAG version resolution status
	OCPRAGCondition condition.Type = "OCPRAGReady"
)

// Common Messages used by API objects.
const (
	// OpenStackLightspeedReadyInitMessage
	OpenStackLightspeedReadyInitMessage = "OpenStack Lightspeed not started"

	// OpenStackLightspeedReadyMessage
	OpenStackLightspeedReadyMessage = "OpenStack Lightspeed created"

	// OpenStackLightspeedWaitingVectorDBMessage
	OpenStackLightspeedWaitingVectorDBMessage = "Waiting for OpenStackLightspeed vector DB pod to become ready"

	// OpenShiftLightspeedOperatorWaiting
	OpenShiftLightspeedOperatorWaiting = "Waiting for the OpenShift Lightspeed operator to deploy."

	// OpenShiftLightspeedOperatorReady
	OpenShiftLightspeedOperatorReady = "OpenShift Lightspeed operator is ready."

	// OCPRAGDisabledMessage
	OCPRAGDisabledMessage = "OCP RAG is disabled"

	// OCPRAGVersionResolvedMessage
	OCPRAGVersionResolvedMessage = "OCP RAG version resolved: %s"

	// OCPRAGVersionFallbackMessage
	OCPRAGVersionFallbackMessage = "Cluster version %s is not explicitly supported. Using 'latest' OCP documentation. Supported versions: %v"

	// OCPRAGDetectionFailedMessage
	OCPRAGDetectionFailedMessage = "Failed to detect OCP cluster version"

	// OCPRAGOverrideInvalidMessage
	OCPRAGOverrideInvalidMessage = "Invalid OCP RAG version override"
)
