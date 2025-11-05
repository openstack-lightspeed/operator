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

// This file contains the logic for managing and ensuring the installation of
// the OpenShift Lightspeed (OLS) Operator in a cluster.
package controller

import (
	"context"
	"fmt"
	"os"
	"strings"

	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// OLSOperatorName - Name of the OpenShift Lightspeed operator.
	OLSOperatorName = "lightspeed-operator"
)

// EnsureOLSOperatorInstalled ensures that a compatible OLS Operator is present in the cluster.
// If the operator already exists, this checks that it matches the required version (otherwise it fails).
// If it is missing, this attempts to install the correct version.
func EnsureOLSOperatorInstalled(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	isUserInstalledOLSOperator, err := IsUserInstalledOLSOperatorMode(ctx, helper, instance)
	if err != nil {
		return false, err
	}

	if isUserInstalledOLSOperator {
		return CheckOLSOperatorVersion(ctx, helper, instance)
	}

	OLSOperatorInstalled, err := InstallInstanceOwnedOLSOperator(ctx, helper, instance)
	if err != nil {
		return false, err
	}

	return OLSOperatorInstalled, nil
}

// InstallInstanceOwnedOLSOperator - ensures that the OpenShift Lightspeed Operator (OLS Operator)
// is installed and owned by the specified OpenStackLightspeed instance. This function:
//  1. Determines the recommended OLS Operator version.
//  2. Creates or updates a Subscription, setting the instance as its owner.
//  3. Approves the related InstallPlan manually.
//  4. Sets ownership of the generated ClusterServiceVersion (CSV) to the instance.
//  5. Returns true if the OLS Operator is installed and owned by the instance, or an error otherwise.
func InstallInstanceOwnedOLSOperator(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	olsVersion, err := GetRecommendedOLSVersion()
	if err != nil {
		return false, err
	}

	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetOLSSubscriptionName(instance),
			Namespace: instance.Namespace,
		},
	}

	instanceOwnerReference := []metav1.OwnerReference{
		{
			APIVersion:         instance.APIVersion,
			Kind:               instance.Kind,
			Name:               instance.GetName(),
			UID:                instance.GetUID(),
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
	}
	opResult, err := controllerutil.CreateOrUpdate(ctx, helper.GetClient(), subscription, func() error {
		subscription.Spec = &operatorsv1alpha1.SubscriptionSpec{
			Channel:                "stable",
			InstallPlanApproval:    operatorsv1alpha1.ApprovalManual,
			CatalogSource:          instance.Spec.CatalogSourceName,
			CatalogSourceNamespace: instance.Spec.CatalogSourceNamespace,
			Package:                OLSOperatorName,
			StartingCSV:            fmt.Sprintf("%s.v%s", OLSOperatorName, olsVersion),
		}

		subscription.SetOwnerReferences(instanceOwnerReference)

		return nil
	})
	if err != nil {
		return false, err
	}

	// If the Subscription was just created, or if it doesn't yet contain an InstallPlanRef,
	// return (false, nil) -> wait. Attempting to approve the InstallPlan before it is properly
	// linked can cause OLM to create unnecessary additional InstallPlans.
	if opResult != controllerutil.OperationResultNone || subscription.Status.InstallPlanRef == nil {
		return false, nil
	}

	// Because we've set the subscription to require manual approval, we need to explicitly
	// approve the InstallPlan at this point. Manual approval is used to prevent OLM from
	// automatically upgrading the operator to a newer version than we've tested. This way,
	// we ensure that only the specific OLS Operator version we've tested is installed.
	installPlanApproved, err := ApproveOLSOperatorInstallPlan(ctx, helper, instance)
	if err != nil {
		return false, err
	} else if !installPlanApproved {
		return false, nil
	}

	// Ensure the CSV is owned by this instance. This helps determine during
	// deletion if the OLS Operator was installed by us or pre-existed before
	// the instance.
	OLSOperatorCSV, err := GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		return false, nil
	}

	OLSOperatorCSV.SetOwnerReferences(instanceOwnerReference)
	err = helper.GetClient().Update(ctx, OLSOperatorCSV)
	if err != nil && k8s_errors.IsConflict(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return InstanceOwnedOLSOperatorComplete(ctx, helper, instance)
}

// InstanceOwnedOLSOperatorComplete checks if the OLS Operator's CSV is owned
// by the given OpenStackLightspeed instance and is in the Succeeded phase.
func InstanceOwnedOLSOperatorComplete(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	OLSOperatorCSV, err := GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		return false, nil
	}

	// When the OLS Operator CSV is owned by us and it is in the Succeeded phase
	// we can be certain that the deployment of OLS Operator is over.
	return IsOwnedBy(OLSOperatorCSV, instance) && OLSOperatorCSV.Status.Phase == operatorsv1alpha1.CSVPhaseSucceeded, nil
}

// GetRecommendedOLSVersion returns the recommended version of the OpenShift
// Lightspeed (OLS) operator to deploy. This version is obtained from the environment
// variable "OPENSHIFT_LIGHTSPEED_OPERATOR_VERSION". If the variable
// is unset or empty, the function returns an error.
func GetRecommendedOLSVersion() (string, error) {
	version := os.Getenv("OPENSHIFT_LIGHTSPEED_OPERATOR_VERSION")
	if version != "" {
		return version, nil
	}

	return "", fmt.Errorf("environment variable OPENSHIFT_LIGHTSPEED_OPERATOR_VERSION is not set")
}

// GetOLSOperatorCSV - retrieves the ClusterServiceVersion (CSV) for the OpenShift Lightspeed operator
// from all namespaces in the OpenShift cluster. It returns the first CSV it finds whose name begins
// with the OLSOperatorName. If no such CSV is found, it returns (nil, nil). If there is an error
// while listing the CSV resources, that error is returned.
func GetOLSOperatorCSV(
	ctx context.Context,
	helper *common_helper.Helper,
) (*operatorsv1alpha1.ClusterServiceVersion, error) {
	var CSVs operatorsv1alpha1.ClusterServiceVersionList
	err := helper.GetClient().List(ctx, &CSVs, client.InNamespace(""))
	if err != nil && k8s_errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	for _, CSV := range CSVs.Items {
		if strings.HasPrefix(CSV.GetName(), OLSOperatorName) {
			return &CSV, nil
		}
	}

	return nil, nil
}

// IsUserInstalledOLSOperatorMode checks if an OpenShift Lightspeed Operator
// (OLS Operator) is installed in the cluster (by the user), but was NOT installed/owned by
// this specific OpenStackLightspeed instance. Returns true only if there is an OLS OperatorIsOwnedBy
// ClusterServiceVersion (CSV) found, and that CSV is NOT owned by the given instance.
func IsUserInstalledOLSOperatorMode(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	OLSOperatorCSV, err := GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		// Note: If no CSV is found we can be 100 % certain we are in the InstanceOwned
		// installation mode.
		return false, nil
	}

	subscription := &operatorsv1alpha1.Subscription{}
	err = helper.GetClient().Get(ctx, client.ObjectKey{
		Name:      GetOLSSubscriptionName(instance),
		Namespace: instance.Namespace,
	}, subscription)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return false, err
	}

	userInstalledMode := !IsOwnedBy(OLSOperatorCSV, instance) && !IsOwnedBy(subscription, instance)
	return userInstalledMode, nil
}

// UninstallInstanceOwnedOLSOperator ensures that the OLS Operator installed by
// a specific OpenStackLightspeed instance is uninstalled from the cluster. The function
// checks if the ClusterServiceVersion (CSV) for the OLS Operator exists and whether it
// is owned by the given OpenStackLightspeed instance. If so, it deletes the CSV.
// The function then checks whether the CSV has been successfully removed. It returns
// true if the operator CSV is no longer found (i.e., uninstalled), or an error if an
// unexpected problem occurs.
func UninstallInstanceOwnedOLSOperator(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	OLSOperatorCSV, err := GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		return true, nil
	}

	if !IsOwnedBy(OLSOperatorCSV, instance) {
		return true, nil
	}

	if err := helper.GetClient().Delete(ctx, OLSOperatorCSV); err != nil {
		return false, err
	}

	OLSOperatorCSV, err = GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		return true, nil
	}

	return false, nil
}

// ApproveOLSOperatorInstallPlan - checks for any pending, unapproved InstallPlans associated with the
// OpenShift Lightspeed Operator (OLS operator) within the namespace of the provided OpenStackLightspeed instance,
// and approves them. The function lists all InstallPlans in the instance's namespace, identifies those linked to
// the OLS operator and not yet approved, and updates their status to approved. Returns true if a pending
// InstallPlan is successfully approved. Returns false if there are no InstallPlans to be approved,
// and returns false with an error if any occurs during the listing or approval process.
func ApproveOLSOperatorInstallPlan(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	var installPlans operatorsv1alpha1.InstallPlanList
	err := helper.GetClient().List(ctx, &installPlans, client.InNamespace(instance.Namespace))
	if err != nil {
		return false, err
	}

	recommendedOLSVersion, err := GetRecommendedOLSVersion()
	if err != nil {
		return false, err
	}

	for _, installPlan := range installPlans.Items {
		// Continue if the InstallPlan does not have any CSVs associated with it.
		if len(installPlan.Spec.ClusterServiceVersionNames) == 0 {
			continue
		}

		isOLSOperatorCSV := strings.HasPrefix(installPlan.Spec.ClusterServiceVersionNames[0], OLSOperatorName)
		if !isOLSOperatorCSV {
			continue
		}

		isCorrectVersion := strings.HasSuffix(installPlan.Spec.ClusterServiceVersionNames[0], recommendedOLSVersion)
		if !isCorrectVersion {
			continue
		}

		installPlan.Spec.Approved = true
		err = helper.GetClient().Update(ctx, &installPlan)
		if err != nil && k8s_errors.IsConflict(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

// CheckOLSOperatorVersion returns true if the installed OLS Operator's version matches
// the recommended version for the currently installed OpenStack Lightspeed operator.
func CheckOLSOperatorVersion(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (bool, error) {
	OLSOperatorCSV, err := GetOLSOperatorCSV(ctx, helper)
	if err != nil {
		return false, err
	} else if OLSOperatorCSV == nil {
		return false, nil
	}

	recommendedVersion, err := GetRecommendedOLSVersion()
	if err != nil {
		return false, err
	}

	installedVersion := OLSOperatorCSV.Spec.Version.String()
	if installedVersion != recommendedVersion {
		errMsg := ("detected an unsupported version of OpenShift Lightspeed in the cluster." +
			"The recommended version is: %s. Please ensure that the installed version " +
			"matches the recommended version or uninstall it (OpenStack Lightspeed " +
			"will install the OpenShift Lightspeed operator automatically)")
		return false, fmt.Errorf(errMsg, recommendedVersion)
	}

	return true, nil
}

// GetOLSSubscriptionName generates a unique subscription name for the OpenStack Lightspeed Operator
// by appending the first 5 characters of the instance's UID. This reduces the likelihood of
// naming collisions with existing subscriptions that may have been created manually by the user.
func GetOLSSubscriptionName(instance *apiv1beta1.OpenStackLightspeed) string {
	return fmt.Sprintf("%s-%s", OLSOperatorName, string(instance.GetUID())[:5])
}
