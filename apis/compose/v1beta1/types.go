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
	v1alpha1 "github.com/rossigee/provider-docker/apis/compose/v1alpha1"
)

// A ComposeStackSpec defines the desired state of a ComposeStack.
type ComposeStackSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider ComposeStackParameters `json:"forProvider"`
}

// ComposeStackParameters are the configurable fields of a ComposeStack.
// This type reuses the v1alpha1 definition for consistency while maintaining
// the namespaced v1beta1 API surface.
type ComposeStackParameters v1alpha1.ComposeStackParameters

// A ComposeStackStatus represents the observed state of a ComposeStack.
type ComposeStackStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains the observed state of the ComposeStack.
	AtProvider ComposeStackObservation `json:"atProvider,omitempty"`
}

// ComposeStackObservation are the observable fields of a ComposeStack.
// This type reuses the v1alpha1 definition for consistency.
type ComposeStackObservation v1alpha1.ComposeStackObservation

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A ComposeStack is a managed resource that represents a Docker Compose stack.
// This is the namespaced v1beta1 version following Crossplane v2 patterns.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="PROJECT",type="string",JSONPath=".spec.forProvider.projectName",priority=1
// +kubebuilder:printcolumn:name="SERVICES",type="string",JSONPath=".status.atProvider.services",priority=1
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,docker,compose,v2}
type ComposeStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComposeStackSpec   `json:"spec"`
	Status ComposeStackStatus `json:"status,omitempty"`
}

// GetCondition returns the condition for the given ConditionType.
func (cr *ComposeStack) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cr.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource.
func (cr *ComposeStack) SetConditions(c ...xpv1.Condition) {
	cr.Status.SetConditions(c...)
}

// GetDeletionPolicy returns the deletion policy of the resource.
func (cr *ComposeStack) GetDeletionPolicy() xpv1.DeletionPolicy {
	return cr.Spec.DeletionPolicy
}

// SetDeletionPolicy sets the deletion policy of the resource.
func (cr *ComposeStack) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	cr.Spec.DeletionPolicy = p
}

// GetManagementPolicies returns the management policies of the resource.
func (cr *ComposeStack) GetManagementPolicies() xpv1.ManagementPolicies {
	return cr.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies of the resource.
func (cr *ComposeStack) SetManagementPolicies(p xpv1.ManagementPolicies) {
	cr.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (cr *ComposeStack) GetProviderConfigReference() *xpv1.Reference {
	return cr.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (cr *ComposeStack) SetProviderConfigReference(r *xpv1.Reference) {
	cr.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference returns the write connection secret to reference.
func (cr *ComposeStack) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return cr.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret to reference.
func (cr *ComposeStack) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	cr.Spec.WriteConnectionSecretToReference = r
}

// +kubebuilder:object:root=true

// ComposeStackList contains a list of ComposeStack.
type ComposeStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComposeStack `json:"items"`
}
