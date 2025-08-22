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

package clients

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
	"github.com/rossigee/provider-docker/apis/v1beta1"
)

func TestDockerClientInterface(t *testing.T) {
	// Simple test to verify the interface compiles correctly
	var _ DockerClient = (*dockerClient)(nil)
}

func TestGetProviderConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	tests := []struct {
		name         string
		prepareFunc  func() *fake.ClientBuilder
		setupMG      func() *v1alpha1.Container
		wantError    bool
		errorMsg     string
		validateSpec func(*v1beta1.ProviderConfigSpec) bool
	}{
		{
			name: "ValidProviderConfig",
			prepareFunc: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.ProviderConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-config",
						},
						Spec: v1beta1.ProviderConfigSpec{
							Host: &[]string{"unix:///var/run/docker.sock"}[0],
						},
					},
				)
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{Name: "test-config"},
						},
					},
				}
			},
			wantError: false,
			validateSpec: func(spec *v1beta1.ProviderConfigSpec) bool {
				return spec.Host != nil && *spec.Host == "unix:///var/run/docker.sock"
			},
		},
		{
			name: "ProviderConfigNotFound",
			prepareFunc: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{Name: "nonexistent"},
						},
					},
				}
			},
			wantError: true,
			errorMsg:  "not found",
		},
		{
			name: "NoProviderConfigReference",
			prepareFunc: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{
							// No ProviderConfigReference
						},
					},
				}
			},
			wantError: true,
			errorMsg:  errNoProviderConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := tt.prepareFunc().Build()
			mg := tt.setupMG()

			config, err := GetProviderConfig(context.Background(), kubeClient, mg)

			if tt.wantError {
				if err == nil {
					t.Errorf("GetProviderConfig() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("GetProviderConfig() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("GetProviderConfig() unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Errorf("GetProviderConfig() returned nil config")
				return
			}

			if tt.validateSpec != nil && !tt.validateSpec(&config.Spec) {
				t.Errorf("GetProviderConfig() spec validation failed")
			}
		})
	}
}

func TestExtractCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	tests := []struct {
		name         string
		prepareFunc  func() *fake.ClientBuilder
		providerConfig *v1beta1.ProviderConfig
		wantError    bool
		errorMsg     string
		validateData func(*DockerCredentials) bool
	}{
		{
			name: "ValidSecretCredentials",
			prepareFunc: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-secret",
							Namespace: "crossplane-system",
						},
						Data: map[string][]byte{
							"ca.pem":   []byte("ca-cert-data"),
							"cert.pem": []byte("cert-data"),
							"key.pem":  []byte("key-data"),
						},
					},
				)
			},
			providerConfig: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{
					Credentials: v1beta1.ProviderCredentials{
						Source: xpv1.CredentialsSourceSecret,
						CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
							SecretRef: &xpv1.SecretKeySelector{
								SecretReference: xpv1.SecretReference{
									Name:      "test-secret",
									Namespace: "crossplane-system",
								},
								Key: "ca.pem",
							},
						},
					},
				},
			},
			wantError: false,
			validateData: func(data *DockerCredentials) bool {
				return data != nil
			},
		},
		{
			name: "NoSecretRef",
			prepareFunc: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			providerConfig: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{
					Credentials: v1beta1.ProviderCredentials{
						Source: xpv1.CredentialsSourceSecret,
						// No SecretRef
					},
				},
			},
			wantError: false,
			validateData: func(data *DockerCredentials) bool {
				return data != nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := tt.prepareFunc().Build()

			data, err := ExtractCredentials(context.Background(), kubeClient, tt.providerConfig)

			if tt.wantError {
				if err == nil {
					t.Errorf("ExtractCredentials() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ExtractCredentials() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractCredentials() unexpected error: %v", err)
				return
			}

			if tt.validateData != nil && !tt.validateData(data) {
				t.Errorf("ExtractCredentials() data validation failed")
			}
		})
	}
}

func TestNewDockerClient(t *testing.T) {
	scheme := runtime.NewScheme()
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	tests := []struct {
		name        string
		setupClient func() *fake.ClientBuilder
		setupMG     func() *v1alpha1.Container
		wantError   bool
		errorMsg    string
	}{
		{
			name: "ValidConfiguration",
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.ProviderConfig{
						ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
						Spec: v1beta1.ProviderConfigSpec{
							Host: &[]string{"unix:///var/run/docker.sock"}[0],
						},
					},
				)
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{Name: "test-config"},
						},
					},
				}
			},
			wantError: false,
		},
		{
			name: "MissingProviderConfig",
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{Name: "nonexistent"},
						},
					},
				}
			},
			wantError: true,
			errorMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := tt.setupClient().Build()
			mg := tt.setupMG()

			client, err := NewDockerClient(context.Background(), kubeClient, mg)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewDockerClient() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("NewDockerClient() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewDockerClient() unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Errorf("NewDockerClient() returned nil client")
			}
		})
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || (len(s) > len(substr) && 
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}