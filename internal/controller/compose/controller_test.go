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

	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	composev1alpha1 "github.com/rossigee/provider-docker/apis/compose/v1alpha1"
	"github.com/rossigee/provider-docker/internal/compose"
)

// Mock Docker Client
type mockDockerClient struct {
	containers           []types.Container
	containerInspectResp *types.ContainerJSON
	containerCreateResp  container.CreateResponse
	inspectError         error
	createError          error
	startError           error
	removeError          error
	listError            error
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.containers, nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	if m.inspectError != nil {
		return types.ContainerJSON{}, m.inspectError
	}
	if m.containerInspectResp != nil {
		return *m.containerInspectResp, nil
	}
	return types.ContainerJSON{}, nil
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.createError != nil {
		return container.CreateResponse{}, m.createError
	}
	return m.containerCreateResp, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return m.startError
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return m.removeError
}

// Additional required methods for DockerClient interface
func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}

func (m *mockDockerClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}

func (m *mockDockerClient) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.ContainerUpdateOKBody, error) {
	return container.ContainerUpdateOKBody{}, nil
}

func (m *mockDockerClient) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	return nil
}

func (m *mockDockerClient) ContainerPause(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDockerClient) ContainerUnpause(ctx context.Context, containerID string) error {
	return nil
}

// Image operations
func (m *mockDockerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDockerClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return nil, nil
}

func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	return types.ImageInspect{}, nil, nil
}

func (m *mockDockerClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	return nil, nil
}

// Volume operations
func (m *mockDockerClient) VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error) {
	return volume.Volume{}, nil
}

func (m *mockDockerClient) VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error) {
	return volume.Volume{}, nil
}

func (m *mockDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	return nil
}

func (m *mockDockerClient) VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error) {
	return volume.ListResponse{}, nil
}

// Network operations
func (m *mockDockerClient) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	return network.CreateResponse{}, nil
}

func (m *mockDockerClient) NetworkInspect(ctx context.Context, networkID string, options network.InspectOptions) (network.Inspect, error) {
	return network.Inspect{}, nil
}

func (m *mockDockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	return nil
}

func (m *mockDockerClient) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
	return nil, nil
}

func (m *mockDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	return nil
}

func (m *mockDockerClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	return nil
}

// System operations
func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClient) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

func TestExternal_Disconnect(t *testing.T) {
	ext := &external{
		service: &mockDockerClient{},
	}

	err := ext.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() error = %v, wantErr false", err)
	}
}

func TestExternal_Observe(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		name         string
		cr           *composev1alpha1.ComposeStack
		dockerClient *mockDockerClient
		wantExists   bool
		wantErr      bool
		wantUpToDate bool
	}{
		{
			name: "stack exists and is up to date",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containers: []types.Container{
					{
						ID:    "container123",
						Names: []string{"/test-stack_web_1"},
						State: "running",
						Image: "nginx:latest",
						Labels: map[string]string{
							"com.docker.compose.project": "test-stack",
							"com.docker.compose.service": "web",
						},
					},
				},
				containerInspectResp: &types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:    "container123",
						Name:  "/test-stack_web_1",
						State: &types.ContainerState{Status: "running"},
					},
					Config: &container.Config{
						Image: "nginx:latest",
					},
				},
			},
			wantExists:   true,
			wantErr:      false,
			wantUpToDate: true,
		},
		{
			name: "stack does not exist",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-stack",
					Namespace: "default",
				},
				Spec: composev1alpha1.ComposeStackSpec{
					ForProvider: composev1alpha1.ComposeStackParameters{
						Compose: stringPtr(`
version: '3.8'
services:
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containers:   []types.Container{}, // No containers
				inspectError: errors.New("container not found"),
			},
			wantExists:   false,
			wantErr:      false,
			wantUpToDate: false,
		},
		{
			name: "docker inspect error",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				inspectError: errors.New("docker inspect error"),
			},
			wantExists:   false,
			wantErr:      false,
			wantUpToDate: false,
		},
		{
			name: "invalid managed resource",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				// Missing spec - invalid resource
			},
			dockerClient: &mockDockerClient{},
			wantExists:   false,
			wantErr:      true,
			wantUpToDate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			ext := &external{
				kube:    fakeClient,
				service: tt.dockerClient,
				parser:  &compose.Parser{},
			}

			obs, err := ext.Observe(context.Background(), tt.cr)

			if (err != nil) != tt.wantErr {
				t.Errorf("Observe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if obs.ResourceExists != tt.wantExists {
					t.Errorf("Observe() ResourceExists = %v, want %v", obs.ResourceExists, tt.wantExists)
				}
				if obs.ResourceUpToDate != tt.wantUpToDate {
					t.Errorf("Observe() ResourceUpToDate = %v, want %v", obs.ResourceUpToDate, tt.wantUpToDate)
				}
			}
		})
	}
}

func TestExternal_Create(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		name         string
		cr           *composev1alpha1.ComposeStack
		dockerClient *mockDockerClient
		wantErr      bool
	}{
		{
			name: "successful create",
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
  web:
    image: nginx:latest
    ports:
      - "80:80"
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containerCreateResp: container.CreateResponse{
					ID: "container123",
				},
			},
			wantErr: false,
		},
		{
			name: "docker create error",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				inspectError: errors.New("container not found"),  // Container doesn't exist
				createError:  errors.New("docker create failed"), // Create will fail
			},
			wantErr: true,
		},
		{
			name: "invalid managed resource",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-stack",
					Namespace: "default",
				},
				// Missing spec - invalid resource
			},
			dockerClient: &mockDockerClient{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			ext := &external{
				kube:    fakeClient,
				service: tt.dockerClient,
				parser:  &compose.Parser{},
			}

			_, err := ext.Create(context.Background(), tt.cr)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExternal_Update(t *testing.T) {
	ext := &external{}

	_, err := ext.Update(context.Background(), &composev1alpha1.ComposeStack{})

	// Update should return ErrNotImplemented
	if err == nil {
		t.Errorf("Update() error = nil, want error")
	}
}

func TestExternal_Delete(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = composev1alpha1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		name         string
		cr           *composev1alpha1.ComposeStack
		dockerClient *mockDockerClient
		wantErr      bool
	}{
		{
			name: "successful delete",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containers: []types.Container{
					{
						ID:    "container123",
						Names: []string{"/test-stack_web_1"},
						State: "running",
						Labels: map[string]string{
							"com.docker.compose.project": "test-stack",
							"com.docker.compose.service": "web",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "docker remove error",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containers: []types.Container{
					{
						ID:    "container123",
						Names: []string{"/test-stack_web_1"},
						State: "running",
						Labels: map[string]string{
							"com.docker.compose.project": "test-stack",
							"com.docker.compose.service": "web",
						},
					},
				},
				removeError: errors.New("docker remove failed"),
			},
			wantErr: true,
		},
		{
			name: "no containers to delete",
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
  web:
    image: nginx:latest
`),
					},
				},
			},
			dockerClient: &mockDockerClient{
				containers: []types.Container{}, // No containers
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			ext := &external{
				kube:    fakeClient,
				service: tt.dockerClient,
				parser:  &compose.Parser{},
			}

			_, err := ext.Delete(context.Background(), tt.cr)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExternal_GetProjectName(t *testing.T) {
	tests := []struct {
		name string
		cr   *composev1alpha1.ComposeStack
		want string
	}{
		{
			name: "basic project name",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
			},
			want: "my-app",
		},
		{
			name: "project name with namespace",
			cr: &composev1alpha1.ComposeStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-service",
					Namespace: "production",
				},
			},
			want: "web-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := &external{}
			got := ext.getProjectName(tt.cr)
			if got != tt.want {
				t.Errorf("getProjectName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExternal_GetContainerName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		serviceName string
		want        string
	}{
		{
			name:        "basic container name",
			projectName: "myapp",
			serviceName: "web",
			want:        "myapp_web_1",
		},
		{
			name:        "complex names",
			projectName: "multi-tier-app",
			serviceName: "redis-cache",
			want:        "multi-tier-app_redis-cache_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := &external{}
			got := ext.getContainerName(tt.projectName, tt.serviceName)
			if got != tt.want {
				t.Errorf("getContainerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
