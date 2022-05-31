/*
Copyright 2022 Guilhem Lettron.

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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	headscalev1alpha1 "github.com/guilhem/headscale-operator/api/v1alpha1"
	"github.com/guilhem/headscale-operator/pkg/utils"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
)

// PreAuthKeyReconciler reconciles a PreAuthKey object
type PreAuthKeyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=preauthkeys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=preauthkeys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=preauthkeys/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *PreAuthKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Begin")

	instance := new(headscalev1alpha1.PreAuthKey)
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch PreAuthKey")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	namespace := &headscalev1alpha1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Namespace,
			Namespace: instance.Namespace,
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		log.Error(err, "unable to fetch Namespace")

		return ctrl.Result{}, err
	}

	if !namespace.Status.Created {
		return ctrl.Result{}, errors.New("Namespace not created")
	}

	server := &headscalev1alpha1.Server{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace.Spec.Server,
			Namespace: instance.Namespace,
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(server), server); err != nil {
		log.Error(err, "unable to fetch Server")

		return ctrl.Result{}, err
	}

	if server.Status.GrpcAddress == "" {
		log.Info("Server not ready")

		return ctrl.Result{}, errors.New("Server not ready")
	}

	clientCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	client, err := utils.NewHeadscaleServiceClient(clientCtx, server.Status.GrpcAddress)
	if err != nil {
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, Finalizer) {
			controllerutil.AddFinalizer(instance, Finalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(instance, Finalizer) {

			if _, err := client.ExpirePreAuthKey(ctx, &v1.ExpirePreAuthKeyRequest{Key: instance.Name, Namespace: namespace.Name}); utils.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, Finalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	list, err := client.ListPreAuthKeys(ctx, &v1.ListPreAuthKeysRequest{Namespace: namespace.Name})
	if err != nil {
		return ctrl.Result{}, err
	}

	preAuthKey := new(v1.PreAuthKey)

	if i := slices.IndexFunc(list.PreAuthKeys, func(pak *v1.PreAuthKey) bool { return pak.Namespace == namespace.Name }); i >= 0 {
		// PreAuthKey already exist
		preAuthKey = list.PreAuthKeys[i]

	} else {
		createPreAuthKeyRequest := &v1.CreatePreAuthKeyRequest{
			Namespace: namespace.Name,
			Reusable:  instance.Spec.Reusable,
			Ephemeral: instance.Spec.Ephemeral,
		}

		if d := instance.Spec.Duration; d != "" {
			duration, err := time.ParseDuration(d)
			if err != nil {
				return ctrl.Result{}, err
			}
			createPreAuthKeyRequest.Expiration = timestamppb.New(time.Now().Add(duration))
		}

		createApiKeyResponse, err := client.CreatePreAuthKey(ctx, createPreAuthKeyRequest)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("PreAuthKey %s created", preAuthKey.Key))

		preAuthKey = createApiKeyResponse.PreAuthKey
	}

	instance.Status.Used = preAuthKey.GetUsed()
	instance.Status.ID = preAuthKey.GetId()
	instance.Status.CreatedAt = preAuthKey.GetCreatedAt().AsTime().String()
	instance.Status.Expiration = preAuthKey.GetExpiration().AsTime().String()
	instance.Status.Key = preAuthKey.GetKey()

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// check every 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PreAuthKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("preauthkey-controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&headscalev1alpha1.PreAuthKey{}).
		Complete(r)
}
