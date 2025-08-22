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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
)

// Helper functions for tests

// contains checks if string s contains substring substr.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the given bool.
func boolPtr(b bool) *bool {
	return &b
}

func TestBuildPortConfiguration(t *testing.T) {
	type args struct {
		ports []v1alpha1.PortSpec
	}
	type want struct {
		exposedPorts nat.PortSet
		portBindings nat.PortMap
		err          error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyPorts": {
			args: args{
				ports: []v1alpha1.PortSpec{},
			},
			want: want{
				exposedPorts: nat.PortSet{},
				portBindings: nat.PortMap{},
				err:          nil,
			},
		},
		"SingleTCPPort": {
			args: args{
				ports: []v1alpha1.PortSpec{
					{
						ContainerPort: 8080,
						HostPort:      func() *int32 { p := int32(8080); return &p }(),
						Protocol:      func() *string { p := "TCP"; return &p }(),
					},
				},
			},
			want: want{
				exposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
				},
				portBindings: nat.PortMap{
					"8080/tcp": []nat.PortBinding{
						{HostPort: "8080"},
					},
				},
				err: nil,
			},
		},
		"MultiplePortsWithDifferentProtocols": {
			args: args{
				ports: []v1alpha1.PortSpec{
					{
						ContainerPort: 8080,
						HostPort:      func() *int32 { p := int32(8080); return &p }(),
						Protocol:      func() *string { p := "TCP"; return &p }(),
					},
					{
						ContainerPort: 53,
						HostPort:      func() *int32 { p := int32(53); return &p }(),
						Protocol:      func() *string { p := "UDP"; return &p }(),
					},
				},
			},
			want: want{
				exposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
					"53/udp":   struct{}{},
				},
				portBindings: nat.PortMap{
					"8080/tcp": []nat.PortBinding{
						{HostPort: "8080"},
					},
					"53/udp": []nat.PortBinding{
						{HostPort: "53"},
					},
				},
				err: nil,
			},
		},
		"PortWithHostIP": {
			args: args{
				ports: []v1alpha1.PortSpec{
					{
						ContainerPort: 8080,
						HostPort:      func() *int32 { p := int32(8080); return &p }(),
						HostIP:        func() *string { ip := "127.0.0.1"; return &ip }(),
						Protocol:      func() *string { p := "TCP"; return &p }(),
					},
				},
			},
			want: want{
				exposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
				},
				portBindings: nat.PortMap{
					"8080/tcp": []nat.PortBinding{
						{HostIP: "127.0.0.1", HostPort: "8080"},
					},
				},
				err: nil,
			},
		},
		"ExposedPortWithoutBinding": {
			args: args{
				ports: []v1alpha1.PortSpec{
					{
						ContainerPort: 8080,
						Protocol:      func() *string { p := "TCP"; return &p }(),
					},
				},
			},
			want: want{
				exposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
				},
				portBindings: nat.PortMap{},
				err:          nil,
			},
		},
		"DefaultProtocol": {
			args: args{
				ports: []v1alpha1.PortSpec{
					{
						ContainerPort: 8080,
						HostPort:      func() *int32 { p := int32(8080); return &p }(),
					},
				},
			},
			want: want{
				exposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
				},
				portBindings: nat.PortMap{
					"8080/tcp": []nat.PortBinding{
						{HostPort: "8080"},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := &defaultContainerConfigBuilder{}
			gotExposedPorts, gotPortBindings, gotErr := builder.buildPortConfiguration(tc.args.ports)

			if diff := cmp.Diff(tc.want.err, gotErr); diff != "" {
				t.Errorf("buildPortConfiguration() error mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.exposedPorts, gotExposedPorts); diff != "" {
				t.Errorf("buildPortConfiguration() exposedPorts mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.portBindings, gotPortBindings); diff != "" {
				t.Errorf("buildPortConfiguration() portBindings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildVolumeConfiguration(t *testing.T) {
	type args struct {
		volumes []v1alpha1.VolumeMount
	}
	type want struct {
		binds  []string
		mounts int // number of mounts
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyVolumes": {
			args: args{
				volumes: []v1alpha1.VolumeMount{},
			},
			want: want{
				binds:  []string{},
				mounts: 0,
				err:    nil,
			},
		},
		"HostPathVolume": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "host-vol",
						MountPath: "/data",
						VolumeSource: v1alpha1.VolumeSource{
							HostPath: &v1alpha1.HostPathVolumeSource{
								Path: "/host/data",
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{"/host/data:/data"},
				mounts: 0,
				err:    nil,
			},
		},
		"HostPathVolumeReadOnly": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "host-vol",
						MountPath: "/data",
						ReadOnly:  func() *bool { b := true; return &b }(),
						VolumeSource: v1alpha1.VolumeSource{
							HostPath: &v1alpha1.HostPathVolumeSource{
								Path: "/host/data",
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{"/host/data:/data:ro"},
				mounts: 0,
				err:    nil,
			},
		},
		"DockerVolume": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "docker-vol",
						MountPath: "/data",
						VolumeSource: v1alpha1.VolumeSource{
							Volume: &v1alpha1.VolumeVolumeSource{
								VolumeName: "my-volume",
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 1,
				err:    nil,
			},
		},
		"BindMountWithPropagation": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "bind-vol",
						MountPath: "/data",
						VolumeSource: v1alpha1.VolumeSource{
							Bind: &v1alpha1.BindVolumeSource{
								SourcePath:  "/host/bind",
								Propagation: func() *string { p := "shared"; return &p }(),
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 1,
				err:    nil,
			},
		},
		"EmptyDirWithSize": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "tmp-vol",
						MountPath: "/tmp",
						VolumeSource: v1alpha1.VolumeSource{
							EmptyDir: &v1alpha1.EmptyDirVolumeSource{
								SizeLimit: func() *string { s := "100Mi"; return &s }(),
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 1,
				err:    nil,
			},
		},
		"EmptyDirReadOnly": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "tmp-vol",
						MountPath: "/tmp",
						ReadOnly:  func() *bool { b := true; return &b }(),
						VolumeSource: v1alpha1.VolumeSource{
							EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 1,
				err:    nil,
			},
		},
		"SecretVolumeSkipped": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "secret-vol",
						MountPath: "/secrets",
						VolumeSource: v1alpha1.VolumeSource{
							Secret: &v1alpha1.SecretVolumeSource{
								SecretName: "my-secret",
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 0, // Secret volumes are skipped
				err:    nil,
			},
		},
		"ConfigMapVolumeSkipped": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "config-vol",
						MountPath: "/config",
						VolumeSource: v1alpha1.VolumeSource{
							ConfigMap: &v1alpha1.ConfigMapVolumeSource{
								Name: "my-config",
							},
						},
					},
				},
			},
			want: want{
				binds:  []string{},
				mounts: 0, // ConfigMap volumes are skipped
				err:    nil,
			},
		},
		"MultipleVolumeTypes": {
			args: args{
				volumes: []v1alpha1.VolumeMount{
					{
						Name:      "host-vol",
						MountPath: "/host-data",
						VolumeSource: v1alpha1.VolumeSource{
							HostPath: &v1alpha1.HostPathVolumeSource{
								Path: "/host/data",
							},
						},
					},
					{
						Name:      "docker-vol",
						MountPath: "/docker-data",
						VolumeSource: v1alpha1.VolumeSource{
							Volume: &v1alpha1.VolumeVolumeSource{
								VolumeName: "my-volume",
							},
						},
					},
					{
						Name:      "tmp-vol",
						MountPath: "/tmp",
						VolumeSource: v1alpha1.VolumeSource{
							EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			want: want{
				binds:  []string{"/host/data:/host-data"},
				mounts: 2, // Docker volume and EmptyDir
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := &defaultContainerConfigBuilder{}
			gotBinds, gotMounts, gotErr := builder.buildVolumeConfiguration(tc.args.volumes)

			if diff := cmp.Diff(tc.want.err, gotErr); diff != "" {
				t.Errorf("buildVolumeConfiguration() error mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.binds, gotBinds); diff != "" {
				t.Errorf("buildVolumeConfiguration() binds mismatch (-want +got):\n%s", diff)
			}

			if len(gotMounts) != tc.want.mounts {
				t.Errorf("buildVolumeConfiguration() mounts count mismatch: want %d, got %d", tc.want.mounts, len(gotMounts))
			}

			// Validate specific mount properties for certain test cases
			if name == "DockerVolume" && len(gotMounts) > 0 {
				mount := gotMounts[0]
				if mount.Type != "volume" {
					t.Errorf("Expected mount type 'volume', got '%s'", mount.Type)
				}
				if mount.Source != "my-volume" {
					t.Errorf("Expected mount source 'my-volume', got '%s'", mount.Source)
				}
				if mount.Target != "/data" {
					t.Errorf("Expected mount target '/data', got '%s'", mount.Target)
				}
			}

			if name == "BindMountWithPropagation" && len(gotMounts) > 0 {
				mount := gotMounts[0]
				if mount.Type != "bind" {
					t.Errorf("Expected mount type 'bind', got '%s'", mount.Type)
				}
				if mount.Source != "/host/bind" {
					t.Errorf("Expected mount source '/host/bind', got '%s'", mount.Source)
				}
				if mount.BindOptions == nil || mount.BindOptions.Propagation != "shared" {
					t.Errorf("Expected bind propagation 'shared', got %v", mount.BindOptions)
				}
			}

			if name == "EmptyDirWithSize" && len(gotMounts) > 0 {
				mount := gotMounts[0]
				if mount.Type != "tmpfs" {
					t.Errorf("Expected mount type 'tmpfs', got '%s'", mount.Type)
				}
				if mount.Target != "/tmp" {
					t.Errorf("Expected mount target '/tmp', got '%s'", mount.Target)
				}
				if mount.TmpfsOptions == nil || mount.TmpfsOptions.SizeBytes != 100*1024*1024 {
					t.Errorf("Expected tmpfs size 100Mi, got %v", mount.TmpfsOptions)
				}
			}
		})
	}
}

func TestParseByteSize(t *testing.T) {
	type args struct {
		sizeStr string
	}
	type want struct {
		bytes int64
		err   error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Megabytes": {
			args: args{sizeStr: "100Mi"},
			want: want{bytes: 100 * 1024 * 1024, err: nil},
		},
		"Gigabytes": {
			args: args{sizeStr: "2Gi"},
			want: want{bytes: 2 * 1024 * 1024 * 1024, err: nil},
		},
		"RawBytes": {
			args: args{sizeStr: "1024"},
			want: want{bytes: 1024, err: nil},
		},
		"ZeroMegabytes": {
			args: args{sizeStr: "0Mi"},
			want: want{bytes: 0, err: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotBytes, gotErr := parseByteSize(tc.args.sizeStr)

			if tc.want.err != nil {
				if gotErr == nil {
					t.Errorf("parseByteSize() expected error but got none")
				}
				return
			}

			if gotErr != nil {
				t.Errorf("parseByteSize() unexpected error: %v", gotErr)
				return
			}

			if gotBytes != tc.want.bytes {
				t.Errorf("parseByteSize() bytes mismatch: want %d, got %d", tc.want.bytes, gotBytes)
			}
		})
	}
}

func TestBuildNetworkConfiguration(t *testing.T) {
	type args struct {
		networks []v1alpha1.NetworkAttachment
	}
	type want struct {
		hasConfig    bool // whether config should be non-nil
		networkCount int  // number of networks configured
		hasIPAM      bool // whether first network has IPAM config
		err          error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyNetworks": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{},
			},
			want: want{
				hasConfig:    false,
				networkCount: 0,
				hasIPAM:      false,
				err:          nil,
			},
		},
		"SingleNetworkBasic": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name: "my-network",
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      false,
				err:          nil,
			},
		},
		"NetworkWithIPAddress": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:      "my-network",
						IPAddress: func() *string { ip := "192.168.1.100"; return &ip }(),
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      true,
				err:          nil,
			},
		},
		"NetworkWithIPv6Address": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:        "my-network",
						IPv6Address: func() *string { ip := "2001:db8::1"; return &ip }(),
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      true,
				err:          nil,
			},
		},
		"NetworkWithBothIPAddresses": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:        "my-network",
						IPAddress:   func() *string { ip := "192.168.1.100"; return &ip }(),
						IPv6Address: func() *string { ip := "2001:db8::1"; return &ip }(),
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      true,
				err:          nil,
			},
		},
		"NetworkWithAliases": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:    "my-network",
						Aliases: []string{"web", "frontend", "nginx"},
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      false,
				err:          nil,
			},
		},
		"NetworkWithLinks": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:  "my-network",
						Links: []string{"db:database", "cache:redis"},
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 1,
				hasIPAM:      false,
				err:          nil,
			},
		},
		"MultipleNetworks": {
			args: args{
				networks: []v1alpha1.NetworkAttachment{
					{
						Name:      "frontend-network",
						IPAddress: func() *string { ip := "192.168.1.100"; return &ip }(),
						Aliases:   []string{"web"},
					},
					{
						Name:    "backend-network",
						Aliases: []string{"api", "service"},
					},
					{
						Name:        "ipv6-network",
						IPv6Address: func() *string { ip := "2001:db8::1"; return &ip }(),
					},
				},
			},
			want: want{
				hasConfig:    true,
				networkCount: 3,
				hasIPAM:      true, // First network has IP
				err:          nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := &defaultContainerConfigBuilder{}
			gotConfig, gotErr := builder.buildNetworkConfiguration(tc.args.networks)

			if diff := cmp.Diff(tc.want.err, gotErr); diff != "" {
				t.Errorf("buildNetworkConfiguration() error mismatch (-want +got):\n%s", diff)
			}

			// Check if config should exist
			if tc.want.hasConfig {
				if gotConfig == nil {
					t.Errorf("buildNetworkConfiguration() expected config but got nil")
					return
				}
			} else {
				if gotConfig != nil {
					t.Errorf("buildNetworkConfiguration() expected nil config but got %v", gotConfig)
				}
				return
			}

			// Check network count
			if len(gotConfig.EndpointsConfig) != tc.want.networkCount {
				t.Errorf("buildNetworkConfiguration() network count mismatch: want %d, got %d", tc.want.networkCount, len(gotConfig.EndpointsConfig))
			}

			// Validate specific test case properties
			if name == "NetworkWithIPAddress" && len(gotConfig.EndpointsConfig) > 0 {
				endpoint := gotConfig.EndpointsConfig["my-network"]
				if endpoint == nil {
					t.Errorf("Expected endpoint for 'my-network' but got nil")
					return
				}
				if endpoint.IPAMConfig == nil || endpoint.IPAMConfig.IPv4Address != "192.168.1.100" {
					t.Errorf("Expected IPv4Address '192.168.1.100', got %v", endpoint.IPAMConfig)
				}
			}

			if name == "NetworkWithIPv6Address" && len(gotConfig.EndpointsConfig) > 0 {
				endpoint := gotConfig.EndpointsConfig["my-network"]
				if endpoint == nil {
					t.Errorf("Expected endpoint for 'my-network' but got nil")
					return
				}
				if endpoint.IPAMConfig == nil || endpoint.IPAMConfig.IPv6Address != "2001:db8::1" {
					t.Errorf("Expected IPv6Address '2001:db8::1', got %v", endpoint.IPAMConfig)
				}
			}

			if name == "NetworkWithBothIPAddresses" && len(gotConfig.EndpointsConfig) > 0 {
				endpoint := gotConfig.EndpointsConfig["my-network"]
				if endpoint == nil {
					t.Errorf("Expected endpoint for 'my-network' but got nil")
					return
				}
				if endpoint.IPAMConfig == nil ||
					endpoint.IPAMConfig.IPv4Address != "192.168.1.100" ||
					endpoint.IPAMConfig.IPv6Address != "2001:db8::1" {
					t.Errorf("Expected both IP addresses, got %v", endpoint.IPAMConfig)
				}
			}

			if name == "NetworkWithAliases" && len(gotConfig.EndpointsConfig) > 0 {
				endpoint := gotConfig.EndpointsConfig["my-network"]
				if endpoint == nil {
					t.Errorf("Expected endpoint for 'my-network' but got nil")
					return
				}
				expectedAliases := []string{"web", "frontend", "nginx"}
				if diff := cmp.Diff(expectedAliases, endpoint.Aliases); diff != "" {
					t.Errorf("Aliases mismatch (-want +got):\n%s", diff)
				}
			}

			if name == "NetworkWithLinks" && len(gotConfig.EndpointsConfig) > 0 {
				endpoint := gotConfig.EndpointsConfig["my-network"]
				if endpoint == nil {
					t.Errorf("Expected endpoint for 'my-network' but got nil")
					return
				}
				expectedLinks := []string{"db:database", "cache:redis"}
				if diff := cmp.Diff(expectedLinks, endpoint.Links); diff != "" {
					t.Errorf("Links mismatch (-want +got):\n%s", diff)
				}
			}

			if name == "MultipleNetworks" && len(gotConfig.EndpointsConfig) == 3 {
				// Check frontend network
				frontend := gotConfig.EndpointsConfig["frontend-network"]
				if frontend == nil || frontend.IPAMConfig == nil || frontend.IPAMConfig.IPv4Address != "192.168.1.100" {
					t.Errorf("Frontend network not configured properly: %v", frontend)
				}
				if len(frontend.Aliases) != 1 || frontend.Aliases[0] != "web" {
					t.Errorf("Frontend network aliases not configured properly: %v", frontend.Aliases)
				}

				// Check backend network
				backend := gotConfig.EndpointsConfig["backend-network"]
				if backend == nil || len(backend.Aliases) != 2 {
					t.Errorf("Backend network not configured properly: %v", backend)
				}

				// Check IPv6 network
				ipv6Net := gotConfig.EndpointsConfig["ipv6-network"]
				if ipv6Net == nil || ipv6Net.IPAMConfig == nil || ipv6Net.IPAMConfig.IPv6Address != "2001:db8::1" {
					t.Errorf("IPv6 network not configured properly: %v", ipv6Net)
				}
			}
		})
	}
}

func TestBuildSecurityConfiguration(t *testing.T) {
	type args struct {
		securityContext *v1alpha1.SecurityContext
	}
	type want struct {
		configUser       string            // expected user in config
		readonlyRootfs   bool              // expected readonly rootfs in hostconfig
		privileged       *bool             // expected privileged setting (nil = not set)
		capAdd           strslice.StrSlice // expected capabilities to add
		capDrop          strslice.StrSlice // expected capabilities to drop
		securityOptCount int               // number of security options
		hasSelinux       bool              // whether SELinux options are set
		hasSeccomp       bool              // whether Seccomp options are set
		hasAppArmor      bool              // whether AppArmor options are set
		err              error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NilSecurityContext": {
			args: args{
				securityContext: nil,
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"RunAsUserOnly": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					RunAsUser: func() *int64 { u := int64(1000); return &u }(),
				},
			},
			want: want{
				configUser:       "1000",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"RunAsUserAndGroup": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					RunAsUser:  func() *int64 { u := int64(1000); return &u }(),
					RunAsGroup: func() *int64 { g := int64(1000); return &g }(),
				},
			},
			want: want{
				configUser:       "1000:1000",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"RunAsGroupOnly": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					RunAsGroup: func() *int64 { g := int64(1000); return &g }(),
				},
			},
			want: want{
				configUser:       ":1000",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"ReadOnlyRootFilesystem": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					ReadOnlyRootFilesystem: func() *bool { b := true; return &b }(),
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   true,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"DisallowPrivilegeEscalation": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       func() *bool { b := false; return &b }(),
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 0,
				err:              nil,
			},
		},
		"Capabilities": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					Capabilities: &v1alpha1.Capabilities{
						Add:  []string{"NET_ADMIN", "SYS_TIME"},
						Drop: []string{"ALL", "SETUID"},
					},
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           strslice.StrSlice{"NET_ADMIN", "SYS_TIME"},
				capDrop:          strslice.StrSlice{"ALL", "SETUID"},
				securityOptCount: 0,
				err:              nil,
			},
		},
		"SELinuxOptions": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					SELinuxOptions: &v1alpha1.SELinuxOptions{
						User:  func() *string { s := "system_u"; return &s }(),
						Role:  func() *string { s := "system_r"; return &s }(),
						Type:  func() *string { s := "container_t"; return &s }(),
						Level: func() *string { s := "s0:c123,c456"; return &s }(),
					},
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 1,
				hasSelinux:       true,
				err:              nil,
			},
		},
		"SeccompProfile": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					SeccompProfile: &v1alpha1.SeccompProfile{
						Type: "RuntimeDefault",
					},
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 1,
				hasSeccomp:       true,
				err:              nil,
			},
		},
		"AppArmorProfile": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					AppArmorProfile: &v1alpha1.AppArmorProfile{
						Type: "RuntimeDefault",
					},
				},
			},
			want: want{
				configUser:       "",
				readonlyRootfs:   false,
				privileged:       nil,
				capAdd:           nil,
				capDrop:          nil,
				securityOptCount: 1,
				hasAppArmor:      true,
				err:              nil,
			},
		},
		"CompleteSecurityContext": {
			args: args{
				securityContext: &v1alpha1.SecurityContext{
					RunAsUser:                func() *int64 { u := int64(1000); return &u }(),
					RunAsGroup:               func() *int64 { g := int64(1000); return &g }(),
					RunAsNonRoot:             func() *bool { b := true; return &b }(),
					ReadOnlyRootFilesystem:   func() *bool { b := true; return &b }(),
					AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
					Capabilities: &v1alpha1.Capabilities{
						Add:  []string{"NET_BIND_SERVICE"},
						Drop: []string{"ALL"},
					},
					SELinuxOptions: &v1alpha1.SELinuxOptions{
						Type: func() *string { s := "container_t"; return &s }(),
					},
					SeccompProfile: &v1alpha1.SeccompProfile{
						Type: "RuntimeDefault",
					},
					AppArmorProfile: &v1alpha1.AppArmorProfile{
						Type: "RuntimeDefault",
					},
				},
			},
			want: want{
				configUser:       "1000:1000",
				readonlyRootfs:   true,
				privileged:       func() *bool { b := false; return &b }(),
				capAdd:           strslice.StrSlice{"NET_BIND_SERVICE"},
				capDrop:          strslice.StrSlice{"ALL"},
				securityOptCount: 3, // SELinux, Seccomp, AppArmor
				hasSelinux:       true,
				hasSeccomp:       true,
				hasAppArmor:      true,
				err:              nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := &defaultContainerConfigBuilder{}
			config := &container.Config{}
			hostConfig := &container.HostConfig{}

			gotErr := builder.buildSecurityConfiguration(tc.args.securityContext, config, hostConfig)

			if diff := cmp.Diff(tc.want.err, gotErr); diff != "" {
				t.Errorf("buildSecurityConfiguration() error mismatch (-want +got):\n%s", diff)
			}

			// Check config.User
			if config.User != tc.want.configUser {
				t.Errorf("buildSecurityConfiguration() config.User = %v, want %v", config.User, tc.want.configUser)
			}

			// Check hostConfig.ReadonlyRootfs
			if hostConfig.ReadonlyRootfs != tc.want.readonlyRootfs {
				t.Errorf("buildSecurityConfiguration() hostConfig.ReadonlyRootfs = %v, want %v", hostConfig.ReadonlyRootfs, tc.want.readonlyRootfs)
			}

			// Check hostConfig.Privileged
			if tc.want.privileged != nil {
				if hostConfig.Privileged != *tc.want.privileged {
					t.Errorf("buildSecurityConfiguration() hostConfig.Privileged = %v, want %v", hostConfig.Privileged, *tc.want.privileged)
				}
			}

			// Check capabilities
			if diff := cmp.Diff(tc.want.capAdd, hostConfig.CapAdd); diff != "" {
				t.Errorf("buildSecurityConfiguration() CapAdd mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.capDrop, hostConfig.CapDrop); diff != "" {
				t.Errorf("buildSecurityConfiguration() CapDrop mismatch (-want +got):\n%s", diff)
			}

			// Check security options count
			if len(hostConfig.SecurityOpt) != tc.want.securityOptCount {
				t.Errorf("buildSecurityConfiguration() SecurityOpt count = %d, want %d", len(hostConfig.SecurityOpt), tc.want.securityOptCount)
			}

			// Check specific security options
			if tc.want.hasSelinux {
				found := false
				for _, opt := range hostConfig.SecurityOpt {
					if strings.HasPrefix(opt, "label:") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildSecurityConfiguration() expected SELinux security option but not found in %v", hostConfig.SecurityOpt)
				}
			}

			if tc.want.hasSeccomp {
				found := false
				for _, opt := range hostConfig.SecurityOpt {
					if strings.HasPrefix(opt, "seccomp:") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildSecurityConfiguration() expected Seccomp security option but not found in %v", hostConfig.SecurityOpt)
				}
			}

			if tc.want.hasAppArmor {
				found := false
				for _, opt := range hostConfig.SecurityOpt {
					if strings.HasPrefix(opt, "apparmor:") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildSecurityConfiguration() expected AppArmor security option but not found in %v", hostConfig.SecurityOpt)
				}
			}

			// Validate specific test case details
			if name == "SELinuxOptions" && len(hostConfig.SecurityOpt) > 0 {
				expectedPattern := "label:user:system_u,role:system_r,type:container_t,level:s0:c123,c456"
				if hostConfig.SecurityOpt[0] != expectedPattern {
					t.Errorf("SELinux label mismatch: got %s, want %s", hostConfig.SecurityOpt[0], expectedPattern)
				}
			}

			if name == "SeccompProfile" && len(hostConfig.SecurityOpt) > 0 {
				expected := "seccomp:runtime/default"
				if hostConfig.SecurityOpt[0] != expected {
					t.Errorf("Seccomp profile mismatch: got %s, want %s", hostConfig.SecurityOpt[0], expected)
				}
			}

			if name == "AppArmorProfile" && len(hostConfig.SecurityOpt) > 0 {
				expected := "apparmor:docker-default"
				if hostConfig.SecurityOpt[0] != expected {
					t.Errorf("AppArmor profile mismatch: got %s, want %s", hostConfig.SecurityOpt[0], expected)
				}
			}
		})
	}
}

func TestBuildHealthCheckConfiguration(t *testing.T) {
	type args struct {
		healthCheck *v1alpha1.HealthCheck
	}
	type want struct {
		hasHealthCheck bool           // whether healthcheck should be set
		testCommand    []string       // expected test command
		interval       *time.Duration // expected interval
		timeout        *time.Duration // expected timeout
		startPeriod    *time.Duration // expected start period
		retries        *int           // expected retries
		err            error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NilHealthCheck": {
			args: args{
				healthCheck: nil,
			},
			want: want{
				hasHealthCheck: false,
				err:            nil,
			},
		},
		"BasicHealthCheck": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test: []string{"CMD", "curl", "-f", "http://localhost/health"},
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "curl", "-f", "http://localhost/health"},
				err:            nil,
			},
		},
		"HealthCheckWithInterval": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test:     []string{"CMD", "wget", "--spider", "http://localhost:8080/healthz"},
					Interval: &metav1.Duration{Duration: 30 * time.Second},
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "wget", "--spider", "http://localhost:8080/healthz"},
				interval:       func() *time.Duration { d := 30 * time.Second; return &d }(),
				err:            nil,
			},
		},
		"HealthCheckWithTimeout": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test:    []string{"CMD", "nc", "-z", "localhost", "3306"},
					Timeout: &metav1.Duration{Duration: 10 * time.Second},
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "nc", "-z", "localhost", "3306"},
				timeout:        func() *time.Duration { d := 10 * time.Second; return &d }(),
				err:            nil,
			},
		},
		"HealthCheckWithStartPeriod": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test:        []string{"CMD", "redis-cli", "ping"},
					StartPeriod: &metav1.Duration{Duration: 60 * time.Second},
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "redis-cli", "ping"},
				startPeriod:    func() *time.Duration { d := 60 * time.Second; return &d }(),
				err:            nil,
			},
		},
		"HealthCheckWithRetries": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test:    []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
					Retries: func() *int { r := 5; return &r }(),
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
				retries:        func() *int { r := 5; return &r }(),
				err:            nil,
			},
		},
		"CompleteHealthCheck": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test:        []string{"CMD", "curl", "-f", "http://localhost:8080/actuator/health"},
					Interval:    &metav1.Duration{Duration: 30 * time.Second},
					Timeout:     &metav1.Duration{Duration: 10 * time.Second},
					StartPeriod: &metav1.Duration{Duration: 120 * time.Second},
					Retries:     func() *int { r := 3; return &r }(),
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD", "curl", "-f", "http://localhost:8080/actuator/health"},
				interval:       func() *time.Duration { d := 30 * time.Second; return &d }(),
				timeout:        func() *time.Duration { d := 10 * time.Second; return &d }(),
				startPeriod:    func() *time.Duration { d := 120 * time.Second; return &d }(),
				retries:        func() *int { r := 3; return &r }(),
				err:            nil,
			},
		},
		"ShellFormHealthCheck": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test: []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				},
			},
			want: want{
				hasHealthCheck: true,
				testCommand:    []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				err:            nil,
			},
		},
		"EmptyTestCommand": {
			args: args{
				healthCheck: &v1alpha1.HealthCheck{
					Test: []string{},
				},
			},
			want: want{
				hasHealthCheck: false,
				err:            errors.New("health check test command is required"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := &defaultContainerConfigBuilder{}
			config := &container.Config{}

			gotErr := builder.buildHealthCheckConfiguration(tc.args.healthCheck, config)

			if tc.want.err != nil {
				if gotErr == nil {
					t.Errorf("buildHealthCheckConfiguration() expected error but got none")
					return
				}
				if gotErr.Error() != tc.want.err.Error() {
					t.Errorf("buildHealthCheckConfiguration() error = %v, want %v", gotErr, tc.want.err)
				}
				return
			}

			if gotErr != nil {
				t.Errorf("buildHealthCheckConfiguration() unexpected error: %v", gotErr)
				return
			}

			// Check if health check should be set
			if tc.want.hasHealthCheck {
				if config.Healthcheck == nil {
					t.Errorf("buildHealthCheckConfiguration() expected healthcheck but got nil")
					return
				}
			} else {
				if config.Healthcheck != nil {
					t.Errorf("buildHealthCheckConfiguration() expected nil healthcheck but got %v", config.Healthcheck)
				}
				return
			}

			// Check test command
			if diff := cmp.Diff(tc.want.testCommand, config.Healthcheck.Test); diff != "" {
				t.Errorf("buildHealthCheckConfiguration() test command mismatch (-want +got):\n%s", diff)
			}

			// Check interval
			if tc.want.interval != nil {
				if config.Healthcheck.Interval != *tc.want.interval {
					t.Errorf("buildHealthCheckConfiguration() interval = %v, want %v", config.Healthcheck.Interval, *tc.want.interval)
				}
			} else if config.Healthcheck.Interval != 0 {
				t.Errorf("buildHealthCheckConfiguration() expected no interval but got %v", config.Healthcheck.Interval)
			}

			// Check timeout
			if tc.want.timeout != nil {
				if config.Healthcheck.Timeout != *tc.want.timeout {
					t.Errorf("buildHealthCheckConfiguration() timeout = %v, want %v", config.Healthcheck.Timeout, *tc.want.timeout)
				}
			} else if config.Healthcheck.Timeout != 0 {
				t.Errorf("buildHealthCheckConfiguration() expected no timeout but got %v", config.Healthcheck.Timeout)
			}

			// Check start period
			if tc.want.startPeriod != nil {
				if config.Healthcheck.StartPeriod != *tc.want.startPeriod {
					t.Errorf("buildHealthCheckConfiguration() startPeriod = %v, want %v", config.Healthcheck.StartPeriod, *tc.want.startPeriod)
				}
			} else if config.Healthcheck.StartPeriod != 0 {
				t.Errorf("buildHealthCheckConfiguration() expected no start period but got %v", config.Healthcheck.StartPeriod)
			}

			// Check retries
			if tc.want.retries != nil {
				if config.Healthcheck.Retries != *tc.want.retries {
					t.Errorf("buildHealthCheckConfiguration() retries = %v, want %v", config.Healthcheck.Retries, *tc.want.retries)
				}
			} else if config.Healthcheck.Retries != 0 {
				t.Errorf("buildHealthCheckConfiguration() expected no retries but got %v", config.Healthcheck.Retries)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	type args struct {
		durationStr string
	}
	type want struct {
		duration time.Duration
		err      error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Seconds": {
			args: args{durationStr: "30s"},
			want: want{duration: 30 * time.Second, err: nil},
		},
		"Minutes": {
			args: args{durationStr: "5m"},
			want: want{duration: 5 * time.Minute, err: nil},
		},
		"Hours": {
			args: args{durationStr: "2h"},
			want: want{duration: 2 * time.Hour, err: nil},
		},
		"Milliseconds": {
			args: args{durationStr: "500ms"},
			want: want{duration: 500 * time.Millisecond, err: nil},
		},
		"Complex": {
			args: args{durationStr: "1h30m45s"},
			want: want{duration: 1*time.Hour + 30*time.Minute + 45*time.Second, err: nil},
		},
		"Zero": {
			args: args{durationStr: "0s"},
			want: want{duration: 0, err: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotDuration, gotErr := parseDuration(tc.args.durationStr)

			if tc.want.err != nil {
				if gotErr == nil {
					t.Errorf("parseDuration() expected error but got none")
				}
				return
			}

			if gotErr != nil {
				t.Errorf("parseDuration() unexpected error: %v", gotErr)
				return
			}

			if gotDuration != tc.want.duration {
				t.Errorf("parseDuration() duration = %v, want %v", gotDuration, tc.want.duration)
			}
		})
	}
}

func TestBuildContainerConfig(t *testing.T) {
	type args struct {
		container *v1alpha1.Container
	}
	type want struct {
		configFields map[string]interface{} // Key fields to verify
		hostFields   map[string]interface{} // Key host config fields to verify
		err          error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BasicContainer": {
			args: args{
				container: &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
					},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{},
						ForProvider: v1alpha1.ContainerParameters{
							Image:   "nginx:latest",
							Command: []string{"/bin/sh"},
							Args:    []string{"-c", "echo hello"},
						},
					},
				},
			},
			want: want{
				configFields: map[string]interface{}{
					"Image": "nginx:latest",
					"Cmd":   []string{"/bin/sh", "-c", "echo hello"}, // Will be converted to StrSlice
				},
				err: nil,
			},
		},
		"ContainerWithPorts": {
			args: args{
				container: &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
					},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{},
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
							Ports: []v1alpha1.PortSpec{
								{
									ContainerPort: 80,
									HostPort:      func() *int32 { p := int32(8080); return &p }(),
								},
							},
						},
					},
				},
			},
			want: want{
				configFields: map[string]interface{}{
					"Image": "nginx:latest",
				},
				err: nil,
			},
		},
		"ContainerWithEnvironment": {
			args: args{
				container: &v1alpha1.Container{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-container",
					},
					Spec: v1alpha1.ContainerSpec{
						ResourceSpec: xpv1.ResourceSpec{},
						ForProvider: v1alpha1.ContainerParameters{
							Image: "nginx:latest",
							Environment: []v1alpha1.EnvVar{
								{
									Name:  "ENV_VAR1",
									Value: func() *string { v := "value1"; return &v }(),
								},
								{
									Name:  "ENV_VAR2",
									Value: func() *string { v := "value2"; return &v }(),
								},
							},
						},
					},
				},
			},
			want: want{
				configFields: map[string]interface{}{
					"Image": "nginx:latest",
					"Env":   []string{"ENV_VAR1=value1", "ENV_VAR2=value2"},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewContainerConfigBuilder()
			gotConfig, gotHostConfig, _, _, gotErr := builder.BuildContainerConfig(tc.args.container)

			if diff := cmp.Diff(tc.want.err, gotErr); diff != "" {
				t.Errorf("BuildContainerConfig() error mismatch (-want +got):\n%s", diff)
				return
			}

			if gotErr == nil {
				// Verify config fields
				for field, expectedValue := range tc.want.configFields {
					var gotValue interface{}
					switch field {
					case "Image":
						gotValue = gotConfig.Image
					case "Cmd":
						gotValue = []string(gotConfig.Cmd) // Convert StrSlice to []string
					case "Env":
						gotValue = gotConfig.Env
					}

					if diff := cmp.Diff(expectedValue, gotValue); diff != "" {
						t.Errorf("BuildContainerConfig() config.%s mismatch (-want +got):\n%s", field, diff)
					}
				}

				// Verify host config fields
				for field, expectedValue := range tc.want.hostFields {
					var gotValue interface{}
					switch field {
					case "RestartPolicy":
						gotValue = gotHostConfig.RestartPolicy
					}

					if diff := cmp.Diff(expectedValue, gotValue); diff != "" {
						t.Errorf("BuildContainerConfig() hostConfig.%s mismatch (-want +got):\n%s", field, diff)
					}
				}
			}
		})
	}
}

func TestBuildEnvironmentConfiguration(t *testing.T) {
	builder := NewContainerConfigBuilder().(*defaultContainerConfigBuilder)

	tests := []struct {
		name     string
		envVars  []v1alpha1.EnvVar
		expected []string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "EmptyEnvironment",
			envVars:  nil,
			expected: []string{},
			wantErr:  false,
		},
		{
			name: "SimpleEnvironmentVariables",
			envVars: []v1alpha1.EnvVar{
				{
					Name:  "ENV1",
					Value: stringPtr("value1"),
				},
				{
					Name:  "ENV2",
					Value: stringPtr("value2"),
				},
			},
			expected: []string{"ENV1=value1", "ENV2=value2"},
			wantErr:  false,
		},
		{
			name: "EnvironmentWithEmptyValue",
			envVars: []v1alpha1.EnvVar{
				{
					Name:  "EMPTY_VAR",
					Value: stringPtr(""),
				},
			},
			expected: []string{"EMPTY_VAR="},
			wantErr:  false,
		},
		{
			name: "EnvironmentWithSpecialCharacters",
			envVars: []v1alpha1.EnvVar{
				{
					Name:  "SPECIAL_VAR",
					Value: stringPtr("value with spaces and = signs"),
				},
			},
			expected: []string{"SPECIAL_VAR=value with spaces and = signs"},
			wantErr:  false,
		},
		{
			name: "ConfigMapValueFrom",
			envVars: []v1alpha1.EnvVar{
				{
					Name: "CONFIG_VAR",
					ValueFrom: &v1alpha1.EnvVarSource{
						ConfigMapKeyRef: &v1alpha1.ConfigMapKeySelector{
							Name: "my-config",
							Key:  "config-key",
						},
					},
				},
			},
			expected: nil,
			wantErr:  true,
			errMsg:   "ConfigMap valueFrom not yet implemented",
		},
		{
			name: "SecretValueFrom",
			envVars: []v1alpha1.EnvVar{
				{
					Name: "SECRET_VAR",
					ValueFrom: &v1alpha1.EnvVarSource{
						SecretKeyRef: &v1alpha1.SecretKeySelector{
							Name: "my-secret",
							Key:  "secret-key",
						},
					},
				},
			},
			expected: nil,
			wantErr:  true,
			errMsg:   "Secret valueFrom not yet implemented",
		},
		{
			name: "EnvironmentVariableWithoutValueOrValueFrom",
			envVars: []v1alpha1.EnvVar{
				{
					Name: "INVALID_VAR",
					// No Value or ValueFrom specified
				},
			},
			expected: nil,
			wantErr:  true,
			errMsg:   "has no value or valueFrom specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := builder.buildEnvironmentConfiguration(tt.envVars)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d environment variables, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected environment variable %d to be %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestIsEnvVarOptional(t *testing.T) {
	builder := NewContainerConfigBuilder().(*defaultContainerConfigBuilder)

	tests := []struct {
		name      string
		valueFrom *v1alpha1.EnvVarSource
		expected  bool
	}{
		{
			name: "ConfigMapOptionalTrue",
			valueFrom: &v1alpha1.EnvVarSource{
				ConfigMapKeyRef: &v1alpha1.ConfigMapKeySelector{
					Name:     "config",
					Key:      "key",
					Optional: boolPtr(true),
				},
			},
			expected: true,
		},
		{
			name: "ConfigMapOptionalFalse",
			valueFrom: &v1alpha1.EnvVarSource{
				ConfigMapKeyRef: &v1alpha1.ConfigMapKeySelector{
					Name:     "config",
					Key:      "key",
					Optional: boolPtr(false),
				},
			},
			expected: false,
		},
		{
			name: "ConfigMapOptionalNil",
			valueFrom: &v1alpha1.EnvVarSource{
				ConfigMapKeyRef: &v1alpha1.ConfigMapKeySelector{
					Name: "config",
					Key:  "key",
				},
			},
			expected: false,
		},
		{
			name: "SecretOptionalTrue",
			valueFrom: &v1alpha1.EnvVarSource{
				SecretKeyRef: &v1alpha1.SecretKeySelector{
					Name:     "secret",
					Key:      "key",
					Optional: boolPtr(true),
				},
			},
			expected: true,
		},
		{
			name: "SecretOptionalFalse",
			valueFrom: &v1alpha1.EnvVarSource{
				SecretKeyRef: &v1alpha1.SecretKeySelector{
					Name:     "secret",
					Key:      "key",
					Optional: boolPtr(false),
				},
			},
			expected: false,
		},
		{
			name: "SecretOptionalNil",
			valueFrom: &v1alpha1.EnvVarSource{
				SecretKeyRef: &v1alpha1.SecretKeySelector{
					Name: "secret",
					Key:  "key",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.isEnvVarOptional(tt.valueFrom)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
