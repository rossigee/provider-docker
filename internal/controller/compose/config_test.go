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

package compose

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	composev1alpha1 "github.com/rossigee/provider-docker/apis/compose/v1alpha1"
)

func TestExternal_GetValueFromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		name      string
		cr        *composev1alpha1.ComposeStack
		secretRef *composev1alpha1.SecretKeySelector
		secret    *corev1.Secret
		wantValue string
		wantErr   bool
	}{
		{
			name: "successful secret resolution",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			secretRef: &composev1alpha1.SecretKeySelector{
				Name: "test-secret",
				Key:  "password",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"password": []byte("secret-password"),
					"username": []byte("admin"),
				},
			},
			wantValue: "secret-password",
			wantErr:   false,
		},
		{
			name: "secret with explicit namespace",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			secretRef: &composev1alpha1.SecretKeySelector{
				Name:      "test-secret",
				Namespace: stringPtr("kube-system"),
				Key:       "api-key",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"api-key": []byte("test-api-key"),
				},
			},
			wantValue: "test-api-key",
			wantErr:   false,
		},
		{
			name: "secret not found",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			secretRef: &composev1alpha1.SecretKeySelector{
				Name: "missing-secret",
				Key:  "password",
			},
			secret:    nil, // No secret created
			wantValue: "",
			wantErr:   true,
		},
		{
			name: "key not found in secret",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			secretRef: &composev1alpha1.SecretKeySelector{
				Name: "test-secret",
				Key:  "missing-key",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"password": []byte("secret-password"),
				},
			},
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.secret != nil {
				objs = append(objs, tt.secret)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			ext := &external{
				kube: fakeClient,
			}

			gotValue, err := ext.getValueFromSecret(context.Background(), tt.cr, tt.secretRef)

			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotValue != tt.wantValue {
				t.Errorf("getValueFromSecret() = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestExternal_GetValueFromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		name         string
		cr           *composev1alpha1.ComposeStack
		configMapRef *composev1alpha1.ConfigMapKeySelector
		configMap    *corev1.ConfigMap
		wantValue    string
		wantErr      bool
	}{
		{
			name: "successful configmap resolution",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			configMapRef: &composev1alpha1.ConfigMapKeySelector{
				Name: "test-config",
				Key:  "database-host",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"database-host": "postgres.example.com",
					"log-level":     "info",
				},
			},
			wantValue: "postgres.example.com",
			wantErr:   false,
		},
		{
			name: "configmap with explicit namespace",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			configMapRef: &composev1alpha1.ConfigMapKeySelector{
				Name:      "test-config",
				Namespace: stringPtr("kube-system"),
				Key:       "cluster-name",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"cluster-name": "production",
				},
			},
			wantValue: "production",
			wantErr:   false,
		},
		{
			name: "configmap not found",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			configMapRef: &composev1alpha1.ConfigMapKeySelector{
				Name: "missing-config",
				Key:  "some-key",
			},
			configMap: nil, // No configmap created
			wantValue: "",
			wantErr:   true,
		},
		{
			name: "key not found in configmap",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
			},
			configMapRef: &composev1alpha1.ConfigMapKeySelector{
				Name: "test-config",
				Key:  "missing-key",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"database-host": "postgres.example.com",
				},
			},
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.configMap != nil {
				objs = append(objs, tt.configMap)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			ext := &external{
				kube: fakeClient,
			}

			gotValue, err := ext.getValueFromConfigMap(context.Background(), tt.cr, tt.configMapRef)

			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFromConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotValue != tt.wantValue {
				t.Errorf("getValueFromConfigMap() = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestExternal_BuildEnvironment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret-password"),
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"database-host": "postgres.example.com",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, configMap).
		Build()

	ext := &external{
		kube: fakeClient,
	}

	cr := &composev1alpha1.ComposeStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stack",
			Namespace: "default",
		},
		Spec: composev1alpha1.ComposeStackSpec{
			ForProvider: composev1alpha1.ComposeStackParameters{
				Environment: []composev1alpha1.ComposeEnvVar{
					{
						Name:  "NODE_ENV",
						Value: stringPtr("production"),
					},
					{
						Name: "DATABASE_HOST",
						ValueFrom: &composev1alpha1.EnvVarSource{
							ConfigMapKeyRef: &composev1alpha1.ConfigMapKeySelector{
								Name: "test-config",
								Key:  "database-host",
							},
						},
					},
					{
						Name: "DATABASE_PASSWORD",
						ValueFrom: &composev1alpha1.EnvVarSource{
							SecretKeyRef: &composev1alpha1.SecretKeySelector{
								Name: "test-secret",
								Key:  "password",
							},
						},
					},
				},
			},
		},
	}

	env := ext.buildEnvironment(context.Background(), cr)

	expectedEnv := map[string]string{
		"NODE_ENV":          "production",
		"DATABASE_HOST":     "postgres.example.com",
		"DATABASE_PASSWORD": "secret-password",
	}

	if diff := cmp.Diff(expectedEnv, env); diff != "" {
		t.Errorf("Environment mismatch (-want +got):\n%s", diff)
	}
}

func TestExternal_GetComposeContent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"docker-compose.yml": `
version: '3.8'
services:
  web:
    image: nginx:latest
`,
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"docker-compose.yml": []byte(`
version: '3.8'
services:
  api:
    image: node:18-alpine
`),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap, secret).
		Build()

	tests := []struct {
		name    string
		cr      *composev1alpha1.ComposeStack
		want    string
		wantErr bool
	}{
		{
			name: "inline compose content",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				Spec: composev1alpha1.ComposeStackSpec{
					ForProvider: composev1alpha1.ComposeStackParameters{
						Compose: stringPtr(`
version: '3.8'
services:
  redis:
    image: redis:latest
`),
					},
				},
			},
			want: `
version: '3.8'
services:
  redis:
    image: redis:latest
`,
			wantErr: false,
		},
		{
			name: "configmap reference",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				Spec: composev1alpha1.ComposeStackSpec{
					ForProvider: composev1alpha1.ComposeStackParameters{
						ComposeRef: &composev1alpha1.ComposeReference{
							ConfigMapRef: &composev1alpha1.ConfigMapReference{
								Name: "compose-config",
								Key:  "docker-compose.yml",
							},
						},
					},
				},
			},
			want: `
version: '3.8'
services:
  web:
    image: nginx:latest
`,
			wantErr: false,
		},
		{
			name: "secret reference",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				Spec: composev1alpha1.ComposeStackSpec{
					ForProvider: composev1alpha1.ComposeStackParameters{
						ComposeRef: &composev1alpha1.ComposeReference{
							SecretRef: &composev1alpha1.SecretReference{
								Name: "compose-secret",
								Key:  "docker-compose.yml",
							},
						},
					},
				},
			},
			want: `
version: '3.8'
services:
  api:
    image: node:18-alpine
`,
			wantErr: false,
		},
		{
			name: "no compose content",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				Spec: composev1alpha1.ComposeStackSpec{
					ForProvider: composev1alpha1.ComposeStackParameters{
						// No compose content specified
					},
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := &external{
				kube: fakeClient,
			}

			got, err := ext.getComposeContent(context.Background(), tt.cr)

			if (err != nil) != tt.wantErr {
				t.Errorf("getComposeContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("getComposeContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
