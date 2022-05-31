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
	"encoding/json"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"strconv"

	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	headscalev1alpha1 "github.com/guilhem/headscale-operator/api/v1alpha1"
	"github.com/guilhem/headscale-operator/pkg/utils"
)

// ServerReconciler reconciles a Server object
type ServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

const Finalizer = "headscale.barpilot.io/finalizer"

const ConfigFileName = "config.json"

// +kubebuilder:rbac:groups=headscale.barpilot.io,resources=servers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=headscale.barpilot.io,resources=servers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=headscale.barpilot.io,resources=servers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Server object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := new(headscalev1alpha1.Server)
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Server")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
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

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, Finalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	config := instance.Spec.Config

	log.Info("Config before default", "config", config)

	// Default value
	if err := mergo.Merge(&config, defaultServerConfig); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Config after default", "config", config)

	// _, grpcPort, err := utils.SliptHostPort(instance.Spec.Config.GRPCAddr)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	labels := labels.Set{
		"app.kubernetes.io/name":       "headscale",
		"app.kubernetes.io/instance":   instance.Name,
		"app.kubernetes.io/managed-by": "headscale-operator",
		"app.kubernetes.io/component":  "server",
	}

	////////////////
	// Service
	////////////////

	const grpcInsecurePort = 8082

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-grpc", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := controllerutil.SetControllerReference(instance, service, r.Scheme); err != nil {
			return err
		}

		service.ObjectMeta.Labels = labels

		service.Spec.Type = corev1.ServiceTypeClusterIP
		ports := []corev1.ServicePort{
			{
				Name:       "server",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromString("server"),
			},
			{
				Name:       "grpc-insecure",
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(grpcInsecurePort),
				TargetPort: intstr.FromString("grpc-insecure"),
			},
		}

		if err := mergo.Merge(&service.Spec.Ports, ports, mergo.WithOverride); err != nil {
			return err
		}

		service.Spec.Selector = labels

		return nil
	}); err != nil {
		r.recorder.Event(instance, "Warning", "Failed", fmt.Sprintf("Fail to reconcile Service %s", service.Name))
		log.Error(err, "Service reconcile failed")
	} else {
		switch op {
		case controllerutil.OperationResultCreated:
			r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("Created Sevice %s", service.Name))
		case controllerutil.OperationResultUpdated:
			r.recorder.Event(instance, "Normal", "Updated", fmt.Sprintf("Updated Sevice %s", service.Name))
		}
	}

	if service.Spec.ClusterIP != "" {
		instance.Status.GrpcAddress = net.JoinHostPort(service.Spec.ClusterIP, strconv.Itoa(grpcInsecurePort))
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// ////////////////
	// // Service LB
	// ////////////////

	// serviceLB := &corev1.Service{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      fmt.Sprintf("%s-lb", instance.Name),
	// 		Namespace: instance.Namespace,
	// 	},
	// }

	// if op, err := controllerutil.CreateOrPatch(ctx, r.Client, serviceLB, func() error {
	// 	if err := controllerutil.SetControllerReference(instance, serviceLB, r.Scheme); err != nil {
	// 		return err
	// 	}

	// 	serviceLB.Spec.Type = corev1.ServiceTypeLoadBalancer
	// 	ports := []corev1.ServicePort{
	// 		{
	// 			Name:       "server",
	// 			Protocol:   corev1.ProtocolTCP,
	// 			Port:       443,
	// 			TargetPort: intstr.FromString("server"),
	// 		},
	// 	}

	// 	if err := mergo.Merge(&serviceLB.Spec.Ports, ports); err != nil {
	// 		return err
	// 	}

	// 	serviceLB.Spec.Selector = labels

	// 	return nil
	// }); err != nil {
	// 	log.Error(err, "Service LB reconcile failed")
	// } else {
	// 	switch op {
	// 	case controllerutil.OperationResultCreated:
	// 		r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("Created Sevice %s", service.Name))
	// 	case controllerutil.OperationResultUpdated:
	// 		r.recorder.Event(instance, "Normal", "Updated", fmt.Sprintf("Updated Sevice %s", service.Name))
	// 	}
	// }

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	// certificate := &certmanagerv1.Certificate{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      instance.Name,
	// 		Namespace: instance.Namespace,
	// 	},
	// }

	// if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, certificate, func() error {
	// 	if err := controllerutil.SetControllerReference(instance, certificate, r.Scheme); err != nil {
	// 		return err
	// 	}

	// 	certificate.Spec.SecretName = fmt.Sprintf("%s-tls", certificate.Name)
	// 	certificate.Spec.IssuerRef = certmanagerv1metav1.ObjectReference{
	// 		Name: instance.Spec.Issuer,
	// 	}
	// 	certificate.Spec.DNSNames = []string{"test.com"}

	// 	ips := service.Spec.ClusterIPs
	// 	ips = append(ips, serviceLB.Spec.ClusterIPs...)
	// 	ips = append(ips, serviceLB.Spec.ExternalIPs...)
	// 	certificate.Spec.IPAddresses = ips

	// 	return nil
	// }); err != nil {
	// 	log.Error(err, "certificate reconcile failed")
	// } else {
	// 	if op != controllerutil.OperationResultNone {
	// 		log.Info("certificate successfully reconciled", "operation", op)
	// 	}
	// }

	// const keypath = "/run/headscale/certs"

	// instance.Spec.Config.TLSCertPath = path.Join(keypath, "tls.crt")
	// instance.Spec.Config.TLSKeyPath = path.Join(keypath, "tls.key")

	const runSocketsPath = "/run/headscale/socket"
	const socketName = "headscale.sock"

	config.UnixSocket = path.Join(runSocketsPath, socketName)
	config.UnixSocketPermission = "0770"

	const dataPath = "/var/lib/headscale"
	const sqlitename = "db.sqlite"

	config.DBtype = "sqlite3"
	config.DBpath = path.Join(dataPath, sqlitename)
	config.PrivateKeyPath = filepath.Join(dataPath, "private.key")

	config.Addr = "0.0.0.0:8080"
	config.ServerURL = fmt.Sprintf("https://%s", instance.Spec.Host)

	if instance.Spec.Debug {
		config.LogLevel = "debug"
	}

	// config.GRPCAllowInsecure = pointer.Bool(true)

	///////////////////////
	// Configmap
	///////////////////////

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Data: make(map[string]string),
	}

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, configmap, func() error {
		if err := controllerutil.SetControllerReference(instance, configmap, r.Scheme); err != nil {
			return err
		}

		configmap.ObjectMeta.Labels = labels

		c, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		configmap.BinaryData = map[string][]byte{}

		configmap.Data[ConfigFileName] = string(c)

		return nil
	}); err != nil {
		r.recorder.Event(instance, "Warning", "Failed", fmt.Sprintf("Fail to reconcile Service %s", configmap.Name))
	} else {
		switch op {
		case controllerutil.OperationResultCreated:
			r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("Created Configmap %s", configmap.Name))
		case controllerutil.OperationResultUpdated:
			r.recorder.Event(instance, "Normal", "Updated", fmt.Sprintf("Updated Configmap %s", configmap.Name))
		}
	}

	////////////////
	// Deployment
	////////////////

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
			return err
		}

		deployment.ObjectMeta.Labels = labels

		version := instance.Spec.Version
		if version == "" {
			version = "latest"
		}

		_, listenPort, err := utils.SliptHostPort(config.Addr)
		if err != nil {
			return fmt.Errorf("can't parse config.Addr %w", err)
		}

		_, metricsPort, err := utils.SliptHostPort(config.MetricsAddr)
		if err != nil {
			return fmt.Errorf("can't parse config.MetricsAddr %w", err)
		}

		// _, grpcPort, err := utils.SliptHostPort(instance.Spec.Config.GRPCAddr)
		// if err != nil {
		// 	return err
		// }

		// immuable
		if deployment.ObjectMeta.CreationTimestamp.IsZero() {
			deployment.Spec.Selector = metav1.SetAsLabelSelector(labels)
		}

		deployment.Spec.Replicas = pointer.Int32(1)
		podTemplate := corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"config-version": configmap.GetResourceVersion(),
					// "certificate-version": certificate.GetResourceVersion(),
				},
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				// InitContainers: []corev1.Container{
				// 	{
				// 		Name: "init-litestream",
				// 		Image: "litestream/litestream:0.3.8",
				// 		Args: []string{"restore", "-if-db-not-exists", "-if-replica-exists", "-v", instance.Spec.Config.DBpath},
				// 		VolumeMounts: []corev1.VolumeMount{
				// 			{
				// 				Name: "data",
				// 				MountPath: sqlitedir,
				// 			},
				// 		},
				// 	},
				// },
				Containers: []corev1.Container{
					{
						Name:            "headscale",
						Image:           fmt.Sprintf("headscale/headscale:%s", version),
						ImagePullPolicy: corev1.PullAlways,
						Command:         []string{"headscale", "serve"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config",
								ReadOnly:  true,
								MountPath: "/etc/headscale/",
							},
							// {
							// 	Name:      "certificate",
							// 	ReadOnly:  true,
							// 	MountPath: keypath,
							// },
							{
								Name:      "data",
								MountPath: dataPath,
							},
							{
								Name:      "run",
								MountPath: runSocketsPath,
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "server",
								ContainerPort: int32(listenPort),
							},
							{
								Name:          "metrics",
								ContainerPort: int32(metricsPort),
							},
							// {
							// 	Name:          "grpc",
							// 	ContainerPort: int32(grpcPort),
							// },
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Port:   intstr.FromString("server"),
									Path:   "/health",
									Scheme: corev1.URISchemeHTTP,
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Port:   intstr.FromString("server"),
									Path:   "/health",
									Scheme: corev1.URISchemeHTTP,
								},
							},
						},
					},
					{
						Name:  "socat",
						Image: "alpine/socat:1.7.4.3-r0",
						Args: []string{
							"tcp-listen:8082,fork,reuseaddr",
							fmt.Sprintf("unix-connect:%s", path.Join(runSocketsPath, socketName)),
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "grpc-insecure",
								ContainerPort: int32(grpcInsecurePort),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "run",
								ReadOnly:  true,
								MountPath: runSocketsPath,
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromString("grpc-insecure"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromString("grpc-insecure"),
								},
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configmap.Name,
								},
							},
						},
					},
					// {
					// 	Name: "certificate",
					// 	VolumeSource: corev1.VolumeSource{
					// 		Secret: &corev1.SecretVolumeSource{
					// 			SecretName: certificate.Spec.SecretName,
					// 		},
					// 	},
					// },
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "run",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		}

		if err := mergo.Merge(&deployment.Spec.Template, podTemplate, mergo.WithOverride); err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Error(err, "Deployment reconcile failed")
	} else {
		if op != controllerutil.OperationResultNone {
			log.Info("Deployment successfully reconciled", "operation", op)
		}
		instance.Status.DeploymentName = deployment.Name
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// ingress := instance.Spec.Ingress
	// if ingress == nil {
	// 	ingress = &networkingv1.Ingress{}
	// }

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	// ingress.SetName(instance.Name)
	// ingress.SetNamespace(instance.Namespace)
	// ingress.ObjectMeta.Name = instance.Name
	// ingress.ObjectMeta.Namespace = instance.Namespace

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
		if err := controllerutil.SetControllerReference(instance, ingress, r.Scheme); err != nil {
			return err
		}

		if instance.Spec.Ingress != nil {
			if err := mergo.Merge(ingress, instance.Spec.Ingress); err != nil {
				return err
			}
		}

		instance.ObjectMeta.Labels = labels

		var prefixPathType = networkingv1.PathTypePrefix

		rules := []networkingv1.IngressRule{
			{
				Host: instance.Spec.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path: "/",
								// PathType: (*networkingv1.PathType)(pointer.String(string(networkingv1.PathTypePrefix))),
								PathType: &prefixPathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: service.GetName(),
										Port: networkingv1.ServiceBackendPort{
											Name: "server",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		if err := mergo.Merge(&ingress.Spec.Rules, rules, mergo.WithOverride); err != nil {
			return err
		}

		tls := []networkingv1.IngressTLS{
			{
				Hosts: []string{
					instance.Spec.Host,
				},
				SecretName: fmt.Sprintf("%s-certificats", instance.GetName()),
			},
		}

		if err := mergo.Merge(&ingress.Spec.TLS, tls, mergo.WithOverride); err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Error(err, "Fail to reconcile Ingress", "ingress", ingress)
		r.recorder.Event(instance, "Warning", "Failed", fmt.Sprintf("Fail to reconcile Ingress %s", ingress.Name))
	} else {
		switch op {
		case controllerutil.OperationResultCreated:
			r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("Created Ingress %s", ingress.Name))
		case controllerutil.OperationResultUpdated:
			r.recorder.Event(instance, "Normal", "Updated", fmt.Sprintf("Updated Ingress %s", ingress.Name))
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("server-controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&headscalev1alpha1.Server{}).
		// Owns(&certmanagerv1.Certificate{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
