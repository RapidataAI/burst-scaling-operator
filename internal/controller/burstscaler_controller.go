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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	burstscalingv1alpha1 "github.com/rapidataai/burst-scaling-operator/api/v1alpha1"
	"github.com/rapidataai/burst-scaling-operator/internal/keda"
	kedav1alpha1 "github.com/rapidataai/burst-scaling-operator/internal/keda/api"
	"github.com/rapidataai/burst-scaling-operator/internal/metrics"
	"github.com/rapidataai/burst-scaling-operator/internal/rabbitmq"
)

const (
	finalizerName = "burstscaling.io/queue-cleanup"
)

// BurstScalerReconciler reconciles a BurstScaler object
type BurstScalerReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ConsumerManager *rabbitmq.ConsumerManager
}

// +kubebuilder:rbac:groups=burstscaling.io,resources=burstscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=burstscaling.io,resources=burstscalers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=burstscaling.io,resources=burstscalers/finalizers,verbs=update
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keda.sh,resources=triggerauthentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles a BurstScaler resource.
func (r *BurstScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	startTime := time.Now()

	defer func() {
		metrics.ReconcileDuration.WithLabelValues(req.Name, req.Namespace).Observe(time.Since(startTime).Seconds())
	}()

	// Fetch the BurstScaler
	var burstScaler burstscalingv1alpha1.BurstScaler
	if err := r.Get(ctx, req.NamespacedName, &burstScaler); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, stop consumer if running
			r.ConsumerManager.StopConsumer(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Unable to fetch BurstScaler")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !burstScaler.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &burstScaler)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&burstScaler, finalizerName) {
		controllerutil.AddFinalizer(&burstScaler, finalizerName)
		if err := r.Update(ctx, &burstScaler); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial phase
	if burstScaler.Status.Phase == "" {
		burstScaler.Status.Phase = burstscalingv1alpha1.PhaseInitializing
		if err := r.Status().Update(ctx, &burstScaler); err != nil {
			logger.Error(err, "Failed to update status phase")
			return ctrl.Result{}, err
		}
	}

	// Get RabbitMQ URI from secret
	amqpURI, err := r.getSecretValue(ctx, burstScaler.Namespace, burstScaler.Spec.RabbitMQ.SecretRef)
	if err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "SecretNotFound", err.Error())
	}

	// Create RabbitMQ client
	rmqClient := rabbitmq.NewClient(amqpURI)
	if err := rmqClient.Connect(ctx); err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "RabbitMQConnectionFailed", err.Error())
	}
	defer func(rmqClient *rabbitmq.Client) {
		err := rmqClient.Close()
		if err != nil {
			logger.Error(err, "Failed to close RabbitMQ client")
		}
	}(rmqClient)

	// Declare source queue
	sourceQueueName := burstScaler.GetSourceQueueName()
	err = rmqClient.DeclareSourceQueue(ctx, rabbitmq.SourceQueueConfig{
		QueueName:  sourceQueueName,
		Exchange:   burstScaler.Spec.Source.Exchange,
		RoutingKey: burstScaler.Spec.Source.RoutingKey,
	})
	if err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "SourceQueueFailed", err.Error())
	}

	// Declare target queue
	targetQueueName := burstScaler.GetTargetQueueName()
	err = rmqClient.DeclareTargetQueue(ctx, rabbitmq.TargetQueueConfig{
		QueueName:  targetQueueName,
		MaxLength:  burstScaler.Spec.Burst.Count,
		TTLSeconds: burstScaler.Spec.Burst.MessageTTLSeconds,
	})
	if err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "TargetQueueFailed", err.Error())
	}

	// Update queue names in status
	burstScaler.Status.SourceQueueName = sourceQueueName
	burstScaler.Status.TargetQueueName = targetQueueName

	// Set QueuesReady condition
	meta.SetStatusCondition(&burstScaler.Status.Conditions, metav1.Condition{
		Type:               burstscalingv1alpha1.ConditionTypeQueuesReady,
		Status:             metav1.ConditionTrue,
		Reason:             "QueuesCreated",
		Message:            fmt.Sprintf("Source queue %s and target queue %s created", sourceQueueName, targetQueueName),
		ObservedGeneration: burstScaler.Generation,
	})

	// Create/update TriggerAuthentication
	triggerAuthName := keda.TriggerAuthName(&burstScaler)
	if err := r.reconcileTriggerAuthentication(ctx, &burstScaler); err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "TriggerAuthFailed", err.Error())
	}

	// Create/update ScaledObject
	if err := r.reconcileScaledObject(ctx, &burstScaler, triggerAuthName); err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "ScaledObjectFailed", err.Error())
	}

	// Set KEDAReady condition
	meta.SetStatusCondition(&burstScaler.Status.Conditions, metav1.Condition{
		Type:               burstscalingv1alpha1.ConditionTypeKEDAReady,
		Status:             metav1.ConditionTrue,
		Reason:             "KEDAResourcesCreated",
		Message:            "TriggerAuthentication and ScaledObject created",
		ObservedGeneration: burstScaler.Generation,
	})

	// Start consumer
	err = r.ConsumerManager.EnsureConsumer(ctx, req.NamespacedName, rabbitmq.ConsumerConfig{
		AMQPURI:         amqpURI,
		SourceQueueName: sourceQueueName,
		TargetQueueName: targetQueueName,
		BurstCount:      burstScaler.Spec.Burst.Count,
	})
	if err != nil {
		return r.setFailedCondition(ctx, &burstScaler, "ConsumerFailed", err.Error())
	}

	// Set ConsumerRunning condition
	meta.SetStatusCondition(&burstScaler.Status.Conditions, metav1.Condition{
		Type:               burstscalingv1alpha1.ConditionTypeConsumerRunning,
		Status:             metav1.ConditionTrue,
		Reason:             "ConsumerStarted",
		Message:            "Consumer goroutine is running",
		ObservedGeneration: burstScaler.Generation,
	})

	// Set Ready condition
	meta.SetStatusCondition(&burstScaler.Status.Conditions, metav1.Condition{
		Type:               burstscalingv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSuccess",
		Message:            "BurstScaler is ready",
		ObservedGeneration: burstScaler.Generation,
	})

	// Update phase and generation
	burstScaler.Status.Phase = burstscalingv1alpha1.PhaseRunning
	burstScaler.Status.ObservedGeneration = burstScaler.Generation

	if err := r.Status().Update(ctx, &burstScaler); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	metrics.ReconcileTotal.WithLabelValues(req.Name, req.Namespace, "success").Inc()
	logger.Info("Successfully reconciled BurstScaler")

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a BurstScaler.
func (r *BurstScalerReconciler) handleDeletion(ctx context.Context, bs *burstscalingv1alpha1.BurstScaler) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(bs, finalizerName) {
		return ctrl.Result{}, nil
	}

	logger.Info("Handling deletion, cleaning up resources")

	// Stop consumer
	key := types.NamespacedName{Name: bs.Name, Namespace: bs.Namespace}
	r.ConsumerManager.StopConsumer(key)

	// Try to delete queues from RabbitMQ
	amqpURI, err := r.getSecretValue(ctx, bs.Namespace, bs.Spec.RabbitMQ.SecretRef)
	if err == nil {
		rmqClient := rabbitmq.NewClient(amqpURI)
		if err := rmqClient.Connect(ctx); err == nil {
			defer func(rmqClient *rabbitmq.Client) {
				err := rmqClient.Close()
				if err != nil {
					logger.Error(err, "Failed to close RabbitMQ client")
				}
			}(rmqClient)

			// Delete source queue
			if bs.Status.SourceQueueName != "" {
				if err := rmqClient.DeleteQueue(ctx, bs.Status.SourceQueueName); err != nil {
					logger.Error(err, "Failed to delete source queue", "queue", bs.Status.SourceQueueName)
				}
			}

			// Delete target queue
			if bs.Status.TargetQueueName != "" {
				if err := rmqClient.DeleteQueue(ctx, bs.Status.TargetQueueName); err != nil {
					logger.Error(err, "Failed to delete target queue", "queue", bs.Status.TargetQueueName)
				}
			}
		} else {
			logger.Error(err, "Failed to connect to RabbitMQ for cleanup")
		}
	} else {
		logger.Error(err, "Failed to get RabbitMQ secret for cleanup")
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(bs, finalizerName)
	if err := r.Update(ctx, bs); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully cleaned up BurstScaler")
	return ctrl.Result{}, nil
}

// getSecretValue retrieves a value from a Kubernetes secret.
func (r *BurstScalerReconciler) getSecretValue(ctx context.Context, namespace string, ref burstscalingv1alpha1.SecretReference) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", ref.Name, err)
	}

	value, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s", ref.Key, ref.Name)
	}

	return string(value), nil
}

// reconcileTriggerAuthentication creates or updates the TriggerAuthentication.
func (r *BurstScalerReconciler) reconcileTriggerAuthentication(ctx context.Context, bs *burstscalingv1alpha1.BurstScaler) error {
	logger := log.FromContext(ctx)

	desired := keda.BuildTriggerAuthentication(bs)

	// Set owner reference
	if err := controllerutil.SetControllerReference(bs, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if it exists
	var current kedav1alpha1.TriggerAuthentication
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create TriggerAuthentication: %w", err)
			}
			logger.Info("Created TriggerAuthentication", "name", desired.Name)
			return nil
		}
		return fmt.Errorf("failed to get TriggerAuthentication: %w", err)
	}

	// Update if needed
	if keda.TriggerAuthNeedsUpdate(&current, desired) {
		keda.UpdateTriggerAuth(&current, desired)
		if err := r.Update(ctx, &current); err != nil {
			return fmt.Errorf("failed to update TriggerAuthentication: %w", err)
		}
		logger.Info("Updated TriggerAuthentication", "name", desired.Name)
	}

	return nil
}

// reconcileScaledObject creates or updates the ScaledObject.
func (r *BurstScalerReconciler) reconcileScaledObject(ctx context.Context, bs *burstscalingv1alpha1.BurstScaler, triggerAuthName string) error {
	logger := log.FromContext(ctx)

	desired := keda.BuildScaledObject(bs, triggerAuthName)

	// Set owner reference
	if err := controllerutil.SetControllerReference(bs, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if it exists
	var current kedav1alpha1.ScaledObject
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create ScaledObject: %w", err)
			}
			logger.Info("Created ScaledObject", "name", desired.Name)
			return nil
		}
		return fmt.Errorf("failed to get ScaledObject: %w", err)
	}

	// Update if needed
	if keda.ScaledObjectNeedsUpdate(&current, desired) {
		keda.UpdateScaledObject(&current, desired)
		if err := r.Update(ctx, &current); err != nil {
			return fmt.Errorf("failed to update ScaledObject: %w", err)
		}
		logger.Info("Updated ScaledObject", "name", desired.Name)
	}

	return nil
}

// setFailedCondition sets the Ready condition to False and updates the status.
func (r *BurstScalerReconciler) setFailedCondition(ctx context.Context, bs *burstscalingv1alpha1.BurstScaler, reason, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	meta.SetStatusCondition(&bs.Status.Conditions, metav1.Condition{
		Type:               burstscalingv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: bs.Generation,
	})

	bs.Status.Phase = burstscalingv1alpha1.PhaseFailed
	bs.Status.ObservedGeneration = bs.Generation

	if err := r.Status().Update(ctx, bs); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	metrics.ReconcileTotal.WithLabelValues(bs.Name, bs.Namespace, "failed").Inc()
	logger.Error(nil, "Reconcile failed", "reason", reason, "message", message)

	// Requeue after a delay to retry
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BurstScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&burstscalingv1alpha1.BurstScaler{}).
		Owns(&kedav1alpha1.ScaledObject{}).
		Owns(&kedav1alpha1.TriggerAuthentication{}).
		Named("burstscaler").
		Complete(r)
}
