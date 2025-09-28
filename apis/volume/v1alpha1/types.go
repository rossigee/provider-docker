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

// A VolumeSpec defines the desired state of a Volume.
type VolumeSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider VolumeParameters `json:"forProvider"`
}

// VolumeParameters are the configurable fields of a Volume.
type VolumeParameters struct {
	// Name is the volume name. If not specified, the resource name will be used.
	// +optional
	Name *string `json:"name,omitempty"`

	// Driver specifies the volume driver to use.
	// Common drivers: local, nfs, tmpfs, overlay2
	// +kubebuilder:default="local"
	// +optional
	Driver *string `json:"driver,omitempty"`

	// DriverOpts is a map of driver-specific options.
	// For local driver: type, o, device
	// For nfs driver: addr, path
	// +optional
	DriverOpts map[string]string `json:"driverOpts,omitempty"`

	// Labels is a map of labels to apply to the volume.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// A VolumeStatus represents the observed state of a Volume.
type VolumeStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains the observed state of the Volume.
	AtProvider VolumeObservation `json:"atProvider,omitempty"`
}

// VolumeObservation are the observable fields of a Volume.
type VolumeObservation struct {
	// Name is the actual volume name.
	Name string `json:"name,omitempty"`

	// Driver is the volume driver being used.
	Driver string `json:"driver,omitempty"`

	// Mountpoint is the path where the volume is mounted on the host.
	Mountpoint string `json:"mountpoint,omitempty"`

	// CreatedAt is the timestamp when the volume was created.
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// Scope indicates the scope of the volume (local or global).
	Scope string `json:"scope,omitempty"`

	// Options are the driver-specific options for the volume.
	Options map[string]string `json:"options,omitempty"`

	// Labels are the labels applied to the volume.
	Labels map[string]string `json:"labels,omitempty"`

	// UsageData contains information about volume usage.
	UsageData *VolumeUsageData `json:"usageData,omitempty"`
}

// VolumeUsageData contains information about volume usage.
type VolumeUsageData struct {
	// Size is the amount of space used by the volume (in bytes).
	Size int64 `json:"size,omitempty"`

	// RefCount is the number of containers using this volume.
	RefCount int64 `json:"refCount,omitempty"`
}

// +kubebuilder:object:root=true

// A Volume is a managed resource that represents a Docker volume.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="DRIVER",type="string",JSONPath=".status.atProvider.driver",priority=1
// +kubebuilder:printcolumn:name="MOUNTPOINT",type="string",JSONPath=".status.atProvider.mountpoint",priority=1
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,docker}
type Volume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSpec   `json:"spec"`
	Status VolumeStatus `json:"status,omitempty"`
}

// GetCondition returns the condition for the given ConditionType.
func (cr *Volume) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cr.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource.
func (cr *Volume) SetConditions(c ...xpv1.Condition) {
	cr.Status.SetConditions(c...)
}

// GetDeletionPolicy returns the deletion policy of the resource.
func (cr *Volume) GetDeletionPolicy() xpv1.DeletionPolicy {
	return cr.Spec.DeletionPolicy
}

// SetDeletionPolicy sets the deletion policy of the resource.
func (cr *Volume) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	cr.Spec.DeletionPolicy = p
}

// GetManagementPolicies returns the management policies of the resource.
func (cr *Volume) GetManagementPolicies() xpv1.ManagementPolicies {
	return cr.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies of the resource.
func (cr *Volume) SetManagementPolicies(p xpv1.ManagementPolicies) {
	cr.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (cr *Volume) GetProviderConfigReference() *xpv1.Reference {
	return cr.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (cr *Volume) SetProviderConfigReference(r *xpv1.Reference) {
	cr.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference returns the write connection secret to reference.
func (cr *Volume) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return cr.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret to reference.
func (cr *Volume) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	cr.Spec.WriteConnectionSecretToReference = r
}

// +kubebuilder:object:root=true

// VolumeList contains a list of Volume.
type VolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Volume `json:"items"`
}
