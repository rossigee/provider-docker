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
	"sort"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
	"github.com/rossigee/provider-docker/internal/clients"
)

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name             string
		container        *v1alpha1.Container
		containerInfo    *container.InspectResponse
		expectedStatus   *v1alpha1.ContainerStatus
		validateFunction func(*v1alpha1.ContainerStatus) bool
	}{
		{
			name: "RunningContainer",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:   "abc123",
					Name: "/test-container",
					State: &container.State{
						Status:    "running",
						StartedAt: "2023-01-01T10:00:00Z",
						Running:   true,
						Pid:       1234,
						ExitCode:  0,
					},
				},
				Config: &container.Config{
					Image: "nginx:latest",
				},
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {
							IPAddress: "172.17.0.2",
						},
					},
				},
			},
			validateFunction: func(status *v1alpha1.ContainerStatus) bool {
				return status.AtProvider.ID == "abc123" &&
					status.AtProvider.State.Status == "running" &&
					status.AtProvider.Started != nil
			},
		},
		{
			name: "ExitedContainer",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:   "def456",
					Name: "/test-container",
					State: &container.State{
						Status:     "exited",
						StartedAt:  "2023-01-01T10:00:00Z",
						FinishedAt: "2023-01-01T10:05:00Z",
						Running:    false,
						Pid:        0,
						ExitCode:   1,
						Error:      "Process exited with code 1",
					},
				},
				Config: &container.Config{
					Image: "nginx:latest",
				},
			},
			validateFunction: func(status *v1alpha1.ContainerStatus) bool {
				return status.AtProvider.ID == "def456" &&
					status.AtProvider.State.Status == "exited" &&
					status.AtProvider.State.ExitCode == 1
			},
		},
		{
			name: "ContainerWithHealthCheck",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						HealthCheck: &v1alpha1.HealthCheck{
							Test:     []string{"CMD", "curl", "-f", "http://localhost/health"},
							Interval: &metav1.Duration{Duration: 30 * time.Second},
						},
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:   "ghi789",
					Name: "/test-container",
					State: &container.State{
						Status:  "running",
						Running: true,
						Health: &container.Health{
							Status:        "healthy",
							FailingStreak: 0,
							Log: []*container.HealthcheckResult{
								{
									Start:    time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
									End:      time.Date(2023, 1, 1, 10, 0, 1, 0, time.UTC),
									ExitCode: 0,
									Output:   "OK",
								},
							},
						},
					},
				},
				Config: &container.Config{
					Image: "nginx:latest",
				},
			},
			validateFunction: func(status *v1alpha1.ContainerStatus) bool {
				return status.AtProvider.ID == "ghi789" &&
					status.AtProvider.State.Status == "running"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			e.updateStatus(tt.container, tt.containerInfo)

			if tt.validateFunction != nil && !tt.validateFunction(&tt.container.Status) {
				t.Errorf("updateStatus() validation failed for %s", tt.name)
				t.Logf("Status: %+v", tt.container.Status)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	tests := []struct {
		name          string
		container     *v1alpha1.Container
		containerInfo *container.InspectResponse
		expected      bool
	}{
		{
			name: "UpToDateContainer",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						Environment: []v1alpha1.EnvVar{
							{Name: "TEST_VAR", Value: stringPtrStatusStatus("test_value")},
						},
						Labels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID: "abc123",
				},
				Config: &container.Config{
					Image: "nginx:latest",
					Env:   []string{"TEST_VAR=test_value"},
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			expected: true,
		},
		{
			name: "OutdatedImage",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:1.21",
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID: "abc123",
				},
				Config: &container.Config{
					Image: "nginx:1.20",
				},
			},
			expected: false,
		},
		{
			name: "OutdatedEnvironment",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						Environment: []v1alpha1.EnvVar{
							{Name: "TEST_VAR", Value: stringPtrStatus("new_value")},
						},
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID: "abc123",
				},
				Config: &container.Config{
					Image: "nginx:latest",
					Env:   []string{"TEST_VAR=old_value"},
				},
			},
			expected: false,
		},
		{
			name: "OutdatedLabels",
			container: &v1alpha1.Container{
				ObjectMeta: metav1.ObjectMeta{Name: "test-container"},
				Spec: v1alpha1.ContainerSpec{
					ForProvider: v1alpha1.ContainerParameters{
						Image: "nginx:latest",
						Labels: map[string]string{
							"app":     "test",
							"version": "v2",
						},
					},
				},
			},
			containerInfo: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID: "abc123",
				},
				Config: &container.Config{
					Image: "nginx:latest",
					Labels: map[string]string{
						"app":     "test",
						"version": "v1",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			result := e.isUpToDate(tt.container, tt.containerInfo)

			if result != tt.expected {
				t.Errorf("isUpToDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsEnvironmentUpToDate(t *testing.T) {
	tests := []struct {
		name        string
		desired     []v1alpha1.EnvVar
		actual      []string
		expected    bool
		description string
	}{
		{
			name:        "EmptyEnvironments",
			desired:     []v1alpha1.EnvVar{},
			actual:      []string{},
			expected:    true,
			description: "Both environments are empty",
		},
		{
			name: "MatchingEnvironments",
			desired: []v1alpha1.EnvVar{
				{Name: "VAR1", Value: stringPtrStatus("value1")},
				{Name: "VAR2", Value: stringPtrStatus("value2")},
			},
			actual:      []string{"VAR1=value1", "VAR2=value2"},
			expected:    true,
			description: "All environment variables match",
		},
		{
			name: "ExtraActualVar",
			desired: []v1alpha1.EnvVar{
				{Name: "VAR1", Value: stringPtrStatus("value1")},
			},
			actual:      []string{"VAR1=value1", "VAR2=value2"},
			expected:    true,
			description: "Extra variables in actual are allowed",
		},
		{
			name: "MissingDesiredVar",
			desired: []v1alpha1.EnvVar{
				{Name: "VAR1", Value: stringPtrStatus("value1")},
				{Name: "VAR2", Value: stringPtrStatus("value2")},
			},
			actual:      []string{"VAR1=value1"},
			expected:    false,
			description: "Missing desired variable in actual",
		},
		{
			name: "DifferentValue",
			desired: []v1alpha1.EnvVar{
				{Name: "VAR1", Value: stringPtrStatus("value1")},
			},
			actual:      []string{"VAR1=different_value"},
			expected:    false,
			description: "Same variable name but different value",
		},
		{
			name: "EmptyValue",
			desired: []v1alpha1.EnvVar{
				{Name: "VAR1", Value: stringPtrStatus("")},
			},
			actual:      []string{"VAR1="},
			expected:    true,
			description: "Empty values should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			result := e.isEnvironmentUpToDate(tt.desired, tt.actual)

			if result != tt.expected {
				t.Errorf("isEnvironmentUpToDate() = %v, want %v - %s", result, tt.expected, tt.description)
				t.Logf("Desired: %+v", tt.desired)
				t.Logf("Actual: %+v", tt.actual)
			}
		})
	}
}

func TestIsLabelsUpToDate(t *testing.T) {
	tests := []struct {
		name        string
		desired     map[string]string
		actual      map[string]string
		expected    bool
		description string
	}{
		{
			name:        "EmptyLabels",
			desired:     map[string]string{},
			actual:      map[string]string{},
			expected:    true,
			description: "Both label maps are empty",
		},
		{
			name: "MatchingLabels",
			desired: map[string]string{
				"app":     "test",
				"version": "v1",
			},
			actual: map[string]string{
				"app":     "test",
				"version": "v1",
			},
			expected:    true,
			description: "All labels match exactly",
		},
		{
			name: "ExtraActualLabels",
			desired: map[string]string{
				"app": "test",
			},
			actual: map[string]string{
				"app":    "test",
				"extra":  "label",
				"system": "generated",
			},
			expected:    true,
			description: "Extra labels in actual are allowed",
		},
		{
			name: "MissingDesiredLabel",
			desired: map[string]string{
				"app":     "test",
				"version": "v1",
			},
			actual: map[string]string{
				"app": "test",
			},
			expected:    false,
			description: "Missing desired label in actual",
		},
		{
			name: "DifferentLabelValue",
			desired: map[string]string{
				"app": "test",
			},
			actual: map[string]string{
				"app": "different",
			},
			expected:    false,
			description: "Same label key but different value",
		},
		{
			name: "EmptyStringValue",
			desired: map[string]string{
				"empty": "",
			},
			actual: map[string]string{
				"empty": "",
			},
			expected:    true,
			description: "Empty string values should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			result := e.isLabelsUpToDate(tt.desired, tt.actual)

			if result != tt.expected {
				t.Errorf("isLabelsUpToDate() = %v, want %v - %s", result, tt.expected, tt.description)
				t.Logf("Desired: %+v", tt.desired)
				t.Logf("Actual: %+v", tt.actual)
			}
		})
	}
}

func TestBuildObservedPorts(t *testing.T) {
	tests := []struct {
		name          string
		containerInfo *container.InspectResponse
		expected      []v1alpha1.ContainerPort
	}{
		{
			name: "EmptyPortMap",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{},
					},
				},
			},
			expected: []v1alpha1.ContainerPort{},
		},
		{
			name: "SinglePort",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "8080"},
							},
						},
					},
				},
			},
			expected: []v1alpha1.ContainerPort{
				{
					PrivatePort: 80,
					Type:        "tcp",
					IP:          "0.0.0.0",
					PublicPort:  8080,
				},
			},
		},
		{
			name: "MultiplePorts",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "8080"},
							},
							"443/tcp": []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "8443"},
							},
							"53/udp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "5353"},
							},
						},
					},
				},
			},
			expected: []v1alpha1.ContainerPort{
				{PrivatePort: 80, Type: "tcp", IP: "0.0.0.0", PublicPort: 8080},
				{PrivatePort: 443, Type: "tcp", IP: "127.0.0.1", PublicPort: 8443},
				{PrivatePort: 53, Type: "udp", IP: "0.0.0.0", PublicPort: 5353},
			},
		},
		{
			name: "MultipleBindingsPerPort",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "8080"},
								{HostIP: "127.0.0.1", HostPort: "8081"},
							},
						},
					},
				},
			},
			expected: []v1alpha1.ContainerPort{
				{PrivatePort: 80, Type: "tcp", IP: "0.0.0.0", PublicPort: 8080},
				{PrivatePort: 80, Type: "tcp", IP: "127.0.0.1", PublicPort: 8081},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			result := e.buildObservedPorts(tt.containerInfo)

			if len(result) != len(tt.expected) {
				t.Errorf("buildObservedPorts() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			// Sort both slices for comparison since order might vary
			sortPorts := func(ports []v1alpha1.ContainerPort) {
				sort.Slice(ports, func(i, j int) bool {
					if ports[i].PrivatePort != ports[j].PrivatePort {
						return ports[i].PrivatePort < ports[j].PrivatePort
					}
					return ports[i].Type < ports[j].Type
				})
			}
			sortPorts(tt.expected)
			sortPorts(result)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("buildObservedPorts() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildObservedNetworks(t *testing.T) {
	tests := []struct {
		name          string
		containerInfo *container.InspectResponse
		expected      map[string]v1alpha1.NetworkInfo
	}{
		{
			name: "EmptyNetworks",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{},
				},
			},
			expected: map[string]v1alpha1.NetworkInfo{},
		},
		{
			name: "SingleNetwork",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {
							IPAddress: "172.17.0.2",
							Gateway:   "172.17.0.1",
						},
					},
				},
			},
			expected: map[string]v1alpha1.NetworkInfo{
				"bridge": {
					IPAddress: "172.17.0.2",
					Gateway:   "172.17.0.1",
				},
			},
		},
		{
			name: "MultipleNetworks",
			containerInfo: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {
							IPAddress: "172.17.0.2",
							Gateway:   "172.17.0.1",
						},
						"custom": {
							IPAddress: "192.168.1.10",
							Gateway:   "192.168.1.1",
						},
					},
				},
			},
			expected: map[string]v1alpha1.NetworkInfo{
				"bridge": {IPAddress: "172.17.0.2", Gateway: "172.17.0.1"},
				"custom": {IPAddress: "192.168.1.10", Gateway: "192.168.1.1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{}
			result := e.buildObservedNetworks(tt.containerInfo)

			if len(result) != len(tt.expected) {
				t.Errorf("buildObservedNetworks() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("buildObservedNetworks() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "NilError",
			err:      nil,
			expected: false,
		},
		{
			name:     "NotFoundError",
			err:      clients.NewNotFoundError("container", "abc123"),
			expected: true,
		},
		{
			name:     "GenericError",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "DockerNotFoundError",
			err:      clients.NewNotFoundError("container", "test-id"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFound(tt.err)

			if result != tt.expected {
				t.Errorf("isNotFound() = %v, want %v for error: %v", result, tt.expected, tt.err)
			}
		})
	}
}

// Helper functions
func stringPtrStatus(s string) *string {
	return &s
}

func stringPtrStatusStatus(s string) *string {
	return &s
}
