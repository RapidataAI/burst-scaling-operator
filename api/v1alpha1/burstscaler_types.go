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

	// Target defines the scaling target configuration.
	// +required
	Target TargetConfig `json:"target"`
}

// RabbitMQConfig contains the RabbitMQ connection configuration.
type RabbitMQConfig struct {
	// SecretRef references a secret containing the AMQP URI.
	// +required
	SecretRef SecretReference `json:"secretRef"`

	// HTTPSecretRef optionally references a secret containing the RabbitMQ HTTP API URI.
	// Used by KEDA for queue length monitoring. If not specified, derived from AMQP URI.
	// +optional
	HTTPSecretRef *SecretReference `json:"httpSecretRef,omitempty"`
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

// TargetConfig defines the scaling target configuration.
type TargetConfig struct {
	// ScaleTargetRef identifies the resource to scale.
	// +required
	ScaleTargetRef ScaleTargetRef `json:"scaleTargetRef"`

	// MinReplicaCount is the minimum number of replicas.
	// Defaults to 0.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinReplicaCount *int32 `json:"minReplicaCount,omitempty"`

	// MaxReplicaCount is the maximum number of replicas.
	// Defaults to burst.count.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxReplicaCount *int32 `json:"maxReplicaCount,omitempty"`

	// CooldownPeriod is the period to wait after the last trigger reported active before scaling to minReplicaCount.
	// Defaults to 300 seconds.
	// +optional
	// +kubebuilder:validation:Minimum=0
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty"`

	// PollingInterval is the interval in seconds to check each trigger on.
	// Defaults to 5 seconds.
	// +optional
	// +kubebuilder:validation:Minimum=1
	PollingInterval *int32 `json:"pollingInterval,omitempty"`
}

// ScaleTargetRef identifies the resource to scale.
type ScaleTargetRef struct {
	// Name is the name of the resource to scale.
	// +required
	Name string `json:"name"`

	// Kind is the kind of the resource to scale.
	// Defaults to "Deployment".
	// +optional
	Kind string `json:"kind,omitempty"`

	// APIVersion is the API version of the resource to scale.
	// Defaults to "apps/v1".
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
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
	ConditionTypeKEDAReady       = "KEDAReady"
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

// GetScaleTargetKind returns the scale target kind, using default if not specified.
func (b *BurstScaler) GetScaleTargetKind() string {
	if b.Spec.Target.ScaleTargetRef.Kind != "" {
		return b.Spec.Target.ScaleTargetRef.Kind
	}
	return "Deployment"
}

// GetScaleTargetAPIVersion returns the scale target API version, using default if not specified.
func (b *BurstScaler) GetScaleTargetAPIVersion() string {
	if b.Spec.Target.ScaleTargetRef.APIVersion != "" {
		return b.Spec.Target.ScaleTargetRef.APIVersion
	}
	return "apps/v1"
}

// GetMinReplicaCount returns the min replica count, using default if not specified.
func (b *BurstScaler) GetMinReplicaCount() int32 {
	if b.Spec.Target.MinReplicaCount != nil {
		return *b.Spec.Target.MinReplicaCount
	}
	return 0
}

// GetMaxReplicaCount returns the max replica count, using burst.count if not specified.
func (b *BurstScaler) GetMaxReplicaCount() int32 {
	if b.Spec.Target.MaxReplicaCount != nil {
		return *b.Spec.Target.MaxReplicaCount
	}
	return b.Spec.Burst.Count
}

// GetCooldownPeriod returns the cooldown period, using default if not specified.
func (b *BurstScaler) GetCooldownPeriod() int32 {
	if b.Spec.Target.CooldownPeriod != nil {
		return *b.Spec.Target.CooldownPeriod
	}
	return 300
}

// GetPollingInterval returns the polling interval, using default if not specified.
func (b *BurstScaler) GetPollingInterval() int32 {
	if b.Spec.Target.PollingInterval != nil {
		return *b.Spec.Target.PollingInterval
	}
	return 5
}
