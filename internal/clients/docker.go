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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	dockerclient "github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/rossigee/provider-docker/apis/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// AddrString returns the textual form of addr, or "" when addr is unset.
func AddrString(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

// PrefixString returns the textual form of prefix, or "" when prefix is unset.
func PrefixString(prefix netip.Prefix) string {
	if !prefix.IsValid() {
		return ""
	}
	return prefix.String()
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	ResourceType string
	ResourceID   string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %s not found", e.ResourceType, e.ResourceID)
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(resourceType, resourceID string) error {
	return &NotFoundError{
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

const (
	errNoProviderConfig     = "no providerConfig specified"
	errGetProviderConfig    = "cannot get providerConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errUnmarshalCredentials = "cannot unmarshal credentials"
	errCreateDockerClient   = "cannot create Docker client"
)

// DockerClient is an interface for Docker operations.
type DockerClient interface {
	// Container operations
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options dockerclient.ContainerStartOptions) error
	ContainerStop(ctx context.Context, containerID string, options dockerclient.ContainerStopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options dockerclient.ContainerStopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options dockerclient.ContainerRemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerList(ctx context.Context, options dockerclient.ContainerListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, containerID string, options dockerclient.ContainerLogsOptions) (io.ReadCloser, error)
	// ContainerStats temporarily disabled due to complex interface mocking
	// ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error)
	ContainerRename(ctx context.Context, containerID, newContainerName string) error
	ContainerPause(ctx context.Context, containerID string) error
	ContainerUnpause(ctx context.Context, containerID string) error

	// Image operations
	ImagePull(ctx context.Context, refStr string, options dockerclient.ImagePullOptions) (io.ReadCloser, error)
	ImageList(ctx context.Context, options dockerclient.ImageListOptions) ([]image.Summary, error)
	ImageRemove(ctx context.Context, imageID string, options dockerclient.ImageRemoveOptions) ([]image.DeleteResponse, error)

	// Volume operations
	VolumeCreate(ctx context.Context, options dockerclient.VolumeCreateOptions) (volume.Volume, error)
	VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumeList(ctx context.Context, options dockerclient.VolumeListOptions) (volume.ListResponse, error)

	// Network operations
	NetworkCreate(ctx context.Context, name string, options dockerclient.NetworkCreateOptions) (network.CreateResponse, error)
	NetworkInspect(ctx context.Context, networkID string, options dockerclient.NetworkInspectOptions) (network.Inspect, error)
	NetworkRemove(ctx context.Context, networkID string) error
	NetworkList(ctx context.Context, options dockerclient.NetworkListOptions) ([]network.Summary, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error

	// System operations
	Ping(ctx context.Context) (dockerclient.PingResult, error)
	Info(ctx context.Context) (system.Info, error)
	ServerVersion(ctx context.Context) (dockerclient.ServerVersionResult, error)

	// Close the client
	Close() error
}

// dockerClient is a wrapper around the Docker client that implements DockerClient.
type dockerClient struct {
	*dockerclient.Client
}

// Ensure dockerClient implements DockerClient interface
var _ DockerClient = (*dockerClient)(nil)

// The moby client takes option structs and returns result structs. The methods
// below adapt it to the DockerClient interface, so callers keep the shapes they use.

func (c *dockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	res, err := c.Client.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             containerName,
	})
	return container.CreateResponse{ID: res.ID, Warnings: res.Warnings}, err
}

func (c *dockerClient) ContainerStart(ctx context.Context, containerID string, options dockerclient.ContainerStartOptions) error {
	_, err := c.Client.ContainerStart(ctx, containerID, options)
	return err
}

func (c *dockerClient) ContainerStop(ctx context.Context, containerID string, options dockerclient.ContainerStopOptions) error {
	_, err := c.Client.ContainerStop(ctx, containerID, options)
	return err
}

func (c *dockerClient) ContainerRestart(ctx context.Context, containerID string, options dockerclient.ContainerStopOptions) error {
	_, err := c.Client.ContainerRestart(ctx, containerID, dockerclient.ContainerRestartOptions(options))
	return err
}

func (c *dockerClient) ContainerRemove(ctx context.Context, containerID string, options dockerclient.ContainerRemoveOptions) error {
	_, err := c.Client.ContainerRemove(ctx, containerID, options)
	return err
}

func (c *dockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	res, err := c.Client.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	return res.Container, err
}

func (c *dockerClient) ContainerList(ctx context.Context, options dockerclient.ContainerListOptions) ([]container.Summary, error) {
	res, err := c.Client.ContainerList(ctx, options)
	return res.Items, err
}

func (c *dockerClient) ContainerLogs(ctx context.Context, containerID string, options dockerclient.ContainerLogsOptions) (io.ReadCloser, error) {
	return c.Client.ContainerLogs(ctx, containerID, options)
}

func (c *dockerClient) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error) {
	resources := updateConfig.Resources
	restartPolicy := updateConfig.RestartPolicy
	res, err := c.Client.ContainerUpdate(ctx, containerID, dockerclient.ContainerUpdateOptions{
		Resources:     &resources,
		RestartPolicy: &restartPolicy,
	})
	return container.UpdateResponse{Warnings: res.Warnings}, err
}

func (c *dockerClient) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	_, err := c.Client.ContainerRename(ctx, containerID, dockerclient.ContainerRenameOptions{NewName: newContainerName})
	return err
}

func (c *dockerClient) ContainerPause(ctx context.Context, containerID string) error {
	_, err := c.Client.ContainerPause(ctx, containerID, dockerclient.ContainerPauseOptions{})
	return err
}

func (c *dockerClient) ContainerUnpause(ctx context.Context, containerID string) error {
	_, err := c.Client.ContainerUnpause(ctx, containerID, dockerclient.ContainerUnpauseOptions{})
	return err
}

func (c *dockerClient) ImagePull(ctx context.Context, refStr string, options dockerclient.ImagePullOptions) (io.ReadCloser, error) {
	return c.Client.ImagePull(ctx, refStr, options)
}

func (c *dockerClient) ImageList(ctx context.Context, options dockerclient.ImageListOptions) ([]image.Summary, error) {
	res, err := c.Client.ImageList(ctx, options)
	return res.Items, err
}

func (c *dockerClient) ImageRemove(ctx context.Context, imageID string, options dockerclient.ImageRemoveOptions) ([]image.DeleteResponse, error) {
	res, err := c.Client.ImageRemove(ctx, imageID, options)
	return res.Items, err
}

func (c *dockerClient) VolumeCreate(ctx context.Context, options dockerclient.VolumeCreateOptions) (volume.Volume, error) {
	res, err := c.Client.VolumeCreate(ctx, options)
	return res.Volume, err
}

func (c *dockerClient) VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error) {
	res, err := c.Client.VolumeInspect(ctx, volumeID, dockerclient.VolumeInspectOptions{})
	return res.Volume, err
}

func (c *dockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	_, err := c.Client.VolumeRemove(ctx, volumeID, dockerclient.VolumeRemoveOptions{Force: force})
	return err
}

func (c *dockerClient) VolumeList(ctx context.Context, options dockerclient.VolumeListOptions) (volume.ListResponse, error) {
	res, err := c.Client.VolumeList(ctx, options)
	volumes := make([]volume.Volume, 0, len(res.Items))
	volumes = append(volumes, res.Items...)
	return volume.ListResponse{Volumes: volumes, Warnings: res.Warnings}, err
}

func (c *dockerClient) NetworkCreate(ctx context.Context, name string, options dockerclient.NetworkCreateOptions) (network.CreateResponse, error) {
	res, err := c.Client.NetworkCreate(ctx, name, options)
	return network.CreateResponse{ID: res.ID, Warning: strings.Join(res.Warning, "\n")}, err
}

func (c *dockerClient) NetworkInspect(ctx context.Context, networkID string, options dockerclient.NetworkInspectOptions) (network.Inspect, error) {
	res, err := c.Client.NetworkInspect(ctx, networkID, options)
	return res.Network, err
}

func (c *dockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	_, err := c.Client.NetworkRemove(ctx, networkID, dockerclient.NetworkRemoveOptions{})
	return err
}

func (c *dockerClient) NetworkList(ctx context.Context, options dockerclient.NetworkListOptions) ([]network.Summary, error) {
	res, err := c.Client.NetworkList(ctx, options)
	return res.Items, err
}

func (c *dockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	_, err := c.Client.NetworkConnect(ctx, networkID, dockerclient.NetworkConnectOptions{
		Container:      containerID,
		EndpointConfig: config,
	})
	return err
}

func (c *dockerClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	_, err := c.Client.NetworkDisconnect(ctx, networkID, dockerclient.NetworkDisconnectOptions{
		Container: containerID,
		Force:     force,
	})
	return err
}

func (c *dockerClient) Ping(ctx context.Context) (dockerclient.PingResult, error) {
	return c.Client.Ping(ctx, dockerclient.PingOptions{})
}

func (c *dockerClient) ServerVersion(ctx context.Context) (dockerclient.ServerVersionResult, error) {
	return c.Client.ServerVersion(ctx, dockerclient.ServerVersionOptions{})
}

func (c *dockerClient) Info(ctx context.Context) (system.Info, error) {
	res, err := c.Client.Info(ctx, dockerclient.InfoOptions{})
	return res.Info, err
}

// NewDockerClient creates a new Docker client from a ProviderConfig.
func NewDockerClient(ctx context.Context, k8s k8sclient.Client, mg resource.Managed) (DockerClient, error) {
	pc, err := GetProviderConfig(ctx, k8s, mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	if err := TrackProviderConfigUsage(ctx, k8s, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	creds, err := ExtractCredentials(ctx, k8s, pc)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}

	dockerCli, err := createDockerClient(pc, creds)
	if err != nil {
		return nil, errors.Wrap(err, errCreateDockerClient)
	}

	return &dockerClient{Client: dockerCli}, nil
}

// createDockerClient creates a new Docker client with the given configuration.
func createDockerClient(pc *v1beta1.ProviderConfig, creds *DockerCredentials) (*dockerclient.Client, error) {
	opts := []dockerclient.Opt{
		dockerclient.FromEnv,
	}

	// Set host if specified
	if pc.Spec.Host != nil {
		opts = append(opts, dockerclient.WithHost(*pc.Spec.Host))
	}

	// Pin the API version if specified; otherwise the client negotiates it with the daemon.
	if pc.Spec.APIVersion != nil {
		opts = append(opts, dockerclient.WithAPIVersion(*pc.Spec.APIVersion))
	}

	// Configure TLS if needed
	if pc.Spec.TLSConfig != nil {
		httpClient, err := createHTTPClientWithTLS(pc.Spec.TLSConfig, creds)
		if err != nil {
			return nil, err
		}
		opts = append(opts, dockerclient.WithHTTPClient(httpClient))
	}

	// Set timeout
	timeout := 30 * time.Second
	if pc.Spec.Timeout != nil {
		timeout = pc.Spec.Timeout.Duration
	}
	opts = append(opts, dockerclient.WithTimeout(timeout))

	return dockerclient.New(opts...)
}

// createHTTPClientWithTLS creates an HTTP client with TLS configuration.
func createHTTPClientWithTLS(tlsConfig *v1beta1.TLSConfig, creds *DockerCredentials) (*http.Client, error) {
	tlsConf := &tls.Config{}

	// Configure certificate verification
	if tlsConfig.Verify != nil {
		tlsConf.InsecureSkipVerify = !*tlsConfig.Verify // #nosec G402 -- gated by TLSConfig.Verify in provider config
	}

	// Load CA certificate
	if len(tlsConfig.CAData) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(tlsConfig.CAData) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConf.RootCAs = caCertPool
	} else if creds != nil && len(creds.CAData) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(creds.CAData) {
			return nil, errors.New("failed to parse CA certificate from credentials")
		}
		tlsConf.RootCAs = caCertPool
	}

	// Load client certificate and key
	if len(tlsConfig.CertData) > 0 && len(tlsConfig.KeyData) > 0 {
		cert, err := tls.X509KeyPair(tlsConfig.CertData, tlsConfig.KeyData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load client certificate")
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	} else if creds != nil && len(creds.CertData) > 0 && len(creds.KeyData) > 0 {
		cert, err := tls.X509KeyPair(creds.CertData, creds.KeyData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load client certificate from credentials")
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	} else if tlsConfig.CertPath != nil {
		// Load from file path
		options := tlsconfig.Options{
			CertFile: *tlsConfig.CertPath + "/cert.pem",
			KeyFile:  *tlsConfig.CertPath + "/key.pem",
		}
		if tlsConfig.Verify != nil && !*tlsConfig.Verify {
			options.InsecureSkipVerify = true // #nosec G402 -- gated by TLSConfig.Verify
		}
		if len(tlsConfig.CAData) > 0 || (creds != nil && len(creds.CAData) > 0) {
			// CA already configured above
		} else {
			options.CAFile = *tlsConfig.CertPath + "/ca.pem"
		}

		tlsConf, err := tlsconfig.Client(options)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create TLS config from cert path")
		}

		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		}, nil
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConf,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}, nil
}

// GetProviderConfig returns the ProviderConfig for the given managed resource.
func GetProviderConfig(ctx context.Context, k8s k8sclient.Client, mg resource.Managed) (*v1beta1.ProviderConfig, error) {
	// Get provider config reference from the managed resource's ResourceSpec
	var pcRef *xpv1.ProviderConfigReference

	// Type assert to extract the ProviderConfigReference from the managed resource
	switch mr := mg.(type) {
	case interface {
		GetProviderConfigReference() *xpv1.ProviderConfigReference
	}:
		pcRef = mr.GetProviderConfigReference()
	default:
		return nil, errors.New(errNoProviderConfig)
	}

	if pcRef == nil {
		return nil, errors.New(errNoProviderConfig)
	}

	pc := &v1beta1.ProviderConfig{}
	if err := k8s.Get(ctx, ktypes.NamespacedName{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	return pc, nil
}

// TrackProviderConfigUsage tracks the usage of a ProviderConfig.
func TrackProviderConfigUsage(ctx context.Context, k8s k8sclient.Client, mg resource.Managed) error {
	// Get provider config reference from the managed resource's ResourceSpec
	var pcRef *xpv1.ProviderConfigReference

	// Type assert to extract the ProviderConfigReference from the managed resource
	switch mr := mg.(type) {
	case interface {
		GetProviderConfigReference() *xpv1.ProviderConfigReference
	}:
		pcRef = mr.GetProviderConfigReference()
	default:
		return errors.New(errNoProviderConfig)
	}

	if pcRef == nil {
		return errors.New(errNoProviderConfig)
	}

	usage := &v1beta1.ProviderConfigUsage{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", pcRef.Name, mg.GetUID()),
		},
		ProviderConfigUsage: xpv1.ProviderConfigUsage{
			ProviderConfigReference: xpv1.Reference{Name: pcRef.Name},
			ResourceReference: xpv1.TypedReference{
				APIVersion: mg.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       mg.GetObjectKind().GroupVersionKind().Kind,
				Name:       mg.GetName(),
				UID:        mg.GetUID(),
			},
		},
	}

	return k8s.Create(ctx, usage)
}

// DockerCredentials represents credentials for Docker daemon connection.
type DockerCredentials struct {
	// TLS certificate data
	CAData   []byte `json:"ca,omitempty"`
	CertData []byte `json:"cert,omitempty"`
	KeyData  []byte `json:"key,omitempty"`

	// Registry authentication
	RegistryAuths map[string]RegistryAuth `json:"auths,omitempty"`
}

// RegistryAuth represents authentication for a Docker registry.
type RegistryAuth struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Email         string `json:"email,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}

// ExtractCredentials extracts credentials from the ProviderConfig.
func ExtractCredentials(ctx context.Context, k8s k8sclient.Client, pc *v1beta1.ProviderConfig) (*DockerCredentials, error) {
	if pc.Spec.Credentials.SecretRef == nil {
		return &DockerCredentials{}, nil
	}

	// Validate credentials source (only when SecretRef is provided)
	if pc.Spec.Credentials.Source != xpv1.CredentialsSourceSecret && pc.Spec.Credentials.Source != "" {
		return nil, errors.Errorf("credentials source %s is not currently supported", pc.Spec.Credentials.Source)
	}

	secret := &corev1.Secret{}
	secretKey := ktypes.NamespacedName{
		Namespace: pc.Spec.Credentials.SecretRef.Namespace,
		Name:      pc.Spec.Credentials.SecretRef.Name,
	}

	if err := k8s.Get(ctx, secretKey, secret); err != nil {
		return nil, errors.Wrap(err, "cannot get secret")
	}

	creds := &DockerCredentials{}

	// Extract TLS certificates if present
	if caData, ok := secret.Data["ca"]; ok {
		creds.CAData = caData
	}
	if certData, ok := secret.Data["cert"]; ok {
		creds.CertData = certData
	}
	if keyData, ok := secret.Data["key"]; ok {
		creds.KeyData = keyData
	}

	// Extract registry authentication if present
	if authsData, ok := secret.Data["auths"]; ok {
		auths := make(map[string]RegistryAuth)
		if err := json.Unmarshal(authsData, &auths); err != nil {
			return nil, errors.Wrap(err, errUnmarshalCredentials)
		}
		creds.RegistryAuths = auths
	}

	return creds, nil
}
