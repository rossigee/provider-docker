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
	"k8s.io/apimachinery/pkg/util/intstr"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// A ContainerSpec defines the desired state of a Container.
type ContainerSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider ContainerParameters `json:"forProvider"`
}

// ContainerParameters are the configurable fields of a Container.
type ContainerParameters struct {
	// Image is the Docker image to run.
	// Examples: nginx:1.21, alpine:latest, ubuntu:20.04
	Image string `json:"image"`

	// Name is the container name. If not specified, a name will be generated.
	// +optional
	Name *string `json:"name,omitempty"`

	// Command overrides the default command specified by the image.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are the arguments to pass to the command.
	// +optional
	Args []string `json:"args,omitempty"`

	// Environment variables for the container.
	// +optional
	Environment []EnvVar `json:"environment,omitempty"`

	// Ports to expose from the container.
	// +optional
	Ports []PortSpec `json:"ports,omitempty"`

	// Volumes to mount in the container.
	// +optional
	Volumes []VolumeMount `json:"volumes,omitempty"`

	// NetworkMode sets the networking mode for the container.
	// Examples: bridge, host, none, container:<name>
	// +optional
	NetworkMode *string `json:"networkMode,omitempty"`

	// Networks to attach the container to.
	// +optional
	Networks []NetworkAttachment `json:"networks,omitempty"`

	// RestartPolicy defines the restart policy for the container.
	// +kubebuilder:validation:Enum=no;on-failure;always;unless-stopped
	// +optional
	RestartPolicy *string `json:"restartPolicy,omitempty"`

	// MaximumRetryCount is used with restart policy "on-failure".
	// +optional
	MaximumRetryCount *int `json:"maximumRetryCount,omitempty"`

	// WorkingDir sets the working directory for the container.
	// +optional
	WorkingDir *string `json:"workingDir,omitempty"`

	// User sets the user inside the container.
	// Can be a username, UID, or UID:GID format.
	// +optional
	User *string `json:"user,omitempty"`

	// Hostname sets the hostname of the container.
	// +optional
	Hostname *string `json:"hostname,omitempty"`

	// ExtraHosts adds entries to /etc/hosts.
	// +optional
	ExtraHosts []string `json:"extraHosts,omitempty"`

	// DNS configuration for the container.
	// +optional
	DNS []string `json:"dns,omitempty"`

	// DNSSearch sets the DNS search domains.
	// +optional
	DNSSearch []string `json:"dnsSearch,omitempty"`

	// DNSOptions sets DNS options.
	// +optional
	DNSOptions []string `json:"dnsOptions,omitempty"`

	// Labels to apply to the container.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Resources specify compute resource requirements.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext defines security attributes.
	// +optional
	SecurityContext *SecurityContext `json:"securityContext,omitempty"`

	// HealthCheck defines health checking configuration.
	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`

	// Init specifies if this is an init container.
	// +optional
	Init *bool `json:"init,omitempty"`

	// Privileged runs the container in privileged mode.
	// +optional
	Privileged *bool `json:"privileged,omitempty"`

	// Remove automatically removes the container when it exits.
	// +optional
	Remove *bool `json:"remove,omitempty"`

	// StartOnCreate starts the container after creating it.
	// Defaults to true.
	// +optional
	StartOnCreate *bool `json:"startOnCreate,omitempty"`
}

// EnvVar represents an environment variable.
type EnvVar struct {
	// Name of the environment variable.
	Name string `json:"name"`

	// Value of the environment variable.
	// +optional
	Value *string `json:"value,omitempty"`

	// ValueFrom defines a source for the environment variable value.
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource represents a source for the value of an EnvVar.
type EnvVarSource struct {
	// SecretKeyRef selects a key of a secret in the same namespace.
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`

	// ConfigMapKeyRef selects a key of a ConfigMap in the same namespace.
	// +optional
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
}

// SecretKeySelector selects a key from a Secret.
type SecretKeySelector struct {
	// Name of the secret.
	Name string `json:"name"`

	// Key to select from the secret.
	Key string `json:"key"`

	// Optional specifies whether the key must exist.
	// +optional
	Optional *bool `json:"optional,omitempty"`
}

// ConfigMapKeySelector selects a key from a ConfigMap.
type ConfigMapKeySelector struct {
	// Name of the ConfigMap.
	Name string `json:"name"`

	// Key to select from the ConfigMap.
	Key string `json:"key"`

	// Optional specifies whether the key must exist.
	// +optional
	Optional *bool `json:"optional,omitempty"`
}

// PortSpec defines a port to expose from the container.
type PortSpec struct {
	// ContainerPort is the port number on the container.
	ContainerPort int32 `json:"containerPort"`

	// HostPort is the port number on the host.
	// If not specified, the container port is not exposed on the host.
	// +optional
	HostPort *int32 `json:"hostPort,omitempty"`

	// HostIP binds the port to a specific host interface.
	// Defaults to all interfaces (0.0.0.0).
	// +optional
	HostIP *string `json:"hostIP,omitempty"`

	// Protocol for the port. Must be UDP, TCP, or SCTP.
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	// +kubebuilder:default=TCP
	// +optional
	Protocol *string `json:"protocol,omitempty"`
}

// VolumeMount describes a mounting of a Volume within a container.
type VolumeMount struct {
	// Name must match the name of a volume in the VolumeSource.
	Name string `json:"name"`

	// MountPath within the container at which the volume should be mounted.
	MountPath string `json:"mountPath"`

	// ReadOnly mounts the volume as read-only.
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`

	// VolumeSource represents the location and type of the mounted volume.
	VolumeSource VolumeSource `json:"source"`
}

// VolumeSource represents a volume source.
type VolumeSource struct {
	// HostPath represents a host path mapped into the container.
	// +optional
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`

	// EmptyDir represents a temporary directory that shares a container's lifetime.
	// +optional
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// Secret represents a secret that should be mounted.
	// +optional
	Secret *SecretVolumeSource `json:"secret,omitempty"`

	// ConfigMap represents a configMap that should be mounted.
	// +optional
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty"`

	// Volume represents a Docker volume.
	// +optional
	Volume *VolumeVolumeSource `json:"volume,omitempty"`

	// Bind represents a bind mount.
	// +optional
	Bind *BindVolumeSource `json:"bind,omitempty"`
}

// HostPathVolumeSource represents a host path mapped into the container.
type HostPathVolumeSource struct {
	// Path of the directory on the host.
	Path string `json:"path"`

	// Type for HostPath Volume.
	// +optional
	Type *HostPathType `json:"type,omitempty"`
}

// HostPathType represents the type of the HostPath.
type HostPathType string

const (
	// For backwards compatibility, this implies the default behavior.
	HostPathUnset HostPathType = ""
	// If nothing exists at the given path, an empty directory will be created.
	HostPathDirectoryOrCreate HostPathType = "DirectoryOrCreate"
	// A directory must exist at the given path.
	HostPathDirectory HostPathType = "Directory"
	// If nothing exists at the given path, an empty file will be created.
	HostPathFileOrCreate HostPathType = "FileOrCreate"
	// A file must exist at the given path.
	HostPathFile HostPathType = "File"
	// A UNIX socket must exist at the given path.
	HostPathSocket HostPathType = "Socket"
	// A character device must exist at the given path.
	HostPathCharDevice HostPathType = "CharDevice"
	// A block device must exist at the given path.
	HostPathBlockDevice HostPathType = "BlockDevice"
)

// HostPathTypePtr returns a pointer to the given HostPathType.
func HostPathTypePtr(t HostPathType) *HostPathType {
	return &t
}

// EmptyDirVolumeSource represents a temporary directory that shares a container's lifetime.
type EmptyDirVolumeSource struct {
	// SizeLimit for the EmptyDir volume.
	// +optional
	SizeLimit *string `json:"sizeLimit,omitempty"`
}

// SecretVolumeSource adapts a Secret into a volume.
type SecretVolumeSource struct {
	// Name of the secret.
	SecretName string `json:"secretName"`

	// Optional specifies whether the Secret must exist.
	// +optional
	Optional *bool `json:"optional,omitempty"`

	// DefaultMode for created files.
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty"`

	// Items to project from the secret.
	// +optional
	Items []KeyToPath `json:"items,omitempty"`
}

// ConfigMapVolumeSource adapts a ConfigMap into a volume.
type ConfigMapVolumeSource struct {
	// Name of the ConfigMap.
	Name string `json:"name"`

	// Optional specifies whether the ConfigMap must exist.
	// +optional
	Optional *bool `json:"optional,omitempty"`

	// DefaultMode for created files.
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty"`

	// Items to project from the ConfigMap.
	// +optional
	Items []KeyToPath `json:"items,omitempty"`
}

// KeyToPath maps a data key to a path.
type KeyToPath struct {
	// Key is the key to project.
	Key string `json:"key"`

	// Path is the relative path of the file to map the key to.
	Path string `json:"path"`

	// Mode is the file mode.
	// +optional
	Mode *int32 `json:"mode,omitempty"`
}

// VolumeVolumeSource represents a Docker volume.
type VolumeVolumeSource struct {
	// VolumeName is the name of the Docker volume.
	VolumeName string `json:"volumeName"`
}

// BindVolumeSource represents a bind mount.
type BindVolumeSource struct {
	// SourcePath on the host.
	SourcePath string `json:"sourcePath"`

	// Propagation mode for the mount.
	// +kubebuilder:validation:Enum=private;rprivate;shared;rshared;slave;rslave
	// +optional
	Propagation *string `json:"propagation,omitempty"`
}

// NetworkAttachment describes how to attach the container to a network.
type NetworkAttachment struct {
	// Name of the network.
	Name string `json:"name"`

	// Aliases for the container on this network.
	// +optional
	Aliases []string `json:"aliases,omitempty"`

	// IPAddress to assign to the container on this network.
	// +optional
	IPAddress *string `json:"ipAddress,omitempty"`

	// IPv6Address to assign to the container on this network.
	// +optional
	IPv6Address *string `json:"ipv6Address,omitempty"`

	// Links to other containers (legacy).
	// +optional
	Links []string `json:"links,omitempty"`
}

// ResourceRequirements describes compute resource requirements.
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required.
	// +optional
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[string]intstr.IntOrString

// SecurityContext holds security configuration.
type SecurityContext struct {
	// RunAsUser is the UID to run the container as.
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`

	// RunAsGroup is the GID to run the container as.
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`

	// RunAsNonRoot indicates whether the container must be run as a non-root user.
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`

	// ReadOnlyRootFilesystem mounts the container's root filesystem as read-only.
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`

	// AllowPrivilegeEscalation controls whether a process can gain more privileges.
	// +optional
	AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation,omitempty"`

	// Capabilities to add/drop.
	// +optional
	Capabilities *Capabilities `json:"capabilities,omitempty"`

	// SELinuxOptions are the SELinux options to apply.
	// +optional
	SELinuxOptions *SELinuxOptions `json:"seLinuxOptions,omitempty"`

	// SeccompProfile is the seccomp profile to apply.
	// +optional
	SeccompProfile *SeccompProfile `json:"seccompProfile,omitempty"`

	// AppArmorProfile is the AppArmor profile to apply.
	// +optional
	AppArmorProfile *AppArmorProfile `json:"appArmorProfile,omitempty"`
}

// Capabilities represent POSIX capabilities.
type Capabilities struct {
	// Added capabilities
	// +optional
	Add []string `json:"add,omitempty"`

	// Removed capabilities
	// +optional
	Drop []string `json:"drop,omitempty"`
}

// SELinuxOptions are the labels to be applied to the container.
type SELinuxOptions struct {
	// User is a SELinux user label.
	// +optional
	User *string `json:"user,omitempty"`

	// Role is a SELinux role label.
	// +optional
	Role *string `json:"role,omitempty"`

	// Type is a SELinux type label.
	// +optional
	Type *string `json:"type,omitempty"`

	// Level is a SELinux level label.
	// +optional
	Level *string `json:"level,omitempty"`
}

// SeccompProfile defines a pod/container's seccomp profile settings.
type SeccompProfile struct {
	// Type indicates which kind of seccomp profile will be applied.
	// +kubebuilder:validation:Enum=RuntimeDefault;Unconfined;Localhost
	Type string `json:"type"`

	// LocalhostProfile indicates a profile defined in a file on the node.
	// Only used when Type is "Localhost".
	// +optional
	LocalhostProfile *string `json:"localhostProfile,omitempty"`
}

// AppArmorProfile defines a pod or container's AppArmor settings.
type AppArmorProfile struct {
	// Type indicates which kind of AppArmor profile will be applied.
	// +kubebuilder:validation:Enum=RuntimeDefault;Unconfined;Localhost
	Type string `json:"type"`

	// LocalhostProfile indicates a profile defined in a file on the node.
	// Only used when Type is "Localhost".
	// +optional
	LocalhostProfile *string `json:"localhostProfile,omitempty"`
}

// HealthCheck defines health checking configuration.
type HealthCheck struct {
	// Test is the command to run to check health.
	// Examples: ["CMD", "curl", "-f", "http://localhost/health"]
	Test []string `json:"test"`

	// Interval between health checks (default 30s).
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Timeout for each health check (default 30s).
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// StartPeriod for the container to initialize (default 0s).
	// +optional
	StartPeriod *metav1.Duration `json:"startPeriod,omitempty"`

	// Retries is the number of consecutive failures needed to consider unhealthy.
	// +optional
	Retries *int `json:"retries,omitempty"`
}

// A ContainerStatus represents the observed state of a Container.
type ContainerStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains the observed state of the Container.
	AtProvider ContainerObservation `json:"atProvider,omitempty"`
}

// ContainerObservation are the observable fields of a Container.
type ContainerObservation struct {
	// ID is the container ID assigned by Docker.
	ID string `json:"id,omitempty"`

	// Name is the actual container name.
	Name string `json:"name,omitempty"`

	// State is the container state.
	State ContainerState `json:"state,omitempty"`

	// Image is the resolved image name and ID.
	Image ContainerImage `json:"image,omitempty"`

	// Created is when the container was created.
	Created *metav1.Time `json:"created,omitempty"`

	// Started is when the container was started.
	Started *metav1.Time `json:"started,omitempty"`

	// Ports shows the actual port mappings.
	Ports []ContainerPort `json:"ports,omitempty"`

	// Networks shows the networks the container is attached to.
	Networks map[string]NetworkInfo `json:"networks,omitempty"`
}

// ContainerState represents the state of a container.
type ContainerState struct {
	// Status is the container status (created, running, paused, restarting, removing, exited, dead).
	Status string `json:"status,omitempty"`

	// Running indicates if the container is running.
	Running bool `json:"running,omitempty"`

	// Paused indicates if the container is paused.
	Paused bool `json:"paused,omitempty"`

	// Restarting indicates if the container is restarting.
	Restarting bool `json:"restarting,omitempty"`

	// OOMKilled indicates if the container was killed due to OOM.
	OOMKilled bool `json:"oomKilled,omitempty"`

	// Dead indicates if the container is dead.
	Dead bool `json:"dead,omitempty"`

	// Pid is the process ID of the container's main process.
	Pid int64 `json:"pid,omitempty"`

	// ExitCode is the exit code of the container.
	ExitCode int64 `json:"exitCode,omitempty"`

	// Error is any error from the container.
	Error string `json:"error,omitempty"`

	// StartedAt is when the container started.
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the container finished.
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// Health is the health check result.
	Health *ContainerHealth `json:"health,omitempty"`
}

// ContainerImage contains image information.
type ContainerImage struct {
	// Name is the image name used.
	Name string `json:"name,omitempty"`

	// ID is the image ID.
	ID string `json:"id,omitempty"`

	// Digest is the image digest.
	Digest string `json:"digest,omitempty"`
}

// ContainerPort shows an exposed port.
type ContainerPort struct {
	// IP is the host IP the port is bound to.
	IP string `json:"ip,omitempty"`

	// PrivatePort is the port on the container.
	PrivatePort int32 `json:"privatePort,omitempty"`

	// PublicPort is the port on the host.
	PublicPort int32 `json:"publicPort,omitempty"`

	// Type is the port type (tcp/udp).
	Type string `json:"type,omitempty"`
}

// NetworkInfo contains network attachment information.
type NetworkInfo struct {
	// NetworkID is the network ID.
	NetworkID string `json:"networkID,omitempty"`

	// EndpointID is the endpoint ID.
	EndpointID string `json:"endpointID,omitempty"`

	// Gateway is the network gateway.
	Gateway string `json:"gateway,omitempty"`

	// IPAddress is the container's IP address on this network.
	IPAddress string `json:"ipAddress,omitempty"`

	// IPPrefixLen is the IP prefix length.
	IPPrefixLen int32 `json:"ipPrefixLen,omitempty"`

	// IPv6Gateway is the IPv6 gateway.
	IPv6Gateway string `json:"ipv6Gateway,omitempty"`

	// GlobalIPv6Address is the global IPv6 address.
	GlobalIPv6Address string `json:"globalIPv6Address,omitempty"`

	// GlobalIPv6PrefixLen is the global IPv6 prefix length.
	GlobalIPv6PrefixLen int32 `json:"globalIPv6PrefixLen,omitempty"`

	// MacAddress is the MAC address.
	MacAddress string `json:"macAddress,omitempty"`
}

// ContainerHealth represents health check status.
type ContainerHealth struct {
	// Status is the health status (starting, healthy, unhealthy).
	Status string `json:"status,omitempty"`

	// FailingStreak is the number of consecutive failed health checks.
	FailingStreak int `json:"failingStreak,omitempty"`

	// Log contains recent health check results.
	Log []HealthCheckResult `json:"log,omitempty"`
}

// HealthCheckResult represents a single health check result.
type HealthCheckResult struct {
	// Start is when the health check started.
	Start *metav1.Time `json:"start,omitempty"`

	// End is when the health check ended.
	End *metav1.Time `json:"end,omitempty"`

	// ExitCode is the exit code of the health check.
	ExitCode int `json:"exitCode,omitempty"`

	// Output is the health check output.
	Output string `json:"output,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Container is a managed resource that represents a Docker container.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="IMAGE",type="string",JSONPath=".spec.forProvider.image",priority=1
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state.status",priority=1
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,docker}
type Container struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerSpec   `json:"spec"`
	Status ContainerStatus `json:"status,omitempty"`
}

// GetCondition returns the condition for the given ConditionType.
func (cr *Container) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cr.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource.
func (cr *Container) SetConditions(c ...xpv1.Condition) {
	cr.Status.SetConditions(c...)
}

// GetDeletionPolicy returns the deletion policy of the resource.
func (cr *Container) GetDeletionPolicy() xpv1.DeletionPolicy {
	return cr.Spec.DeletionPolicy
}

// SetDeletionPolicy sets the deletion policy of the resource.
func (cr *Container) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	cr.Spec.DeletionPolicy = p
}

// GetManagementPolicies returns the management policies of the resource.
func (cr *Container) GetManagementPolicies() xpv1.ManagementPolicies {
	return cr.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies of the resource.
func (cr *Container) SetManagementPolicies(p xpv1.ManagementPolicies) {
	cr.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (cr *Container) GetProviderConfigReference() *xpv1.Reference {
	return cr.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (cr *Container) SetProviderConfigReference(r *xpv1.Reference) {
	cr.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference returns the write connection secret to reference.
func (cr *Container) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return cr.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret to reference.
func (cr *Container) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	cr.Spec.WriteConnectionSecretToReference = r
}

// +kubebuilder:object:root=true

// ContainerList contains a list of Container.
type ContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Container `json:"items"`
}
