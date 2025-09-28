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

// A NetworkSpec defines the desired state of a Network.
type NetworkSpec struct {
	xpv1.ResourceSpec `json:",inline"`

	// ForProvider contains the provider-specific configuration.
	ForProvider NetworkParameters `json:"forProvider"`
}

// NetworkParameters are the configurable fields of a Network.
type NetworkParameters struct {
	// Name is the network name. If not specified, the resource name will be used.
	// +optional
	Name *string `json:"name,omitempty"`

	// Driver specifies the network driver to use.
	// Common drivers: bridge, overlay, host, none, macvlan
	// +kubebuilder:default="bridge"
	// +optional
	Driver *string `json:"driver,omitempty"`

	// Internal restricts external access to the network.
	// +optional
	Internal *bool `json:"internal,omitempty"`

	// Attachable enables manual container attachment to the network.
	// +optional
	Attachable *bool `json:"attachable,omitempty"`

	// Ingress designates the network as providing the routing-mesh.
	// +optional
	Ingress *bool `json:"ingress,omitempty"`

	// EnableIPv6 enables IPv6 networking.
	// +optional
	EnableIPv6 *bool `json:"enableIPv6,omitempty"`

	// IPAM contains IP Address Management configuration.
	// +optional
	IPAM *IPAMConfig `json:"ipam,omitempty"`

	// Options is a map of driver-specific options.
	// +optional
	Options map[string]string `json:"options,omitempty"`

	// Labels is a map of labels to apply to the network.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// IPAMConfig contains IP Address Management configuration.
type IPAMConfig struct {
	// Driver specifies the IPAM driver to use.
	// +kubebuilder:default="default"
	// +optional
	Driver *string `json:"driver,omitempty"`

	// Config contains IPAM configuration.
	// +optional
	Config []IPAMConfigEntry `json:"config,omitempty"`

	// Options is a map of driver-specific IPAM options.
	// +optional
	Options map[string]string `json:"options,omitempty"`
}

// IPAMConfigEntry represents a single IPAM configuration entry.
type IPAMConfigEntry struct {
	// Subnet is the subnet in CIDR format that represents a network segment.
	// +optional
	Subnet *string `json:"subnet,omitempty"`

	// IPRange limits allocation to a subset of the subnet.
	// +optional
	IPRange *string `json:"ipRange,omitempty"`

	// Gateway is the gateway for the master subnet.
	// +optional
	Gateway *string `json:"gateway,omitempty"`

	// AuxAddresses are auxiliary IPv4 or IPv6 addresses used by the network driver.
	// +optional
	AuxAddresses map[string]string `json:"auxAddresses,omitempty"`
}

// A NetworkStatus represents the observed state of a Network.
type NetworkStatus struct {
	xpv1.ResourceStatus `json:",inline"`

	// AtProvider contains the observed state of the Network.
	AtProvider NetworkObservation `json:"atProvider,omitempty"`
}

// NetworkObservation are the observable fields of a Network.
type NetworkObservation struct {
	// ID is the unique identifier of the network.
	ID string `json:"id,omitempty"`

	// Name is the actual network name.
	Name string `json:"name,omitempty"`

	// Driver is the network driver being used.
	Driver string `json:"driver,omitempty"`

	// Scope indicates whether the network is local or global.
	Scope string `json:"scope,omitempty"`

	// Internal indicates if the network is internal-only.
	Internal bool `json:"internal,omitempty"`

	// Attachable indicates if the network allows manual attachment.
	Attachable bool `json:"attachable,omitempty"`

	// Ingress indicates if this is an ingress network.
	Ingress bool `json:"ingress,omitempty"`

	// EnableIPv6 indicates if IPv6 is enabled.
	EnableIPv6 bool `json:"enableIPv6,omitempty"`

	// CreatedAt is the timestamp when the network was created.
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// IPAM contains the current IPAM configuration.
	IPAM *IPAMConfig `json:"ipam,omitempty"`

	// Options are the driver-specific options for the network.
	Options map[string]string `json:"options,omitempty"`

	// Labels are the labels applied to the network.
	Labels map[string]string `json:"labels,omitempty"`

	// Containers is a map of containers attached to the network.
	Containers map[string]*NetworkContainer `json:"containers,omitempty"`
}

// NetworkContainer represents a container attached to the network.
type NetworkContainer struct {
	// Name is the container name.
	Name string `json:"name,omitempty"`

	// EndpointID is the endpoint identifier.
	EndpointID string `json:"endpointID,omitempty"`

	// MacAddress is the MAC address of the container on this network.
	MacAddress string `json:"macAddress,omitempty"`

	// IPv4Address is the IPv4 address of the container on this network.
	IPv4Address string `json:"ipv4Address,omitempty"`

	// IPv6Address is the IPv6 address of the container on this network.
	IPv6Address string `json:"ipv6Address,omitempty"`
}

// +kubebuilder:object:root=true

// A Network is a managed resource that represents a Docker network.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="DRIVER",type="string",JSONPath=".status.atProvider.driver",priority=1
// +kubebuilder:printcolumn:name="SCOPE",type="string",JSONPath=".status.atProvider.scope",priority=1
// +kubebuilder:printcolumn:name="INTERNAL",type="boolean",JSONPath=".status.atProvider.internal",priority=1
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,docker}
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
