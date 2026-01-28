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

// Package api contains KEDA CRD types for ScaledObject and TriggerAuthentication.
// These are local definitions to avoid dependency conflicts with KEDA's webhooks.
// +kubebuilder:object:generate=true
package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=scaledobjects,scope=Namespaced,shortName=so

// ScaledObject is a KEDA resource that defines how to scale a workload.
type ScaledObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScaledObjectSpec   `json:"spec,omitempty"`
	Status ScaledObjectStatus `json:"status,omitempty"`
}

// ScaledObjectSpec defines the desired state of ScaledObject.
type ScaledObjectSpec struct {
	// ScaleTargetRef holds reference to the target resource.
	ScaleTargetRef *ScaleTarget `json:"scaleTargetRef"`

	// PollingInterval is the interval to check each trigger on.
	// +optional
	PollingInterval *int32 `json:"pollingInterval,omitempty"`

	// CooldownPeriod is the period to wait after the last trigger reported active before scaling to 0.
	// +optional
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty"`

	// MinReplicaCount is the minimum number of replicas KEDA will scale the resource down to.
	// +optional
	MinReplicaCount *int32 `json:"minReplicaCount,omitempty"`

	// MaxReplicaCount is the maximum number of replicas KEDA will scale the resource up to.
	// +optional
	MaxReplicaCount *int32 `json:"maxReplicaCount,omitempty"`

	// Advanced holds advanced configuration options.
	// +optional
	Advanced *AdvancedConfig `json:"advanced,omitempty"`

	// Triggers is a list of triggers to activate scaling of the target resource.
	Triggers []ScaleTriggers `json:"triggers"`

	// Fallback is the config for specifying the fallback behavior.
	// +optional
	Fallback *Fallback `json:"fallback,omitempty"`

	// IdleReplicaCount is the number of replicas KEDA will keep when no triggers are active.
	// +optional
	IdleReplicaCount *int32 `json:"idleReplicaCount,omitempty"`
}

// ScaleTarget holds the reference to the scale target.
type ScaleTarget struct {
	// Name of the target resource.
	Name string `json:"name"`

	// APIVersion of the target resource.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the target resource.
	// +optional
	Kind string `json:"kind,omitempty"`

	// EnvSourceContainerName specifies the name of the container from which to get env vars for scaling.
	// +optional
	EnvSourceContainerName string `json:"envSourceContainerName,omitempty"`
}

// AdvancedConfig holds advanced configuration options.
type AdvancedConfig struct {
	// HorizontalPodAutoscalerConfig holds the HPA configuration.
	// +optional
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig `json:"horizontalPodAutoscalerConfig,omitempty"`

	// RestoreToOriginalReplicaCount specifies whether to restore replicas on deletion.
	// +optional
	RestoreToOriginalReplicaCount bool `json:"restoreToOriginalReplicaCount,omitempty"`

	// ScalingModifiers holds the scaling modifiers configuration.
	// +optional
	ScalingModifiers ScalingModifiers `json:"scalingModifiers,omitempty"`
}

// HorizontalPodAutoscalerConfig holds the HPA configuration.
type HorizontalPodAutoscalerConfig struct {
	// Name of the HPA resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Behavior configures the scaling behavior of the target.
	// +optional
	Behavior *HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// HorizontalPodAutoscalerBehavior configures the scaling behavior of the target.
type HorizontalPodAutoscalerBehavior struct {
	// ScaleUp is scaling policy for scaling Up.
	// +optional
	ScaleUp *HPAScalingRules `json:"scaleUp,omitempty"`

	// ScaleDown is scaling policy for scaling Down.
	// +optional
	ScaleDown *HPAScalingRules `json:"scaleDown,omitempty"`
}

// HPAScalingRules configures the scaling behavior for one direction.
type HPAScalingRules struct {
	// StabilizationWindowSeconds is the number of seconds for which past recommendations should be
	// considered while scaling up or scaling down.
	// +optional
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`

	// SelectPolicy is used to specify which policy should be used.
	// +optional
	SelectPolicy *string `json:"selectPolicy,omitempty"`

	// Policies is a list of potential scaling polices which can be used during scaling.
	// +optional
	Policies []HPAScalingPolicy `json:"policies,omitempty"`
}

// HPAScalingPolicy is a single policy which must hold true for a specified past interval.
type HPAScalingPolicy struct {
	// Type is used to specify the scaling policy.
	Type string `json:"type"`

	// Value contains the amount of change which is permitted by the policy.
	Value int32 `json:"value"`

	// PeriodSeconds specifies the window of time for which the policy should hold true.
	PeriodSeconds int32 `json:"periodSeconds"`
}

// ScalingModifiers holds the scaling modifiers configuration.
type ScalingModifiers struct {
	// Target is the desired count of the trigger's metric target.
	// +optional
	Target string `json:"target,omitempty"`

	// ActivationTarget is the activation target of the trigger's metric.
	// +optional
	ActivationTarget string `json:"activationTarget,omitempty"`

	// MetricType is the type of metric to use.
	// +optional
	MetricType string `json:"metricType,omitempty"`

	// Formula is used to specify a formula to calculate the metric.
	// +optional
	Formula string `json:"formula,omitempty"`
}

// ScaleTriggers reference the scaler that will be used.
type ScaleTriggers struct {
	// Type is the type of trigger.
	Type string `json:"type"`

	// Name is the name of the trigger.
	// +optional
	Name string `json:"name,omitempty"`

	// UseCachedMetrics enables caching of metrics.
	// +optional
	UseCachedMetrics bool `json:"useCachedMetrics,omitempty"`

	// Metadata contains the trigger configuration.
	Metadata map[string]string `json:"metadata"`

	// AuthenticationRef references the TriggerAuthentication.
	// +optional
	AuthenticationRef *ScaledObjectAuthRef `json:"authenticationRef,omitempty"`

	// MetricType is the type of metric to use.
	// +optional
	MetricType string `json:"metricType,omitempty"`
}

// ScaledObjectAuthRef references a TriggerAuthentication or ClusterTriggerAuthentication.
type ScaledObjectAuthRef struct {
	// Name of the TriggerAuthentication.
	Name string `json:"name"`

	// Kind of the auth resource (TriggerAuthentication or ClusterTriggerAuthentication).
	// +optional
	Kind string `json:"kind,omitempty"`
}

// Fallback is the config for specifying the fallback behavior.
type Fallback struct {
	// FailureThreshold is the number of times a trigger must fail before using the fallback.
	FailureThreshold int32 `json:"failureThreshold"`

	// Replicas is the number of replicas to fallback to.
	Replicas int32 `json:"replicas"`
}

// ScaledObjectStatus defines the observed state of ScaledObject.
type ScaledObjectStatus struct {
	// ScaleTargetKind is the kind of the target resource.
	// +optional
	ScaleTargetKind string `json:"scaleTargetKind,omitempty"`

	// ScaleTargetGVKR is the GroupVersionKindResource of the target.
	// +optional
	ScaleTargetGVKR *GroupVersionKindResource `json:"scaleTargetGVKR,omitempty"`

	// OriginalReplicaCount is the original replica count of the target.
	// +optional
	OriginalReplicaCount *int32 `json:"originalReplicaCount,omitempty"`

	// LastActiveTime is the last time a trigger was active.
	// +optional
	LastActiveTime *metav1.Time `json:"lastActiveTime,omitempty"`

	// ExternalMetricNames contains the names of the external metrics.
	// +optional
	ExternalMetricNames []string `json:"externalMetricNames,omitempty"`

	// ResourceMetricNames contains the names of the resource metrics.
	// +optional
	ResourceMetricNames []string `json:"resourceMetricNames,omitempty"`

	// Conditions contains the conditions of the ScaledObject.
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`

	// Health contains the health status of each trigger.
	// +optional
	Health map[string]HealthStatus `json:"health,omitempty"`

	// PausedReplicaCount is the replica count when paused.
	// +optional
	PausedReplicaCount *int32 `json:"pausedReplicaCount,omitempty"`

	// HpaName is the name of the HPA created by KEDA.
	// +optional
	HpaName string `json:"hpaName,omitempty"`

	// CompositeScalerName is the name of the composite scaler.
	// +optional
	CompositeScalerName string `json:"compositeScalerName,omitempty"`
}

// GroupVersionKindResource represents the GroupVersionKindResource of a resource.
type GroupVersionKindResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
}

// HealthStatus represents the health status of a trigger.
type HealthStatus struct {
	// NumberOfFailures is the number of consecutive failures.
	// +optional
	NumberOfFailures *int32 `json:"numberOfFailures,omitempty"`

	// Status represents the current health status.
	// +optional
	Status HealthStatusType `json:"status,omitempty"`
}

// HealthStatusType represents the health status.
type HealthStatusType string

// Conditions is a list of conditions.
type Conditions []Condition

// Condition contains details for the current condition.
type Condition struct {
	// Type is the type of the condition.
	Type ConditionType `json:"type"`

	// Status is the status of the condition.
	Status metav1.ConditionStatus `json:"status"`

	// Reason is a brief reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// ConditionType is the type of a condition.
type ConditionType string

// +kubebuilder:object:root=true

// ScaledObjectList contains a list of ScaledObject.
type ScaledObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScaledObject `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=triggerauthentications,scope=Namespaced,shortName=ta

// TriggerAuthentication defines how a trigger can authenticate.
type TriggerAuthentication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TriggerAuthenticationSpec `json:"spec"`
}

// TriggerAuthenticationSpec defines the various ways to authenticate.
type TriggerAuthenticationSpec struct {
	// PodIdentity allows you to use pod identity providers.
	// +optional
	PodIdentity *AuthPodIdentity `json:"podIdentity,omitempty"`

	// SecretTargetRef references Kubernetes secrets containing credentials.
	// +optional
	SecretTargetRef []AuthSecretTargetRef `json:"secretTargetRef,omitempty"`

	// Env references environment variables on the container.
	// +optional
	Env []AuthEnvironment `json:"env,omitempty"`

	// HashiCorpVault references a HashiCorp Vault secret.
	// +optional
	HashiCorpVault *HashiCorpVault `json:"hashiCorpVault,omitempty"`

	// AzureKeyVault references an Azure Key Vault secret.
	// +optional
	AzureKeyVault *AzureKeyVault `json:"azureKeyVault,omitempty"`

	// GCPSecretManager references a GCP Secret Manager secret.
	// +optional
	GCPSecretManager *GCPSecretManager `json:"gcpSecretManager,omitempty"`

	// AwsSecretManager references an AWS Secrets Manager secret.
	// +optional
	AwsSecretManager *AwsSecretManager `json:"awsSecretManager,omitempty"`

	// ConfigMapTargetRef references ConfigMap entries containing credentials.
	// +optional
	ConfigMapTargetRef []AuthConfigMapTargetRef `json:"configMapTargetRef,omitempty"`
}

// AuthPodIdentity allows you to use pod identity providers.
type AuthPodIdentity struct {
	// Provider is the pod identity provider.
	Provider string `json:"provider"`

	// IdentityID is the identity ID.
	// +optional
	IdentityID *string `json:"identityId,omitempty"`

	// IdentityOwner is the owner of the identity.
	// +optional
	IdentityOwner *string `json:"identityOwner,omitempty"`

	// IdentityTenantID is the tenant ID.
	// +optional
	IdentityTenantID *string `json:"identityTenantId,omitempty"`

	// RoleArn is the AWS role ARN.
	// +optional
	RoleArn *string `json:"roleArn,omitempty"`
}

// AuthSecretTargetRef references a Kubernetes secret containing credentials.
type AuthSecretTargetRef struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Name is the name of the secret.
	Name string `json:"name"`

	// Key is the key in the secret.
	Key string `json:"key"`
}

// AuthEnvironment references an environment variable.
type AuthEnvironment struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Name is the name of the env var.
	Name string `json:"name"`

	// ContainerName is the name of the container.
	// +optional
	ContainerName string `json:"containerName,omitempty"`
}

// HashiCorpVault references a HashiCorp Vault secret.
type HashiCorpVault struct {
	// Address is the Vault address.
	Address string `json:"address"`

	// Authentication is the authentication method.
	Authentication string `json:"authentication"`

	// Secrets is the list of secrets to retrieve.
	Secrets []VaultSecret `json:"secrets"`

	// Namespace is the Vault namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Role is the Vault role.
	// +optional
	Role string `json:"role,omitempty"`

	// Mount is the auth mount path.
	// +optional
	Mount string `json:"mount,omitempty"`

	// Credential is the credential for authentication.
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// VaultSecret represents a Vault secret.
type VaultSecret struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Path is the path to the secret.
	Path string `json:"path"`

	// Key is the key in the secret.
	Key string `json:"key"`

	// Type is the type of secret.
	// +optional
	Type string `json:"type,omitempty"`

	// PKIData is the PKI data.
	// +optional
	PKIData *VaultPkiData `json:"pkiData,omitempty"`
}

// VaultPkiData represents PKI data.
type VaultPkiData struct {
	// CommonName is the common name.
	// +optional
	CommonName string `json:"commonName,omitempty"`

	// AltNames is the list of alternative names.
	// +optional
	AltNames string `json:"altNames,omitempty"`

	// IPSans is the list of IP SANs.
	// +optional
	IPSans string `json:"ipSans,omitempty"`

	// URISans is the list of URI SANs.
	// +optional
	URISans string `json:"uriSans,omitempty"`

	// TTL is the TTL.
	// +optional
	TTL string `json:"ttl,omitempty"`

	// Format is the format.
	// +optional
	Format string `json:"format,omitempty"`
}

// Credential represents a credential for authentication.
type Credential struct {
	// Token is the token.
	// +optional
	Token string `json:"token,omitempty"`

	// ServiceAccount is the service account.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// AzureKeyVault references an Azure Key Vault secret.
type AzureKeyVault struct {
	// VaultURI is the Azure Key Vault URI.
	VaultURI string `json:"vaultUri"`

	// PodIdentity is the pod identity.
	// +optional
	PodIdentity *AuthPodIdentity `json:"podIdentity,omitempty"`

	// Credentials are the Azure credentials.
	// +optional
	Credentials *AzureKeyVaultCredentials `json:"credentials,omitempty"`

	// Secrets is the list of secrets to retrieve.
	Secrets []AzureKeyVaultSecret `json:"secrets"`

	// Cloud is the Azure cloud.
	// +optional
	Cloud *AzureKeyVaultCloudInfo `json:"cloud,omitempty"`
}

// AzureKeyVaultCredentials represents Azure credentials.
type AzureKeyVaultCredentials struct {
	// ClientID is the client ID.
	ClientID string `json:"clientId"`

	// ClientSecret is the client secret.
	ClientSecret AuthSecretTargetRef `json:"clientSecret"`

	// TenantID is the tenant ID.
	TenantID string `json:"tenantId"`
}

// AzureKeyVaultSecret represents an Azure Key Vault secret.
type AzureKeyVaultSecret struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Name is the name of the secret.
	Name string `json:"name"`

	// Version is the version of the secret.
	// +optional
	Version string `json:"version,omitempty"`
}

// AzureKeyVaultCloudInfo represents Azure cloud info.
type AzureKeyVaultCloudInfo struct {
	// Type is the cloud type.
	Type string `json:"type"`

	// KeyVaultResourceURL is the Key Vault resource URL.
	// +optional
	KeyVaultResourceURL string `json:"keyVaultResourceURL,omitempty"`

	// ActiveDirectoryEndpoint is the Active Directory endpoint.
	// +optional
	ActiveDirectoryEndpoint string `json:"activeDirectoryEndpoint,omitempty"`
}

// GCPSecretManager references a GCP Secret Manager secret.
type GCPSecretManager struct {
	// Secrets is the list of secrets to retrieve.
	Secrets []GCPSecretManagerSecret `json:"secrets"`

	// PodIdentity is the pod identity.
	// +optional
	PodIdentity *AuthPodIdentity `json:"podIdentity,omitempty"`

	// Credentials are the GCP credentials.
	// +optional
	Credentials *GCPSecretManagerCredentials `json:"credentials,omitempty"`
}

// GCPSecretManagerSecret represents a GCP Secret Manager secret.
type GCPSecretManagerSecret struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// ID is the ID of the secret.
	ID string `json:"id"`

	// Version is the version of the secret.
	// +optional
	Version string `json:"version,omitempty"`
}

// GCPSecretManagerCredentials represents GCP credentials.
type GCPSecretManagerCredentials struct {
	// ClientSecret is the client secret.
	ClientSecret AuthSecretTargetRef `json:"clientSecret"`
}

// AwsSecretManager references an AWS Secrets Manager secret.
type AwsSecretManager struct {
	// Secrets is the list of secrets to retrieve.
	Secrets []AwsSecretManagerSecret `json:"secrets"`

	// PodIdentity is the pod identity.
	// +optional
	PodIdentity *AuthPodIdentity `json:"podIdentity,omitempty"`

	// Credentials are the AWS credentials.
	// +optional
	Credentials *AwsSecretManagerCredentials `json:"credentials,omitempty"`

	// Region is the AWS region.
	// +optional
	Region string `json:"region,omitempty"`
}

// AwsSecretManagerSecret represents an AWS Secrets Manager secret.
type AwsSecretManagerSecret struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Name is the name of the secret.
	Name string `json:"name"`

	// VersionID is the version ID.
	// +optional
	VersionID string `json:"versionId,omitempty"`

	// VersionStage is the version stage.
	// +optional
	VersionStage string `json:"versionStage,omitempty"`
}

// AwsSecretManagerCredentials represents AWS credentials.
type AwsSecretManagerCredentials struct {
	// AccessKey is the access key.
	AccessKey AuthSecretTargetRef `json:"accessKey"`

	// AccessSecretKey is the secret access key.
	AccessSecretKey AuthSecretTargetRef `json:"accessSecretKey"`

	// AccessToken is the session token.
	// +optional
	AccessToken *AuthSecretTargetRef `json:"accessToken,omitempty"`
}

// AuthConfigMapTargetRef references a ConfigMap entry.
type AuthConfigMapTargetRef struct {
	// Parameter is the name of the parameter to set.
	Parameter string `json:"parameter"`

	// Name is the name of the ConfigMap.
	Name string `json:"name"`

	// Key is the key in the ConfigMap.
	Key string `json:"key"`
}

// +kubebuilder:object:root=true

// TriggerAuthenticationList contains a list of TriggerAuthentication.
type TriggerAuthenticationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TriggerAuthentication `json:"items"`
}
