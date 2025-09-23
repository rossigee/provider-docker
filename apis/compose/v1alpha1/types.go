/*
Copyright 2025 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// A ComposeStackSpec defines the desired state of a ComposeStack.
type ComposeStackSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider ComposeStackParameters `json:"forProvider"`
}

// ComposeStackParameters are the configurable fields of a ComposeStack.
type ComposeStackParameters struct {
	// Compose contains the Docker Compose file content inline.
	// This is mutually exclusive with ComposeRef.
	// +optional
	Compose *string `json:"compose,omitempty"`

	// ComposeRef references a ConfigMap or Secret containing the compose file.
	// This is mutually exclusive with Compose.
	// +optional
	ComposeRef *ComposeReference `json:"composeRef,omitempty"`

	// ProjectName sets the project name for the compose stack.
	// This is equivalent to docker-compose -p flag.
	// If not specified, the resource name will be used.
	// +optional
	ProjectName *string `json:"projectName,omitempty"`

	// Environment variables to inject into all services.
	// These variables are available for interpolation in the compose file.
	// +optional
	Environment []ComposeEnvVar `json:"environment,omitempty"`

	// ServiceOverrides allow overriding specific service configurations.
	// +optional
	ServiceOverrides map[string]ServiceOverride `json:"serviceOverrides,omitempty"`

	// WorkingDir sets the working directory for compose file resolution.
	// This affects relative paths in the compose file.
	// +optional
	WorkingDir *string `json:"workingDir,omitempty"`

	// EnvFiles specifies additional .env files to load.
	// These can reference ConfigMaps containing environment definitions.
	// +optional
	EnvFiles []ComposeReference `json:"envFiles,omitempty"`
}

// ComposeReference references a ConfigMap or Secret containing compose-related data.
type ComposeReference struct {
	// ConfigMapRef references a ConfigMap.
	// This is mutually exclusive with SecretRef.
	// +optional
	ConfigMapRef *ConfigMapReference `json:"configMapRef,omitempty"`

	// SecretRef references a Secret.
	// This is mutually exclusive with ConfigMapRef.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// ConfigMapReference references a specific key in a ConfigMap.
type ConfigMapReference struct {
	// Name of the ConfigMap.
	Name string `json:"name"`

	// Namespace of the ConfigMap. If not specified, uses the same namespace as the ComposeStack.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Key within the ConfigMap to read from.
	Key string `json:"key"`
}

// SecretReference references a specific key in a Secret.
type SecretReference struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Namespace of the Secret. If not specified, uses the same namespace as the ComposeStack.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Key within the Secret to read from.
	Key string `json:"key"`
}

// ComposeEnvVar represents an environment variable for the compose stack.
type ComposeEnvVar struct {
	// Name of the environment variable.
	Name string `json:"name"`

	// Value is the literal value of the environment variable.
	// This is mutually exclusive with ValueFrom.
	// +optional
	Value *string `json:"value,omitempty"`

	// ValueFrom specifies a source for the environment variable value.
	// This is mutually exclusive with Value.
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource represents a source for an environment variable value.
type EnvVarSource struct {
	// SecretKeyRef selects a key from a Secret.
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`

	// ConfigMapKeyRef selects a key from a ConfigMap.
	// +optional
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
}

// SecretKeySelector selects a key from a Secret.
type SecretKeySelector struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Namespace of the Secret. If not specified, uses the same namespace as the ComposeStack.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Key within the Secret to select.
	Key string `json:"key"`
}

// ConfigMapKeySelector selects a key from a ConfigMap.
type ConfigMapKeySelector struct {
	// Name of the ConfigMap.
	Name string `json:"name"`

	// Namespace of the ConfigMap. If not specified, uses the same namespace as the ComposeStack.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Key within the ConfigMap to select.
	Key string `json:"key"`
}

// ServiceOverride allows overriding specific service configurations.
type ServiceOverride struct {
	// Replicas overrides the number of replicas for this service.
	// Note: This is a Crossplane-specific extension to Docker Compose.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources overrides resource requirements for this service.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Environment adds or overrides environment variables for this service.
	// +optional
	Environment []ComposeEnvVar `json:"environment,omitempty"`

	// Labels adds or overrides labels for this service.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// RestartPolicy overrides the restart policy for this service.
	// +kubebuilder:validation:Enum=no;on-failure;always;unless-stopped
	// +optional
	RestartPolicy *string `json:"restartPolicy,omitempty"`
}

// ResourceRequirements describes resource requirements for a service.
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits map[string]string `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required.
	// +optional
	Requests map[string]string `json:"requests,omitempty"`
}

// ComposeStackStatus represents the observed state of a ComposeStack.
type ComposeStackStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains provider-specific status information.
	AtProvider ComposeStackObservation `json:"atProvider,omitempty"`
}

// ComposeStackObservation contains the observed state of the ComposeStack.
type ComposeStackObservation struct {
	// ProjectName is the actual project name used for the compose stack.
	ProjectName string `json:"projectName,omitempty"`

	// Services contains the status of individual services in the stack.
	Services map[string]ServiceStatus `json:"services,omitempty"`

	// Networks contains the status of networks created by the stack.
	Networks []NetworkStatus `json:"networks,omitempty"`

	// Volumes contains the status of volumes created by the stack.
	Volumes []VolumeStatus `json:"volumes,omitempty"`

	// ParsedAt indicates when the compose file was last successfully parsed.
	// +optional
	ParsedAt *metav1.Time `json:"parsedAt,omitempty"`

	// ComposeVersion indicates the detected compose file format version.
	// +optional
	ComposeVersion *string `json:"composeVersion,omitempty"`
}

// ServiceStatus represents the status of a service within the compose stack.
type ServiceStatus struct {
	// Name of the service as defined in the compose file.
	Name string `json:"name"`

	// ContainerID is the ID of the running container for this service.
	// +optional
	ContainerID *string `json:"containerID,omitempty"`

	// State indicates the current state of the service.
	// +kubebuilder:validation:Enum=pending;creating;running;restarting;exited;paused;dead;unknown
	State string `json:"state"`

	// Image is the resolved image name and tag for this service.
	// +optional
	Image *string `json:"image,omitempty"`

	// Ports contains the exposed ports for this service.
	// +optional
	Ports []PortMapping `json:"ports,omitempty"`

	// Health indicates the health check status if configured.
	// +optional
	Health *HealthStatus `json:"health,omitempty"`

	// CreatedAt indicates when the service was created.
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// StartedAt indicates when the service was started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
}

// NetworkStatus represents the status of a network created by the compose stack.
type NetworkStatus struct {
	// Name of the network.
	Name string `json:"name"`

	// ID is the Docker network ID.
	// +optional
	ID *string `json:"id,omitempty"`

	// Driver is the network driver type.
	// +optional
	Driver *string `json:"driver,omitempty"`

	// CreatedAt indicates when the network was created.
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// VolumeStatus represents the status of a volume created by the compose stack.
type VolumeStatus struct {
	// Name of the volume.
	Name string `json:"name"`

	// ID is the Docker volume ID.
	// +optional
	ID *string `json:"id,omitempty"`

	// Driver is the volume driver type.
	// +optional
	Driver *string `json:"driver,omitempty"`

	// Mountpoint is the volume mount point on the host.
	// +optional
	Mountpoint *string `json:"mountpoint,omitempty"`

	// CreatedAt indicates when the volume was created.
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// PortMapping represents a port mapping for a service.
type PortMapping struct {
	// ContainerPort is the port inside the container.
	ContainerPort int32 `json:"containerPort"`

	// HostPort is the port on the host machine.
	// +optional
	HostPort *int32 `json:"hostPort,omitempty"`

	// HostIP is the host IP to bind to.
	// +optional
	HostIP *string `json:"hostIP,omitempty"`

	// Protocol is the port protocol.
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	// +optional
	Protocol *string `json:"protocol,omitempty"`
}

// HealthStatus represents the health check status of a service.
type HealthStatus struct {
	// Status is the current health status.
	// +kubebuilder:validation:Enum=starting;healthy;unhealthy;none
	Status string `json:"status"`

	// FailingStreak is the number of consecutive failed health checks.
	// +optional
	FailingStreak *int32 `json:"failingStreak,omitempty"`

	// Log contains recent health check results.
	// +optional
	Log []HealthCheckResult `json:"log,omitempty"`
}

// HealthCheckResult represents a single health check result.
type HealthCheckResult struct {
	// ExitCode is the exit code of the health check.
	ExitCode int32 `json:"exitCode"`

	// Output is the output from the health check command.
	// +optional
	Output *string `json:"output,omitempty"`

	// Start is when the health check started.
	// +optional
	Start *metav1.Time `json:"start,omitempty"`

	// End is when the health check completed.
	// +optional
	End *metav1.Time `json:"end,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,docker}
// +kubebuilder:printcolumn:name="PROJECT",type="string",JSONPath=".status.atProvider.projectName"
// +kubebuilder:printcolumn:name="SERVICES",type="integer",JSONPath=".status.atProvider.services.length"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ComposeStack is a managed resource that represents a Docker Compose stack.
type ComposeStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComposeStackSpec   `json:"spec"`
	Status ComposeStackStatus `json:"status,omitempty"`
}

// GetCondition returns the condition for the given ConditionType.
func (cs *ComposeStack) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cs.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource.
func (cs *ComposeStack) SetConditions(c ...xpv1.Condition) {
	cs.Status.SetConditions(c...)
}

// GetDeletionPolicy returns the deletion policy of the resource.
func (cs *ComposeStack) GetDeletionPolicy() xpv1.DeletionPolicy {
	return cs.Spec.DeletionPolicy
}

// SetDeletionPolicy sets the deletion policy of the resource.
func (cs *ComposeStack) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	cs.Spec.DeletionPolicy = p
}

// GetManagementPolicies returns the management policies of the resource.
func (cs *ComposeStack) GetManagementPolicies() xpv1.ManagementPolicies {
	return cs.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies of the resource.
func (cs *ComposeStack) SetManagementPolicies(p xpv1.ManagementPolicies) {
	cs.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (cs *ComposeStack) GetProviderConfigReference() *xpv1.Reference {
	return cs.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (cs *ComposeStack) SetProviderConfigReference(r *xpv1.Reference) {
	cs.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference returns the write connection secret to reference.
func (cs *ComposeStack) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return cs.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret to reference.
func (cs *ComposeStack) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	cs.Spec.WriteConnectionSecretToReference = r
}

// +kubebuilder:object:root=true

// ComposeStackList contains a list of ComposeStack.
type ComposeStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComposeStack `json:"items"`
}
