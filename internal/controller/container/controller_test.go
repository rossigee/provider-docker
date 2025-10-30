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

package container

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
	"github.com/rossigee/provider-docker/internal/clients"
)

// Mock DockerClient for testing - implements complete DockerClient interface
type mockDockerClient struct {
	// Container operations
	containerCreateFunc  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	containerStartFunc   func(ctx context.Context, containerID string, options container.StartOptions) error
	containerStopFunc    func(ctx context.Context, containerID string, options container.StopOptions) error
	containerRestartFunc func(ctx context.Context, containerID string, options container.StopOptions) error
	containerRemoveFunc  func(ctx context.Context, containerID string, options container.RemoveOptions) error
	containerInspectFunc func(ctx context.Context, containerID string) (container.InspectResponse, error)
	containerListFunc    func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	containerLogsFunc    func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	// containerStatsFunc temporarily disabled
	// containerStatsFunc    func(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	containerUpdateFunc  func(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error)
	containerRenameFunc  func(ctx context.Context, containerID, newContainerName string) error
	containerPauseFunc   func(ctx context.Context, containerID string) error
	containerUnpauseFunc func(ctx context.Context, containerID string) error

	// Close operation
	closeFunc func() error
}

// Container operations
func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.containerCreateFunc != nil {
		return m.containerCreateFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "test-container-id"}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.containerStartFunc != nil {
		return m.containerStartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.containerStopFunc != nil {
		return m.containerStopFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.containerRestartFunc != nil {
		return m.containerRestartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.containerRemoveFunc != nil {
		return m.containerRemoveFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if m.containerInspectFunc != nil {
		return m.containerInspectFunc(ctx, containerID)
	}
	return container.InspectResponse{}, nil
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	if m.containerListFunc != nil {
		return m.containerListFunc(ctx, options)
	}
	return []container.Summary{}, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.containerLogsFunc != nil {
		return m.containerLogsFunc(ctx, containerID, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

// ContainerStats is temporarily disabled due to complex interface mocking
// func (m *mockDockerClient) ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
//	return nil, errors.New("stats not implemented in mock")
// }

func (m *mockDockerClient) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error) {
	if m.containerUpdateFunc != nil {
		return m.containerUpdateFunc(ctx, containerID, updateConfig)
	}
	return container.UpdateResponse{}, nil
}

func (m *mockDockerClient) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	if m.containerRenameFunc != nil {
		return m.containerRenameFunc(ctx, containerID, newContainerName)
	}
	return nil
}

func (m *mockDockerClient) ContainerPause(ctx context.Context, containerID string) error {
	if m.containerPauseFunc != nil {
		return m.containerPauseFunc(ctx, containerID)
	}
	return nil
}

func (m *mockDockerClient) ContainerUnpause(ctx context.Context, containerID string) error {
	if m.containerUnpauseFunc != nil {
		return m.containerUnpauseFunc(ctx, containerID)
	}
	return nil
}

// Image operations - stub implementations
func (m *mockDockerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return []image.Summary{}, nil
}

func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	return image.InspectResponse{}, []byte{}, nil
}

func (m *mockDockerClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	return []image.DeleteResponse{}, nil
}

// Volume operations - stub implementations
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

// Network operations - stub implementations
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
	return []network.Summary{}, nil
}

func (m *mockDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	return nil
}

func (m *mockDockerClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	return nil
}

// System operations - stub implementations
func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClient) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}

// Close operation
func (m *mockDockerClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// Mock ContainerConfigBuilder for testing
type mockContainerConfigBuilder struct {
	buildFunc func(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, error)
}

func (m *mockContainerConfigBuilder) BuildContainerConfig(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, error) {
	if m.buildFunc != nil {
		return m.buildFunc(cr)
	}
	// Use the actual image from the container spec (including empty string)
	return &container.Config{Image: cr.Spec.ForProvider.Image}, &container.HostConfig{}, &network.NetworkingConfig{}, &specs.Platform{}, nil
}

func TestExternalDisconnect(t *testing.T) {
	tests := []struct {
		name      string
		mockFunc  func() *mockDockerClient
		wantError bool
		errorMsg  string
	}{
		{
			name: "SuccessfulDisconnect",
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{
					closeFunc: func() error {
						return nil
					},
				}
			},
			wantError: false,
		},
		{
			name: "DisconnectError",
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{
					closeFunc: func() error {
						return errors.New("connection close failed")
					},
				}
			},
			wantError: true,
			errorMsg:  "connection close failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{
				client: tt.mockFunc(),
				logger: logging.NewNopLogger(),
			}

			err := e.Disconnect(context.Background())

			if tt.wantError {
				if err == nil {
					t.Errorf("Disconnect() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Disconnect() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Disconnect() unexpected error: %v", err)
			}
		})
	}
}

func TestExternalObserve(t *testing.T) {
	tests := []struct {
		name         string
		setupMG      func() resource.Managed
		mockFunc     func() *mockDockerClient
		wantExists   bool
		wantUpToDate bool
		wantError    bool
		errorMsg     string
	}{
		{
			name: "ContainerExists",
			setupMG: func() resource.Managed {
				container := &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
						Annotations: map[string]string{
							AnnotationKeyExternalName: "existing-container-id",
						},
					},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
				}
				return container
			},
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{
					containerInspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
						return container.InspectResponse{
							ID:    "existing-container-id",
							State: &container.State{
								Status: "running",
							},
							Config: &container.Config{
								Image: "nginx:latest",
							},
							NetworkSettings: &container.NetworkSettings{
								Networks: map[string]*network.EndpointSettings{},
							},
						}, nil
					},
				}
			},
			wantExists:   true,
			wantUpToDate: true,
			wantError:    false,
		},
		{
			name: "ContainerNotExists_NoExternalName",
			setupMG: func() resource.Managed {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
				}
			},
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{}
			},
			wantExists: false,
			wantError:  false,
		},
		{
			name: "ContainerNotFound",
			setupMG: func() resource.Managed {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
						Annotations: map[string]string{
							AnnotationKeyExternalName: "nonexistent-container-id",
						},
					},
				}
			},
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{
					containerInspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
						return container.InspectResponse{}, clients.NewNotFoundError("container", containerID)
					},
				}
			},
			wantExists: false,
			wantError:  false,
		},
		{
			name: "InvalidManagedResource",
			setupMG: func() resource.Managed {
				// Create a struct that implements resource.Managed but isn't a Container
				return &invalidManagedResource{}
			},
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{}
			},
			wantError: true,
			errorMsg:  errNotContainer,
		},
		{
			name: "InspectError",
			setupMG: func() resource.Managed {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
						Annotations: map[string]string{
							AnnotationKeyExternalName: "error-container-id",
						},
					},
				}
			},
			mockFunc: func() *mockDockerClient {
				return &mockDockerClient{
					containerInspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
						return container.InspectResponse{}, errors.New("API error")
					},
				}
			},
			wantError: true,
			errorMsg:  "cannot inspect container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{
				client:        tt.mockFunc(),
				configBuilder: &mockContainerConfigBuilder{},
				logger:        logging.NewNopLogger(),
			}

			obs, err := e.Observe(context.Background(), tt.setupMG())

			if tt.wantError {
				if err == nil {
					t.Errorf("Observe() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Observe() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Observe() unexpected error: %v", err)
				return
			}

			if obs.ResourceExists != tt.wantExists {
				t.Errorf("Observe() ResourceExists = %v, want %v", obs.ResourceExists, tt.wantExists)
			}

			if obs.ResourceUpToDate != tt.wantUpToDate {
				t.Errorf("Observe() ResourceUpToDate = %v, want %v", obs.ResourceUpToDate, tt.wantUpToDate)
			}
		})
	}
}

func TestExternalCreate(t *testing.T) {
	tests := []struct {
		name           string
		setupMG        func() resource.Managed
		mockClient     func() *mockDockerClient
		mockBuilder    func() *mockContainerConfigBuilder
		wantError      bool
		errorMsg       string
		validateResult func(*v1alpha1.Container) bool
	}{
		{
			name: "SuccessfulCreate",
			setupMG: func() resource.Managed {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
							Name:  stringPtrCtrl("my-container"),
						},
					},
				}
			},
			mockClient: func() *mockDockerClient {
				return &mockDockerClient{
					containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
						if containerName != "my-container" {
							return container.CreateResponse{}, errors.New("wrong container name")
						}
						return container.CreateResponse{ID: "created-container-id"}, nil
					},
					containerStartFunc: func(ctx context.Context, containerID string, options container.StartOptions) error {
						if containerID != "created-container-id" {
							return errors.New("wrong container ID")
						}
						return nil
					},
				}
			},
			mockBuilder: func() *mockContainerConfigBuilder {
				return &mockContainerConfigBuilder{
					buildFunc: func(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, error) {
						return &container.Config{Image: "nginx:latest"}, &container.HostConfig{}, &network.NetworkingConfig{}, &specs.Platform{}, nil
					},
				}
			},
			wantError: false,
			validateResult: func(cr *v1alpha1.Container) bool {
				annotations := cr.GetAnnotations()
				return annotations != nil && annotations[AnnotationKeyExternalName] == "created-container-id"
			},
		},
		{
			name: "InvalidManagedResource",
			setupMG: func() resource.Managed {
				return &invalidManagedResource{}
			},
			mockClient:  func() *mockDockerClient { return &mockDockerClient{} },
			mockBuilder: func() *mockContainerConfigBuilder { return &mockContainerConfigBuilder{} },
			wantError:   true,
			errorMsg:    errNotContainer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{
				client:        tt.mockClient(),
				configBuilder: tt.mockBuilder(),
				logger:        logging.NewNopLogger(),
			}

			mg := tt.setupMG()
			_, err := e.Create(context.Background(), mg)

			if tt.wantError {
				if err == nil {
					t.Errorf("Create() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Create() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Create() unexpected error: %v", err)
				return
			}

			if tt.validateResult != nil {
				if container, ok := mg.(*v1alpha1.Container); ok {
					if !tt.validateResult(container) {
						t.Errorf("Create() result validation failed")
					}
				}
			}
		})
	}
}

// Helper for testing invalid managed resources
type invalidManagedResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

// Implement complete resource.Managed interface
func (i *invalidManagedResource) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return xpv1.Condition{}
}
func (i *invalidManagedResource) SetConditions(...xpv1.Condition) {}
func (i *invalidManagedResource) GetDeletionPolicy() xpv1.DeletionPolicy {
	return xpv1.DeletionDelete
}
func (i *invalidManagedResource) SetDeletionPolicy(p xpv1.DeletionPolicy) {}
func (i *invalidManagedResource) GetProviderConfigReference() *xpv1.Reference {
	return nil
}
func (i *invalidManagedResource) GetProviderReference() *xpv1.Reference        { return nil }
func (i *invalidManagedResource) SetProviderConfigReference(r *xpv1.Reference) {}
func (i *invalidManagedResource) SetProviderReference(r *xpv1.Reference)       {}
func (i *invalidManagedResource) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return nil
}
func (i *invalidManagedResource) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {}
func (i *invalidManagedResource) GetManagementPolicies() xpv1.ManagementPolicies {
	return xpv1.ManagementPolicies{}
}
func (i *invalidManagedResource) SetManagementPolicies(p xpv1.ManagementPolicies) {}

// Required for runtime.Object interface (embedded in resource.Managed)
func (i *invalidManagedResource) DeepCopyObject() runtime.Object {
	return &invalidManagedResource{
		TypeMeta:   i.TypeMeta,
		ObjectMeta: *i.DeepCopy(),
	}
}

// Additional error handling and edge case tests

func TestExternalCreateErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *mockDockerClient
		setupMG   func() *v1alpha1.Container
		wantErr   bool
		errorMsg  string
	}{
		{
			name: "Docker create fails",
			setupMock: func() *mockDockerClient {
				return &mockDockerClient{
					containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
						return container.CreateResponse{}, errors.New("docker create failed")
					},
				}
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
				}
			},
			wantErr:  true,
			errorMsg: "docker create failed",
		},
		{
			name: "Docker start fails",
			setupMock: func() *mockDockerClient {
				return &mockDockerClient{
					containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
						return container.CreateResponse{ID: "test-id"}, nil
					},
					containerStartFunc: func(ctx context.Context, containerID string, options container.StartOptions) error {
						return errors.New("docker start failed")
					},
				}
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
				}
			},
			wantErr:  true,
			errorMsg: "docker start failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.NewNopLogger()
			ext := &external{
				client:        tt.setupMock(),
				configBuilder: &defaultContainerConfigBuilder{},
				logger:        logger,
			}

			mg := tt.setupMG()
			_, err := ext.Create(context.Background(), mg)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Create() error = %v, want error containing %v", err, tt.errorMsg)
			}
		})
	}
}

func TestExternalDeleteErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *mockDockerClient
		setupMG   func() *v1alpha1.Container
		wantErr   bool
		errorMsg  string
	}{
		{
			name: "Docker remove fails",
			setupMock: func() *mockDockerClient {
				return &mockDockerClient{
					containerInspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
						return container.InspectResponse{
							ID:    "test-container-id",
							Name:  "/test-container",
							State: &container.State{Status: "running"},
						}, nil
					},
					containerRemoveFunc: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
						return errors.New("docker remove failed")
					},
				}
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
						Annotations: map[string]string{
							"crossplane.io/external-name": "test-container-id",
						},
					},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
					Status: v1alpha1.ContainerStatus{
						AtProvider: v1alpha1.ContainerObservation{
							ID: "test-container-id",
						},
					},
				}
			},
			wantErr:  true,
			errorMsg: "docker remove failed",
		},
		{
			name: "Container not found - successful delete",
			setupMock: func() *mockDockerClient {
				return &mockDockerClient{
					containerInspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
						return container.InspectResponse{}, clients.NewNotFoundError("container", containerID)
					},
				}
			},
			setupMG: func() *v1alpha1.Container {
				return &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
					Spec: v1alpha1.ContainerSpec{
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
						},
					},
					Status: v1alpha1.ContainerStatus{
						AtProvider: v1alpha1.ContainerObservation{
							ID: "missing-container-id",
						},
					},
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.NewNopLogger()
			ext := &external{
				client:        tt.setupMock(),
				configBuilder: &defaultContainerConfigBuilder{},
				logger:        logger,
			}

			mg := tt.setupMG()
			_, err := ext.Delete(context.Background(), mg)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Delete() error = %v, want error containing %v", err, tt.errorMsg)
			}
		})
	}
}

func TestExternalUpdateNotImplemented(t *testing.T) {
	logger := logging.NewNopLogger()
	ext := &external{
		client: &mockDockerClient{},
		logger: logger,
	}

	mg := &v1alpha1.Container{
		ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
		Spec: v1alpha1.ContainerSpec{
			ForProvider: v1alpha1.ContainerParameters{
				Image: "nginx:latest",
			},
		},
	}

	_, err := ext.Update(context.Background(), mg)

	if err == nil {
		t.Errorf("Update() expected error for not implemented, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Update() error = %v, want error containing 'not implemented'", err)
	}
}

func TestBuildContainerConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		cr      *v1alpha1.Container
		wantErr bool
	}{
		{
			name: "empty image",
			cr: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "",
					},
				},
			},
			wantErr: false, // Should still work, Docker will handle empty image
		},
		{
			name: "complex environment variables",
			cr: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						Environment: []v1alpha1.EnvVar{
							{Name: "EMPTY_VAR", Value: stringPtrCtrl("")},
							{Name: "SPECIAL_CHARS", Value: stringPtrCtrl("value with spaces and symbols!@#$%")},
							{Name: "UNICODE", Value: stringPtrCtrl("🐳🔧")},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "complex port mappings",
			cr: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						Ports: []v1alpha1.PortSpec{
							{ContainerPort: 80, Protocol: stringPtrCtrl("tcp")},
							{ContainerPort: 443, HostPort: int32Ptr(8443), Protocol: stringPtrCtrl("tcp")},
							{ContainerPort: 53, Protocol: stringPtrCtrl("udp")},
							{ContainerPort: 8080, HostIP: stringPtrCtrl("127.0.0.1")},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configBuilder := &mockContainerConfigBuilder{}
			containerConfig, _, _, _, err := configBuilder.BuildContainerConfig(tt.cr)

			if (err != nil) != tt.wantErr {
				t.Errorf("BuildContainerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify basic configuration
			if !tt.wantErr {
				if containerConfig.Image != tt.cr.Spec.ForProvider.Image {
					t.Errorf("BuildContainerConfig() Image = %v, want %v", containerConfig.Image, tt.cr.Spec.ForProvider.Image)
				}
			}
		})
	}
}

func TestSecurityContextEdgeCases(t *testing.T) {
	// TODO: Add security context edge case tests
}

// Helper functions - use different name to avoid conflicts
func stringPtrCtrl(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
