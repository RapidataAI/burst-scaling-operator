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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BurstScalerSpec defines the desired state of BurstScaler.
type BurstScalerSpec struct {
	// RabbitMQ contains the RabbitMQ connection configuration.
	// +required
	RabbitMQ RabbitMQConfig `json:"rabbitmq"`

	// Source defines the source queue configuration for consuming messages.
	// +required
	Source SourceConfig `json:"source"`

	// Burst defines the burst publishing configuration.
	// +required
	Burst BurstConfig `json:"burst"`
}

// RabbitMQConfig contains the RabbitMQ connection configuration.
type RabbitMQConfig struct {
	// SecretRef references a secret containing the AMQP URI.
	// +required
	SecretRef SecretReference `json:"secretRef"`
}

// SecretReference references a key in a Secret.
type SecretReference struct {
	// Name is the name of the secret.
	// +required
	Name string `json:"name"`

	// Key is the key within the secret.
	// +required
	Key string `json:"key"`
}

// SourceConfig defines the source queue configuration.
type SourceConfig struct {
	// Exchange is the name of the exchange to bind to.
	// +required
	Exchange string `json:"exchange"`

	// RoutingKey is the routing key pattern for binding.
	// +required
	RoutingKey string `json:"routingKey"`

	// QueueName is the name of the source queue.
	// If not specified, defaults to "burst-<name>-source".
	// +optional
	QueueName string `json:"queueName,omitempty"`
}

// BurstConfig defines the burst publishing configuration.
type BurstConfig struct {
	// Count is the number of messages to publish on each burst.
	// This also determines the maximum number of replicas.
	// +kubebuilder:validation:Minimum=1
	// +required
	Count int32 `json:"count"`

	// MessageTTLSeconds is the TTL for messages in the target queue in seconds.
	// After this period, messages expire and pods scale down.
	// +kubebuilder:validation:Minimum=1
	// +required
	MessageTTLSeconds int32 `json:"messageTTLSeconds"`

	// TargetQueueName is the name of the target ring-buffer queue.
	// If not specified, defaults to "burst-<name>-target".
	// +optional
	TargetQueueName string `json:"targetQueueName,omitempty"`
}

// BurstScalerStatus defines the observed state of BurstScaler.
type BurstScalerStatus struct {
	// Conditions represent the current state of the BurstScaler.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the BurstScaler.
	// +optional
	Phase string `json:"phase,omitempty"`

	// SourceQueueName is the actual name of the source queue.
	// +optional
	SourceQueueName string `json:"sourceQueueName,omitempty"`

	// TargetQueueName is the actual name of the target queue.
	// +optional
	TargetQueueName string `json:"targetQueueName,omitempty"`

	// LastBurstTime is the timestamp of the last burst event.
	// +optional
	LastBurstTime *metav1.Time `json:"lastBurstTime,omitempty"`

	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// BurstScaler phase constants.
const (
	PhaseInitializing = "Initializing"
	PhaseRunning      = "Running"
	PhaseFailed       = "Failed"
)

// Condition type constants.
const (
	ConditionTypeReady           = "Ready"
	ConditionTypeQueuesReady     = "QueuesReady"
	ConditionTypeConsumerRunning = "ConsumerRunning"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Source Queue",type=string,JSONPath=`.status.sourceQueueName`
// +kubebuilder:printcolumn:name="Target Queue",type=string,JSONPath=`.status.targetQueueName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BurstScaler is the Schema for the burstscalers API.
type BurstScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of BurstScaler.
	// +required
	Spec BurstScalerSpec `json:"spec"`

	// Status defines the observed state of BurstScaler.
	// +optional
	Status BurstScalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BurstScalerList contains a list of BurstScaler.
type BurstScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BurstScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BurstScaler{}, &BurstScalerList{})
}

// GetSourceQueueName returns the source queue name, using default if not specified.
func (b *BurstScaler) GetSourceQueueName() string {
	if b.Spec.Source.QueueName != "" {
		return b.Spec.Source.QueueName
	}
	return "burst-" + b.Name + "-source"
}

// GetTargetQueueName returns the target queue name, using default if not specified.
func (b *BurstScaler) GetTargetQueueName() string {
	if b.Spec.Burst.TargetQueueName != "" {
		return b.Spec.Burst.TargetQueueName
	}
	return "burst-" + b.Name + "-target"
}
