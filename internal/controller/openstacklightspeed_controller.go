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
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
)

// OpenStackLightspeedReconciler reconciles a OpenStackLightspeed object
type OpenStackLightspeedReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackLightspeedReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackLightspeed")
}

// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds/finalizers,verbs=update
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;watch
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,namespace=openshift-lightspeed,verbs=update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,namespace=openshift-lightspeed,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=installplans,namespace=openshift-lightspeed,verbs=get;list;watch;update;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,namespace=openshift-lightspeed,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=pods,namespace=openshift-lightspeed,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,namespace=openshift-lightspeed,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackLightspeed object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *OpenStackLightspeedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("OpenStackLightspeed Reconciling")

	instance := &apiv1beta1.OpenStackLightspeed{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("OpenStackLightspeed CR not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	helper, err := common_helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)

	// Save a copy of the conditions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// Don't update the status, if reconciler Panics
		if r := recover(); r != nil {
			Log.Info(fmt.Sprintf("panic during reconcile %v\n", r))
			panic(r)
		}

		condition.RestoreLastTransitionTimes(&instance.Status.Conditions, savedConditions)
		// update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			// something is not ready so reset the Ready condition
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			// and recalculate it based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			return
		}

	}()

	cl := condition.CreateList(
		condition.UnknownCondition(
			apiv1beta1.OpenStackLightspeedReadyCondition,
			condition.InitReason,
			apiv1beta1.OpenStackLightspeedReadyInitMessage,
		),
	)

	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, helper, instance)
	}

	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	if instance.Spec.RAGImage == "" {
		instance.Spec.RAGImage = apiv1beta1.OpenStackLightspeedDefaultValues.RAGImageURL
	}

	if instance.Spec.MaxTokensForResponse == 0 {
		instance.Spec.MaxTokensForResponse = apiv1beta1.OpenStackLightspeedDefaultValues.MaxTokensForResponse
	}

	// Ensure a compatible version of the OpenShift Lightspeed Operator is running in the cluster.
	// This checks if the correct OLS Operator version is present and installs it if necessary.
	isOLSOperatorInstalled, err := EnsureOLSOperatorInstalled(ctx, helper, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			apiv1beta1.OpenShiftLightspeedOperatorReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error(),
		))

		return ctrl.Result{}, nil
	} else if !isOLSOperatorInstalled {
		instance.Status.Conditions.Set(condition.FalseCondition(
			apiv1beta1.OpenShiftLightspeedOperatorReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			apiv1beta1.OpenShiftLightspeedOperatorWaiting,
		))

		// In this branch we know that the
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	}

	// Mark the OpenShift Lightspeed Operator as ready in the status conditions.
	instance.Status.Conditions.MarkTrue(
		apiv1beta1.OpenShiftLightspeedOperatorReadyCondition,
		apiv1beta1.OpenShiftLightspeedOperatorReady,
	)

	// NOTE: We cannot consume the OLSConfig definition directly from the OLS operator's code due to
	// a conflict in Go versions. When this comment was written, the min. required Go version for
	// openstack-operator was 1.21 whereas OLS operator required at least Go version 1.23. Once the
	// Go versions catch up with each other we should consider consuming OLSConfig directly from OLS
	// operator and updating this code and any subsequent code that consumes this structure.
	olsConfig := uns.Unstructured{}
	olsConfigGVK := schema.GroupVersionKind{
		Group:   "ols.openshift.io",
		Version: "v1alpha1",
		Kind:    "OLSConfig",
	}

	olsConfig.SetGroupVersionKind(olsConfigGVK)
	olsConfig.SetName(OLSConfigName)

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, &olsConfig, func() error {
		// Check if the OpenStackLightspeed instance that is being processed owns the OLSConfig. If
		// it is owned by other OpenStackLightspeed instance stop the reconciliation.
		olsConfigLabels := olsConfig.GetLabels()
		ownerLabel := ""
		if val, ok := olsConfigLabels[OpenStackLightspeedOwnerIDLabel]; ok {
			ownerLabel = val
		}

		if ownerLabel != "" && ownerLabel != string(instance.GetObjectMeta().GetUID()) {
			return fmt.Errorf("OLSConfig is managed by different OpenStackLightspeed instance")
		}

		err = PatchOLSConfig(helper, instance, &olsConfig)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			apiv1beta1.OpenStackLightspeedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	OLSConfigReady, err := IsOLSConfigReady(ctx, helper)
	if err != nil {
		return ctrl.Result{}, err
	}

	if OLSConfigReady {
		instance.Status.Conditions.MarkTrue(
			apiv1beta1.OpenStackLightspeedReadyCondition,
			apiv1beta1.OpenStackLightspeedReadyMessage,
		)
		Log.Info("OLSConfig is ready!")
	} else {
		Log.Info("OLSConfig is not ready yet. Waiting...")
		return ctrl.Result{RequeueAfter: time.Second * time.Duration(5)}, nil
	}

	Log.Info("OpenStackLightspeed Reconciled successfully")
	return ctrl.Result{}, nil
}

// reconcileDelete reconciles the deletion of OpenStackLightspeed instance
func (r *OpenStackLightspeedReconciler) reconcileDelete(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("OpenStackLightspeed Reconciling Delete")

	isRemoved, err := RemoveOLSConfig(ctx, helper, instance)
	if err != nil {
		return ctrl.Result{}, err
	} else if !isRemoved {
		Log.Info("OLSConfig removal in progress ...")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	isUninstalled, err := UninstallInstanceOwnedOLSOperator(ctx, helper, instance)
	if err != nil {
		return ctrl.Result{}, err
	} else if !isUninstalled {
		Log.Info("OLS Operator uninstallation in progress ...")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())

	Log.Info("OpenStackLightspeed Reconciling Delete completed")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackLightspeedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta1.OpenStackLightspeed{}).
		Owns(&operatorsv1alpha1.ClusterServiceVersion{}).
		Owns(&operatorsv1alpha1.Subscription{}).
		Watches(
			&operatorsv1alpha1.InstallPlan{},
			handler.EnqueueRequestsFromMapFunc(r.NotifyAllOpenStackLightspeeds),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

// NotifyAllOpenStackLightspeeds returns a list of reconcile requests for all OpenStackLightspeed objects
// in the same namespace as the given InstallPlan. This is used to trigger reconciliation on all
// OpenStackLightspeed resources when an InstallPlan in their namespace changes.
func (r *OpenStackLightspeedReconciler) NotifyAllOpenStackLightspeeds(ctx context.Context, obj client.Object) []ctrl.Request {
	// Pre-allocate requests slice with the capacity equal to the number of OpenStackLightspeed objects
	var lightspeedList apiv1beta1.OpenStackLightspeedList
	if err := r.List(ctx, &lightspeedList, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]ctrl.Request, 0, len(lightspeedList.Items))

	for _, item := range lightspeedList.Items {
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
		})
	}

	return requests
}
