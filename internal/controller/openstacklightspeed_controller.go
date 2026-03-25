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

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	if err != nil {
		return ctrl.Result{}, err
	}

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

	// OCP Version Detection and Resolution - must be done early so status field is always set
	r.resolveOCPVersion(ctx, helper, instance)

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

	Log.Info("OpenStackLightspeed Reconciled successfully")
	return ctrl.Result{}, nil
}

// resolveOCPVersion detects and resolves the OCP version to use for RAG configuration.
// Returns the active OCP version to use (or empty string if OCP RAG is disabled).
func (r *OpenStackLightspeedReconciler) resolveOCPVersion(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) string {
	Log := helper.GetLogger()

	// If OCP RAG is disabled, mark condition as True with "disabled" message
	if !instance.Spec.EnableOCPRAG {
		instance.Status.Conditions.MarkTrue(
			apiv1beta1.OCPRAGCondition,
			apiv1beta1.OCPRAGDisabledMessage,
		)
		instance.Status.ActiveOCPRAGVersion = ""
		return ""
	}

	// Step 1: Detect cluster version
	detectedVersion, err := DetectOCPVersion(ctx, helper)

	if err != nil {
		Log.Info("Failed to detect OCP version, disabling OCP RAG", "error", err)
		cond := condition.FalseCondition(
			apiv1beta1.OCPRAGCondition,
			condition.ErrorReason,
			condition.SeverityError,
			apiv1beta1.OCPRAGDetectionFailedMessage,
		)
		cond.Message = fmt.Sprintf("%s: %s", apiv1beta1.OCPRAGDetectionFailedMessage, err.Error())
		instance.Status.Conditions.Set(cond)
		instance.Status.ActiveOCPRAGVersion = ""
		return ""
	}

	Log.Info("Detected OCP cluster version", "version", detectedVersion)

	// Step 2: Resolve which version to use (with override and fallback)
	activeVersion, isFallback, err := ResolveOCPVersion(
		detectedVersion,
		instance.Spec.OCPRAGVersionOverride,
		instance.Spec.EnableOCPRAG,
	)

	if err != nil {
		// Invalid override
		Log.Error(err, "Invalid OCP version configuration")
		cond := condition.FalseCondition(
			apiv1beta1.OCPRAGCondition,
			condition.ErrorReason,
			condition.SeverityError,
			apiv1beta1.OCPRAGOverrideInvalidMessage,
		)
		cond.Message = fmt.Sprintf("%s: %s", apiv1beta1.OCPRAGOverrideInvalidMessage, err.Error())
		instance.Status.Conditions.Set(cond)
		instance.Status.ActiveOCPRAGVersion = ""
		return ""
	}

	// Step 3: Update status and conditions based on resolution
	instance.Status.ActiveOCPRAGVersion = activeVersion

	if isFallback {
		Log.Info("Using 'latest' OCP documentation as fallback",
			"detectedVersion", detectedVersion,
			"supportedVersions", SupportedOCPVersions)

		cond := condition.TrueCondition(
			apiv1beta1.OCPRAGCondition,
			"Fallback",
		)
		cond.Message = fmt.Sprintf(apiv1beta1.OCPRAGVersionFallbackMessage,
			detectedVersion, SupportedOCPVersions)
		instance.Status.Conditions.Set(cond)
	} else {
		Log.Info("Using OCP RAG documentation", "version", activeVersion)
		cond := condition.TrueCondition(
			apiv1beta1.OCPRAGCondition,
			"Resolved",
		)
		cond.Message = fmt.Sprintf(apiv1beta1.OCPRAGVersionResolvedMessage, activeVersion)
		instance.Status.Conditions.Set(cond)
	}

	return activeVersion
}

// reconcileDelete reconciles the deletion of OpenStackLightspeed instance
func (r *OpenStackLightspeedReconciler) reconcileDelete(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *apiv1beta1.OpenStackLightspeed,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("OpenStackLightspeed Reconciling Delete")

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())

	Log.Info("OpenStackLightspeed Reconciling Delete completed")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackLightspeedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create an unstructured ClusterVersion for watching
	// This triggers reconciliation when OCP is upgraded (e.g., 4.16 -> 4.18)
	clusterVersion := &uns.Unstructured{}
	clusterVersion.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "ClusterVersion",
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta1.OpenStackLightspeed{}).
		Owns(&operatorsv1alpha1.ClusterServiceVersion{}).
		Owns(&operatorsv1alpha1.Subscription{}).
		Watches(
			&operatorsv1alpha1.InstallPlan{},
			handler.EnqueueRequestsFromMapFunc(r.NotifyAllOpenStackLightspeeds),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			clusterVersion,
			handler.EnqueueRequestsFromMapFunc(r.NotifyAllOpenStackLightspeeds),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

// NotifyAllOpenStackLightspeeds returns a list of reconcile requests for all OpenStackLightspeed objects.
// For namespace-scoped resources (like InstallPlan), it lists in the same namespace as the triggering object.
// For cluster-scoped resources (like ClusterVersion), it lists in all namespaces the operator can access.
func (r *OpenStackLightspeedReconciler) NotifyAllOpenStackLightspeeds(ctx context.Context, obj client.Object) []ctrl.Request {
	var lightspeedList apiv1beta1.OpenStackLightspeedList
	var err error

	// For cluster-scoped resources (no namespace), list without namespace filter
	// The operator's cache is already restricted to the watch namespace, so this is safe
	if obj.GetNamespace() == "" {
		err = r.List(ctx, &lightspeedList)
	} else {
		err = r.List(ctx, &lightspeedList, client.InNamespace(obj.GetNamespace()))
	}

	if err != nil {
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
