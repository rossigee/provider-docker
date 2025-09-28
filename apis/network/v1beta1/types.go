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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	// Import v1alpha1 types for reuse
	v1alpha1 "github.com/rossigee/provider-docker/apis/network/v1alpha1"
)

// A NetworkSpec defines the desired state of a Network.
type NetworkSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider NetworkParameters `json:"forProvider"`
}

// NetworkParameters are the configurable fields of a Network.
// This type reuses the v1alpha1 definition for consistency while maintaining
// the namespaced v1beta1 API surface.
type NetworkParameters v1alpha1.NetworkParameters

// A NetworkStatus represents the observed state of a Network.
type NetworkStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains the observed state of the Network.
	AtProvider NetworkObservation `json:"atProvider,omitempty"`
}

// NetworkObservation are the observable fields of a Network.
// This type reuses the v1alpha1 definition for consistency.
type NetworkObservation v1alpha1.NetworkObservation

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Network is a managed resource that represents a Docker network.
// This is the namespaced v1beta1 version following Crossplane v2 patterns.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="DRIVER",type="string",JSONPath=".status.atProvider.driver",priority=1
// +kubebuilder:printcolumn:name="SCOPE",type="string",JSONPath=".status.atProvider.scope",priority=1
// +kubebuilder:printcolumn:name="INTERNAL",type="boolean",JSONPath=".status.atProvider.internal",priority=1
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,docker,v2}
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// GetCondition returns the condition for the given ConditionType.
func (cr *Network) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cr.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource.
func (cr *Network) SetConditions(c ...xpv1.Condition) {
	cr.Status.SetConditions(c...)
}

// GetDeletionPolicy returns the deletion policy of the resource.
func (cr *Network) GetDeletionPolicy() xpv1.DeletionPolicy {
	return cr.Spec.DeletionPolicy
}

// SetDeletionPolicy sets the deletion policy of the resource.
func (cr *Network) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	cr.Spec.DeletionPolicy = p
}

// GetManagementPolicies returns the management policies of the resource.
func (cr *Network) GetManagementPolicies() xpv1.ManagementPolicies {
	return cr.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies of the resource.
func (cr *Network) SetManagementPolicies(p xpv1.ManagementPolicies) {
	cr.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (cr *Network) GetProviderConfigReference() *xpv1.Reference {
	return cr.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (cr *Network) SetProviderConfigReference(r *xpv1.Reference) {
	cr.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference returns the write connection secret to reference.
func (cr *Network) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return cr.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret to reference.
func (cr *Network) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	cr.Spec.WriteConnectionSecretToReference = r
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}
