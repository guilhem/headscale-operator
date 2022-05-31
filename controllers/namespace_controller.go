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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	headscalev1alpha1 "github.com/guilhem/headscale-operator/api/v1alpha1"
	"github.com/guilhem/headscale-operator/pkg/utils"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=headscale.barpilot.io,resources=namespaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := new(headscalev1alpha1.Namespace)
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Namespace")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	server := &headscalev1alpha1.Server{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Server,
			Namespace: instance.Namespace,
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(server), server); err != nil {
		log.Error(err, "unable to fetch Server")

		return ctrl.Result{}, err
	}

	if server.Status.GrpcAddress == "" {
		err := errors.New("Server not ready")
		log.Error(err, "GrpcAddress empty")

		return ctrl.Result{}, err
	}

	clientCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	client, err := utils.NewHeadscaleServiceClient(clientCtx, server.Status.GrpcAddress)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Headscale Client created", "GrpcAddress", server.Status.GrpcAddress)

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

			if _, err := client.DeleteNamespace(ctx, &v1.DeleteNamespaceRequest{Name: instance.Name}); utils.IgnoreNotFound(err) != nil {
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

	logNamespace := log.WithValues("namespace", instance.Name)
	if _, err := client.GetNamespace(ctx, &v1.GetNamespaceRequest{Name: instance.Name}); err != nil {

		if utils.IgnoreNotFound(err) != nil {
			logNamespace.Error(err, "Can't get Namespace")
			return ctrl.Result{}, err
		}

		if _, err := client.CreateNamespace(ctx, &v1.CreateNamespaceRequest{Name: instance.Name}); err != nil {
			logNamespace.Error(err, "can't create Namespace")
			return ctrl.Result{}, err
		}
		logNamespace.Info("Namespace Created")
	} else {
		logNamespace.Info("Namespace Already Exists")
	}

	instance.Status.Created = true

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// reconcile every 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&headscalev1alpha1.Namespace{}).
		Complete(r)
}
