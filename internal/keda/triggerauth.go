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
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	burstscalingv1alpha1 "github.com/rapidataai/burst-scaling-operator/api/v1alpha1"
	kedav1alpha1 "github.com/rapidataai/burst-scaling-operator/internal/keda/api"
)

// TriggerAuthName returns the name for the TriggerAuthentication associated with a BurstScaler.
func TriggerAuthName(bs *burstscalingv1alpha1.BurstScaler) string {
	return bs.Name + "-triggerauth"
}

// BuildTriggerAuthentication builds a KEDA TriggerAuthentication for the given BurstScaler.
// It references the RabbitMQ HTTP API secret for KEDA to authenticate with RabbitMQ.
func BuildTriggerAuthentication(bs *burstscalingv1alpha1.BurstScaler) *kedav1alpha1.TriggerAuthentication {
	// Use httpSecretRef if provided, otherwise use the AMQP secret
	secretRef := bs.Spec.RabbitMQ.SecretRef
	if bs.Spec.RabbitMQ.HTTPSecretRef != nil {
		secretRef = *bs.Spec.RabbitMQ.HTTPSecretRef
	}

	ta := &kedav1alpha1.TriggerAuthentication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "keda.sh/v1alpha1",
			Kind:       "TriggerAuthentication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      TriggerAuthName(bs),
			Namespace: bs.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "burstscaler",
				"app.kubernetes.io/instance":   bs.Name,
				"app.kubernetes.io/managed-by": "burst-scaling-operator",
			},
		},
		Spec: kedav1alpha1.TriggerAuthenticationSpec{
			SecretTargetRef: []kedav1alpha1.AuthSecretTargetRef{
				{
					Parameter: "host",
					Name:      secretRef.Name,
					Key:       secretRef.Key,
				},
			},
		},
	}

	return ta
}

// TriggerAuthNeedsUpdate checks if the TriggerAuthentication needs to be updated.
func TriggerAuthNeedsUpdate(current, desired *kedav1alpha1.TriggerAuthentication) bool {
	if len(current.Spec.SecretTargetRef) != len(desired.Spec.SecretTargetRef) {
		return true
	}

	if len(current.Spec.SecretTargetRef) > 0 && len(desired.Spec.SecretTargetRef) > 0 {
		currentRef := current.Spec.SecretTargetRef[0]
		desiredRef := desired.Spec.SecretTargetRef[0]

		if currentRef.Name != desiredRef.Name ||
			currentRef.Key != desiredRef.Key ||
			currentRef.Parameter != desiredRef.Parameter {
			return true
		}
	}

	return false
}

// UpdateTriggerAuth updates the current TriggerAuthentication with desired values.
func UpdateTriggerAuth(current, desired *kedav1alpha1.TriggerAuthentication) {
	current.Spec = desired.Spec
	// Preserve labels
	if current.Labels == nil {
		current.Labels = make(map[string]string)
	}
	maps.Copy(current.Labels, desired.Labels)
}
