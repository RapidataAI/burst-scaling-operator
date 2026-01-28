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

package keda

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	burstscalingv1alpha1 "github.com/rapidataai/burst-scaling-operator/api/v1alpha1"
	kedav1alpha1 "github.com/rapidataai/burst-scaling-operator/internal/keda/api"
)

// ScaledObjectName returns the name for the ScaledObject associated with a BurstScaler.
func ScaledObjectName(bs *burstscalingv1alpha1.BurstScaler) string {
	return bs.Name + "-scaledobject"
}

// BuildScaledObject builds a KEDA ScaledObject for the given BurstScaler.
func BuildScaledObject(bs *burstscalingv1alpha1.BurstScaler, triggerAuthName string) *kedav1alpha1.ScaledObject {
	targetQueueName := bs.GetTargetQueueName()
	minReplicas := bs.GetMinReplicaCount()
	maxReplicas := bs.GetMaxReplicaCount()
	cooldownPeriod := bs.GetCooldownPeriod()
	pollingInterval := bs.GetPollingInterval()

	so := &kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "keda.sh/v1alpha1",
			Kind:       "ScaledObject",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ScaledObjectName(bs),
			Namespace: bs.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "burstscaler",
				"app.kubernetes.io/instance":   bs.Name,
				"app.kubernetes.io/managed-by": "burst-scaling-operator",
			},
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name:       bs.Spec.Target.ScaleTargetRef.Name,
				Kind:       bs.GetScaleTargetKind(),
				APIVersion: bs.GetScaleTargetAPIVersion(),
			},
			MinReplicaCount: &minReplicas,
			MaxReplicaCount: &maxReplicas,
			CooldownPeriod:  &cooldownPeriod,
			PollingInterval: &pollingInterval,
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "rabbitmq",
					Metadata: map[string]string{
						"mode":      "QueueLength",
						"value":     "1",
						"queueName": targetQueueName,
						"protocol":  "http",
						"vhostName": "/",
					},
					AuthenticationRef: &kedav1alpha1.ScaledObjectAuthRef{
						Name: triggerAuthName,
					},
				},
			},
		},
	}

	return so
}

// ScaledObjectNeedsUpdate checks if the ScaledObject needs to be updated.
func ScaledObjectNeedsUpdate(current, desired *kedav1alpha1.ScaledObject) bool {
	// Check scale target
	if current.Spec.ScaleTargetRef == nil || desired.Spec.ScaleTargetRef == nil {
		return true
	}
	if current.Spec.ScaleTargetRef.Name != desired.Spec.ScaleTargetRef.Name ||
		current.Spec.ScaleTargetRef.Kind != desired.Spec.ScaleTargetRef.Kind ||
		current.Spec.ScaleTargetRef.APIVersion != desired.Spec.ScaleTargetRef.APIVersion {
		return true
	}

	// Check replica counts
	if !int32PtrEqual(current.Spec.MinReplicaCount, desired.Spec.MinReplicaCount) ||
		!int32PtrEqual(current.Spec.MaxReplicaCount, desired.Spec.MaxReplicaCount) {
		return true
	}

	// Check intervals
	if !int32PtrEqual(current.Spec.CooldownPeriod, desired.Spec.CooldownPeriod) ||
		!int32PtrEqual(current.Spec.PollingInterval, desired.Spec.PollingInterval) {
		return true
	}

	// Check triggers
	if len(current.Spec.Triggers) != len(desired.Spec.Triggers) {
		return true
	}
	if len(current.Spec.Triggers) > 0 && len(desired.Spec.Triggers) > 0 {
		currentTrigger := current.Spec.Triggers[0]
		desiredTrigger := desired.Spec.Triggers[0]

		if currentTrigger.Type != desiredTrigger.Type {
			return true
		}

		// Check metadata
		for key, val := range desiredTrigger.Metadata {
			if currentTrigger.Metadata[key] != val {
				return true
			}
		}

		// Check auth ref
		if currentTrigger.AuthenticationRef == nil && desiredTrigger.AuthenticationRef != nil {
			return true
		}
		if currentTrigger.AuthenticationRef != nil && desiredTrigger.AuthenticationRef != nil {
			if currentTrigger.AuthenticationRef.Name != desiredTrigger.AuthenticationRef.Name {
				return true
			}
		}
	}

	return false
}

// UpdateScaledObject updates the current ScaledObject with desired values.
func UpdateScaledObject(current, desired *kedav1alpha1.ScaledObject) {
	current.Spec = desired.Spec
	// Preserve labels
	if current.Labels == nil {
		current.Labels = make(map[string]string)
	}
	for k, v := range desired.Labels {
		current.Labels[k] = v
	}
}

func int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// GetHTTPURIFromAMQPURI converts an AMQP URI to an HTTP management API URI.
// This is a best-effort conversion for common RabbitMQ setups.
func GetHTTPURIFromAMQPURI(amqpURI string) (string, error) {
	// Basic conversion: amqp://user:pass@host:5672/vhost -> http://user:pass@host:15672/api
	// This is a simplified conversion - in production, users should provide httpSecretRef

	// For now, we'll just return a placeholder since proper URL parsing
	// and conversion is complex and users should ideally provide httpSecretRef
	return "", fmt.Errorf("automatic AMQP to HTTP URI conversion not implemented, please provide httpSecretRef")
}
