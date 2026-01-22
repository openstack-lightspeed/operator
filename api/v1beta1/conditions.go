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
	// operational and it can be used by OpenStack Lihgtspeed operator.
	OpenShiftLightspeedOperatorReadyCondition condition.Type = "OpenShiftLightspeedOperatorReady"

	// OCPVersionCondition Status=True condition which indicates OCP version detection and resolution
	OCPVersionCondition condition.Type = "OCPVersionResolved"
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

	// OpenShiftLigthspeedOperatorReady
	OpenShiftLightspeedOperatorReady = "OpenShift Lightspeed operator is ready."

	// OCPVersionDetected message when OCP version is detected and resolved successfully
	OCPVersionDetected = "OCP version detected and resolved successfully"

	// OCPVersionFallback message when using 'latest' OCP documentation as fallback
	OCPVersionFallback = "Using 'latest' OCP documentation as fallback for unsupported cluster version"

	// OCPVersionOverrideInvalid message when OCP version override is invalid
	OCPVersionOverrideInvalid = "OCP version override is invalid"

	// OCPVersionDetectionFailed message when OCP version detection fails
	OCPVersionDetectionFailed = "Failed to detect OCP cluster version"
)
