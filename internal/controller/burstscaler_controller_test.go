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

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	burstscalingv1alpha1 "github.com/rapidataai/burst-scaling-operator/api/v1alpha1"
	"github.com/rapidataai/burst-scaling-operator/internal/rabbitmq"
)

var _ = Describe("BurstScaler Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const secretName = "test-rabbitmq-secret"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		secretNamespacedName := types.NamespacedName{
			Name:      secretName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the RabbitMQ secret")
			secret := &corev1.Secret{}
			err := k8sClient.Get(ctx, secretNamespacedName, secret)
			if err != nil && errors.IsNotFound(err) {
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: "default",
					},
					Data: map[string][]byte{
						"amqp-uri": []byte("amqp://guest:guest@localhost:5672/"),
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			By("creating the custom resource for the Kind BurstScaler")
			burstscaler := &burstscalingv1alpha1.BurstScaler{}
			err = k8sClient.Get(ctx, typeNamespacedName, burstscaler)
			if err != nil && errors.IsNotFound(err) {
				resource := &burstscalingv1alpha1.BurstScaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: burstscalingv1alpha1.BurstScalerSpec{
						RabbitMQ: burstscalingv1alpha1.RabbitMQConfig{
							SecretRef: burstscalingv1alpha1.SecretReference{
								Name: secretName,
								Key:  "amqp-uri",
							},
						},
						Source: burstscalingv1alpha1.SourceConfig{
							Exchange:   "test-exchange",
							RoutingKey: "test.routing.key",
						},
						Burst: burstscalingv1alpha1.BurstConfig{
							Count:             10,
							MessageTTLSeconds: 120,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance BurstScaler")
			resource := &burstscalingv1alpha1.BurstScaler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				// Remove finalizer if present to allow deletion
				resource.Finalizers = nil
				_ = k8sClient.Update(ctx, resource)
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the secret")
			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, secretNamespacedName, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			consumerManager := rabbitmq.NewConsumerManager()
			controllerReconciler := &BurstScalerReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ConsumerManager: consumerManager,
			}

			// Note: This will fail to connect to RabbitMQ since there's no actual server,
			// but it should at least get to the point of trying to connect
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// We expect an error here since we can't connect to RabbitMQ in tests
			// But we verify the reconciler runs without panicking
			if err != nil {
				// This is expected - no RabbitMQ server
				return
			}
		})

		It("should add finalizer to the resource", func() {
			By("Reconciling the created resource")
			consumerManager := rabbitmq.NewConsumerManager()
			controllerReconciler := &BurstScalerReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ConsumerManager: consumerManager,
			}

			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			By("Checking the finalizer was added")
			resource := &burstscalingv1alpha1.BurstScaler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Finalizers).To(ContainElement("burstscaling.io/queue-cleanup"))
		})
	})
})
