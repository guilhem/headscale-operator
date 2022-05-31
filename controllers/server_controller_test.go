package controllers_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	headscalev1alpha1 "github.com/guilhem/headscale-operator/api/v1alpha1"
	"github.com/guilhem/headscale-operator/controllers"
	"github.com/guilhem/headscale-operator/pkg/headscale"
)

var _ = Describe("Controllers/ServerController", func() {
	const (
		InstanceName      = "test-server"
		InstanceNamespace = "default"
		Host              = "test.domain.com"

		timeout  = time.Second * 30
		duration = "10s"
		interval = "1s"
	)

	Context("When creating new server", func() {
		It("Should create Server", func() {

			ctx := context.Background()

			By("Creating a new Instance")

			instance := &headscalev1alpha1.Server{
				ObjectMeta: metav1.ObjectMeta{
					Name:      InstanceName,
					Namespace: InstanceNamespace,
				},
				Spec: headscalev1alpha1.ServerSpec{
					Version: "0.15.O",
					Ingress: &networkingv1.Ingress{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: networkingv1.IngressSpec{
							IngressClassName: pointer.String("ingress-class"),
						},
					},
					Config: headscale.Config{
						LogLevel: "debug",
					},
					Host: Host,
				},
			}

			// ingress := &networkingv1.Ingress{
			// 	ObjectMeta: metav1.ObjectMeta{},
			// 	Spec: networkingv1.IngressSpec{
			// 		IngressClassName: pointer.String("ingressClass"),
			// 	},
			// }

			Expect(k8sClient.Create(ctx, instance)).Should(Succeed())

			By("Should have deployment")

			Eventually(func() (int, error) {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(instance), instance); err != nil {
					return -1, err

				}
				return len(instance.Status.DeploymentName), nil
			}, duration, interval).ShouldNot(Equal(0))

			By("Should have the rigth configuration")

			Eventually(func() (string, error) {
				confimap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      InstanceName,
						Namespace: InstanceNamespace,
					},
				}

				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(confimap), confimap); err != nil {
					return "", err
				}

				json := confimap.Data[controllers.ConfigFileName]

				if !gjson.Valid(json) {
					return "", errors.New("not valid json")
				}

				logLevel := gjson.Get(json, "log_level")
				return logLevel.String(), nil

			}, duration, interval).Should(Equal("debug"))

			By("Should have ingress")

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      InstanceName,
					Namespace: InstanceNamespace,
				},
			}

			Eventually(func() (int, error) {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress); err != nil {
					return -1, err
				}

				size := len(ingress.Spec.Rules)

				if ingress.Spec.Rules[0].Host != Host {
					return size, errors.New("host unmatch")
				}

				if *ingress.Spec.IngressClassName != "ingress-class" {
					return size, errors.New("missing ingress class")
				}
				return size, nil
			}, duration, interval).Should(Equal(1))

			// By("Should have pod ready")

			// createdDeployment := &appsv1.Deployment{}

			// Eventually(func() (int, error) {
			// 	if err := k8sClient.Get(ctx, types.NamespacedName{Name: instance.Status.DeploymentName, Namespace: instance.Namespace}, createdDeployment); err != nil {
			// 		return -1, err
			// 	}

			// 	return int(createdDeployment.Status.AvailableReplicas), nil
			// }, duration, interval).ShouldNot(BeZero())

		})
	})
})
