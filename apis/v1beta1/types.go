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
)

// A ProviderConfigSpec defines the desired state of a ProviderConfig.
type ProviderConfigSpec struct {
	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`

	// Host is the Docker daemon endpoint to connect to.
	// Supported formats:
	// - unix:///var/run/docker.sock (Unix socket)
	// - tcp://host:port (TCP without TLS)
	// - tcp://host:port (TCP with TLS when TLSConfig is provided)
	// +optional
	Host *string `json:"host,omitempty"`

	// TLSConfig configures TLS for TCP connections to Docker daemon.
	// Only used when Host uses tcp:// scheme.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// APIVersion is the Docker API version to use.
	// If not specified, the client will negotiate the API version.
	// +optional
	APIVersion *string `json:"apiVersion,omitempty"`

	// Timeout is the connection timeout for Docker API calls.
	// Defaults to 30s.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RegistryAuth provides default registry authentication.
	// This can be overridden per-container.
	// +optional
	RegistryAuth *RegistryAuth `json:"registryAuth,omitempty"`
}

// TLSConfig configures TLS for Docker daemon connections.
type TLSConfig struct {
	// Verify enables TLS certificate verification.
	// Defaults to true for tcp:// connections.
	// +optional
	Verify *bool `json:"verify,omitempty"`

	// CertPath is the path to the directory containing TLS certificates.
	// Expected files: ca.pem, cert.pem, key.pem
	// +optional
	CertPath *string `json:"certPath,omitempty"`

	// CAData contains the CA certificate data (PEM format).
	// Alternative to CertPath for CA certificate.
	// +optional
	CAData []byte `json:"caData,omitempty"`

	// CertData contains the client certificate data (PEM format).
	// Alternative to CertPath for client certificate.
	// +optional
	CertData []byte `json:"certData,omitempty"`

	// KeyData contains the client private key data (PEM format).
	// Alternative to CertPath for client private key.
	// +optional
	KeyData []byte `json:"keyData,omitempty"`
}

// RegistryAuth configures default registry authentication.
type RegistryAuth struct {
	// Registry is the registry hostname (e.g., docker.io, gcr.io).
	Registry string `json:"registry"`

	// Username for registry authentication.
	// +optional
	Username *string `json:"username,omitempty"`

	// Password for registry authentication.
	// +optional
	Password *string `json:"password,omitempty"`

	// Email for registry authentication (legacy Docker registries).
	// +optional
	Email *string `json:"email,omitempty"`

	// IdentityToken for registry authentication (OAuth/JWT).
	// Alternative to Username/Password.
	// +optional
	IdentityToken *string `json:"identityToken,omitempty"`

	// RegistryToken for registry authentication.
	// Alternative to Username/Password.
	// +optional
	RegistryToken *string `json:"registryToken,omitempty"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=Secret
	Source xpv1.CredentialsSource `json:"source"`

	xpv1.CommonCredentialSelectors `json:",inline"`
}

// A ProviderConfigStatus reflects the observed state of a ProviderConfig.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures a Docker provider.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:printcolumn:name="HOST",type="string",JSONPath=".spec.host",priority=1
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,docker}
// +kubebuilder:storageversion
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// +kubebuilder:object:root=true

// A ProviderConfigUsage indicates that a resource is using a ProviderConfig.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,docker}
// +kubebuilder:storageversion
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}

// GetProviderConfigReference returns the provider config reference.
func (pcu *ProviderConfigUsage) GetProviderConfigReference() xpv1.Reference {
	return pcu.ProviderConfigReference
}

// GetResourceReference returns the resource reference.
func (pcu *ProviderConfigUsage) GetResourceReference() xpv1.TypedReference {
	return pcu.ResourceReference
}

// SetProviderConfigReference sets the provider config reference.
func (pcu *ProviderConfigUsage) SetProviderConfigReference(r xpv1.Reference) {
	pcu.ProviderConfigReference = r
}

// SetResourceReference sets the resource reference.
func (pcu *ProviderConfigUsage) SetResourceReference(r xpv1.TypedReference) {
	pcu.ResourceReference = r
}

// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}
