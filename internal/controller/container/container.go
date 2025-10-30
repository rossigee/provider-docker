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
	"fmt"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
	"github.com/rossigee/provider-docker/apis/container/v1beta1"
	"github.com/rossigee/provider-docker/internal/clients"
)

const (
	errNotContainer = "managed resource is not a Container custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errNewClient    = "cannot create new Docker client"
	errCreateFailed = "cannot create container"
	errDeleteFailed = "cannot delete container"
	errUpdateFailed = "cannot update container"

	// AnnotationKeyExternalName is the annotation key for external names
	AnnotationKeyExternalName = "crossplane.io/external-name"
)

// ContainerConfigBuilder builds Docker container configuration from Crossplane resources.
type ContainerConfigBuilder interface {
	BuildContainerConfig(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, error)
}

// defaultContainerConfigBuilder implements ContainerConfigBuilder.
type defaultContainerConfigBuilder struct{}

// NewContainerConfigBuilder creates a new ContainerConfigBuilder.
func NewContainerConfigBuilder() ContainerConfigBuilder {
	return &defaultContainerConfigBuilder{}
}

// Setup adds a controller that reconciles Container managed resources.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	name := managed.ControllerName(v1alpha1.ContainerGroupKind.Kind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ContainerGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:   mgr.GetClient(),
			usage:  resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			logger: o.Logger,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Container{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube   client.Client
	usage  resource.Tracker
	logger logging.Logger
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.Container); !ok {
		return nil, errors.New(errNotContainer)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	dockerClient, err := clients.NewDockerClient(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		client:        dockerClient,
		configBuilder: NewContainerConfigBuilder(),
		logger:        c.logger,
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client        clients.DockerClient
	configBuilder ContainerConfigBuilder
	logger        logging.Logger
}

// Disconnect closes any connection to the external resource.
func (c *external) Disconnect(ctx context.Context) error {
	return c.client.Close()
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotContainer)
	}

	// Get the container ID from external name or status
	containerID := cr.GetAnnotations()[AnnotationKeyExternalName]
	if containerID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Inspect the container
	containerInfo, err := c.client.ContainerInspect(ctx, containerID)
	if err != nil {
		// If container not found, it doesn't exist
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot inspect container")
	}

	// Update the status with observed state
	c.updateStatus(cr, &containerInfo)

	// Check if container is up to date
	upToDate := c.isUpToDate(cr, &containerInfo)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotContainer)
	}

	c.logger.Debug("Creating container", "container", cr.Name)

	// Convert Container spec to Docker API types
	containerConfig, hostConfig, networkingConfig, platform, err := c.configBuilder.BuildContainerConfig(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot build container configuration")
	}

	// Create the container
	containerName := ""
	if cr.Spec.ForProvider.Name != nil {
		containerName = *cr.Spec.ForProvider.Name
	}
	response, err := c.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, platform, containerName)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateFailed)
	}

	// Start the container if requested
	if cr.Spec.ForProvider.StartOnCreate == nil || *cr.Spec.ForProvider.StartOnCreate {
		if err := c.client.ContainerStart(ctx, response.ID, container.StartOptions{}); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot start container")
		}
	}

	// Set the external name annotation
	annotations := cr.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationKeyExternalName] = response.ID
	cr.SetAnnotations(annotations)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotContainer)
	}

	// Container updates are not implemented as they require recreation
	// due to Docker API limitations. Most container config changes
	// require stopping and recreating the container.
	return managed.ExternalUpdate{}, errors.New("container update is not implemented")
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotContainer)
	}

	containerID := cr.GetAnnotations()[AnnotationKeyExternalName]
	if containerID == "" {
		return managed.ExternalDelete{}, nil // Nothing to delete
	}

	c.logger.Debug("Deleting container", "container", cr.Name, "id", containerID)

	// Stop the container first
	timeout := 10
	if err := c.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		if !isNotFound(err) {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot stop container")
		}
	}

	// Remove the container
	if err := c.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		if !isNotFound(err) {
			return managed.ExternalDelete{}, errors.Wrap(err, errDeleteFailed)
		}
	}

	return managed.ExternalDelete{}, nil
}

// Helper functions

// BuildContainerConfig implements ContainerConfigBuilder interface.
func (b *defaultContainerConfigBuilder) BuildContainerConfig(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, error) {
	config := &container.Config{
		Image: cr.Spec.ForProvider.Image,
	}

	// Command and args
	if len(cr.Spec.ForProvider.Command) > 0 {
		config.Cmd = cr.Spec.ForProvider.Command
	}

	if len(cr.Spec.ForProvider.Args) > 0 {
		if config.Cmd == nil {
			config.Cmd = cr.Spec.ForProvider.Args
		} else {
			config.Cmd = append(config.Cmd, cr.Spec.ForProvider.Args...)
		}
	}

	// Environment variables
	if len(cr.Spec.ForProvider.Environment) > 0 {
		env, err := b.buildEnvironmentConfiguration(cr.Spec.ForProvider.Environment)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "cannot build environment configuration")
		}
		config.Env = env
	}

	// Labels
	if len(cr.Spec.ForProvider.Labels) > 0 {
		config.Labels = cr.Spec.ForProvider.Labels
	}

	// Working directory
	if cr.Spec.ForProvider.WorkingDir != nil {
		config.WorkingDir = *cr.Spec.ForProvider.WorkingDir
	}

	// User
	if cr.Spec.ForProvider.User != nil {
		config.User = *cr.Spec.ForProvider.User
	}

	// Hostname
	if cr.Spec.ForProvider.Hostname != nil {
		config.Hostname = *cr.Spec.ForProvider.Hostname
	}

	// Exposed ports
	exposedPorts, portBindings, err := b.buildPortConfiguration(cr.Spec.ForProvider.Ports)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "cannot build port configuration")
	}
	config.ExposedPorts = exposedPorts

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
	}

	// Restart policy
	if cr.Spec.ForProvider.RestartPolicy != nil {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(*cr.Spec.ForProvider.RestartPolicy),
		}
		if cr.Spec.ForProvider.MaximumRetryCount != nil {
			hostConfig.RestartPolicy.MaximumRetryCount = *cr.Spec.ForProvider.MaximumRetryCount
		}
	}

	// Network mode
	if cr.Spec.ForProvider.NetworkMode != nil {
		hostConfig.NetworkMode = container.NetworkMode(*cr.Spec.ForProvider.NetworkMode)
	}

	// Volume mounts
	binds, mounts, err := b.buildVolumeConfiguration(cr.Spec.ForProvider.Volumes)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "cannot build volume configuration")
	}
	hostConfig.Binds = binds
	hostConfig.Mounts = mounts

	// Network attachments
	networkingConfig, err := b.buildNetworkConfiguration(cr.Spec.ForProvider.Networks)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "cannot build network configuration")
	}

	// Security context
	err = b.buildSecurityConfiguration(cr.Spec.ForProvider.SecurityContext, config, hostConfig)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "cannot build security configuration")
	}

	// Health checks
	err = b.buildHealthCheckConfiguration(cr.Spec.ForProvider.HealthCheck, config)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "cannot build health check configuration")
	}

	return config, hostConfig, networkingConfig, nil, nil
}

// buildPortConfiguration builds Docker port configuration from Crossplane port specs.
func (b *defaultContainerConfigBuilder) buildPortConfiguration(ports []v1alpha1.PortSpec) (nat.PortSet, nat.PortMap, error) {
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)

	for _, portSpec := range ports {
		protocol := "tcp"
		if portSpec.Protocol != nil {
			protocol = strings.ToLower(*portSpec.Protocol)
		}

		// Create the Docker port format
		dockerPort := nat.Port(fmt.Sprintf("%d/%s", portSpec.ContainerPort, protocol))
		exposedPorts[dockerPort] = struct{}{}

		// Add port binding if host port is specified
		if portSpec.HostPort != nil {
			hostBinding := nat.PortBinding{
				HostPort: strconv.Itoa(int(*portSpec.HostPort)),
			}
			if portSpec.HostIP != nil {
				hostBinding.HostIP = *portSpec.HostIP
			}
			portBindings[dockerPort] = []nat.PortBinding{hostBinding}
		}
	}

	return exposedPorts, portBindings, nil
}

// buildVolumeConfiguration builds Docker volume configuration from Crossplane volume specs.
func (b *defaultContainerConfigBuilder) buildVolumeConfiguration(volumes []v1alpha1.VolumeMount) ([]string, []mount.Mount, error) {
	binds := make([]string, 0)
	mounts := make([]mount.Mount, 0)

	for _, volumeSpec := range volumes {
		readOnly := false
		if volumeSpec.ReadOnly != nil {
			readOnly = *volumeSpec.ReadOnly
		}

		// Handle different volume source types
		switch {
		case volumeSpec.VolumeSource.HostPath != nil:
			// Host path mount using binds
			bind := fmt.Sprintf("%s:%s", volumeSpec.VolumeSource.HostPath.Path, volumeSpec.MountPath)
			if readOnly {
				bind += ":ro"
			}
			binds = append(binds, bind)

		case volumeSpec.VolumeSource.Volume != nil:
			// Named Docker volume using mounts
			mountSpec := mount.Mount{
				Type:     mount.TypeVolume,
				Source:   volumeSpec.VolumeSource.Volume.VolumeName,
				Target:   volumeSpec.MountPath,
				ReadOnly: readOnly,
			}
			mounts = append(mounts, mountSpec)

		case volumeSpec.VolumeSource.Bind != nil:
			// Bind mount using mounts with propagation options
			mountSpec := mount.Mount{
				Type:     mount.TypeBind,
				Source:   volumeSpec.VolumeSource.Bind.SourcePath,
				Target:   volumeSpec.MountPath,
				ReadOnly: readOnly,
			}

			// Set bind propagation if specified
			if volumeSpec.VolumeSource.Bind.Propagation != nil {
				switch *volumeSpec.VolumeSource.Bind.Propagation {
				case "private":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationPrivate}
				case "rprivate":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationRPrivate}
				case "shared":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationShared}
				case "rshared":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationRShared}
				case "slave":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationSlave}
				case "rslave":
					mountSpec.BindOptions = &mount.BindOptions{Propagation: mount.PropagationRSlave}
				}
			}
			mounts = append(mounts, mountSpec)

		case volumeSpec.VolumeSource.EmptyDir != nil:
			// EmptyDir using tmpfs mount
			mountSpec := mount.Mount{
				Type:     mount.TypeTmpfs,
				Target:   volumeSpec.MountPath,
				ReadOnly: readOnly,
			}

			// Set size limit if specified
			if volumeSpec.VolumeSource.EmptyDir.SizeLimit != nil {
				if mountSpec.TmpfsOptions == nil {
					mountSpec.TmpfsOptions = &mount.TmpfsOptions{}
				}
				mountSpec.TmpfsOptions.SizeBytes, _ = parseByteSize(*volumeSpec.VolumeSource.EmptyDir.SizeLimit)
			}
			mounts = append(mounts, mountSpec)

		case volumeSpec.VolumeSource.Secret != nil || volumeSpec.VolumeSource.ConfigMap != nil:
			// Secret and ConfigMap mounts will need special handling via init containers
			// or pre-populated volumes. For now, we'll skip these as they require
			// Kubernetes integration.
			continue

		default:
			return nil, nil, errors.Errorf("unsupported volume source type for volume %s", volumeSpec.Name)
		}
	}

	return binds, mounts, nil
}

// buildNetworkConfiguration builds Docker network configuration from Crossplane network specs.
func (b *defaultContainerConfigBuilder) buildNetworkConfiguration(networks []v1alpha1.NetworkAttachment) (*network.NetworkingConfig, error) {
	if len(networks) == 0 {
		return nil, nil
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}

	for _, networkSpec := range networks {
		endpointSettings := &network.EndpointSettings{}

		// Set IP address if specified
		if networkSpec.IPAddress != nil {
			endpointSettings.IPAMConfig = &network.EndpointIPAMConfig{
				IPv4Address: *networkSpec.IPAddress,
			}
		}

		// Set IPv6 address if specified
		if networkSpec.IPv6Address != nil {
			if endpointSettings.IPAMConfig == nil {
				endpointSettings.IPAMConfig = &network.EndpointIPAMConfig{}
			}
			endpointSettings.IPAMConfig.IPv6Address = *networkSpec.IPv6Address
		}

		// Set aliases if specified
		if len(networkSpec.Aliases) > 0 {
			endpointSettings.Aliases = networkSpec.Aliases
		}

		// Set links if specified (legacy feature)
		if len(networkSpec.Links) > 0 {
			endpointSettings.Links = networkSpec.Links
		}

		networkingConfig.EndpointsConfig[networkSpec.Name] = endpointSettings
	}

	return networkingConfig, nil
}

// buildSecurityConfiguration builds Docker security configuration from Crossplane security context.
func (b *defaultContainerConfigBuilder) buildSecurityConfiguration(securityContext *v1alpha1.SecurityContext, config *container.Config, hostConfig *container.HostConfig) error {
	if securityContext == nil {
		return nil
	}

	// RunAsUser - set the user ID
	if securityContext.RunAsUser != nil {
		config.User = strconv.FormatInt(*securityContext.RunAsUser, 10)
	}

	// RunAsGroup - set the group ID (combine with user if both specified)
	if securityContext.RunAsGroup != nil {
		if config.User != "" {
			config.User = fmt.Sprintf("%s:%d", config.User, *securityContext.RunAsGroup)
		} else {
			config.User = fmt.Sprintf(":%d", *securityContext.RunAsGroup)
		}
	}

	// ReadOnlyRootFilesystem
	if securityContext.ReadOnlyRootFilesystem != nil && *securityContext.ReadOnlyRootFilesystem {
		hostConfig.ReadonlyRootfs = true
	}

	// Privileged mode
	if securityContext.AllowPrivilegeEscalation != nil {
		// Docker doesn't have a direct equivalent, but we can use privileged mode
		// This is a conservative mapping - if privilege escalation is explicitly disabled,
		// we ensure the container is not privileged
		if !*securityContext.AllowPrivilegeEscalation {
			hostConfig.Privileged = false
		}
	}

	// Capabilities
	if securityContext.Capabilities != nil {
		if len(securityContext.Capabilities.Add) > 0 {
			hostConfig.CapAdd = securityContext.Capabilities.Add
		}
		if len(securityContext.Capabilities.Drop) > 0 {
			hostConfig.CapDrop = securityContext.Capabilities.Drop
		}
	}

	// Security options for SELinux, AppArmor, and Seccomp
	var securityOpts []string

	// SELinux options
	if securityContext.SELinuxOptions != nil {
		var selinuxLabel []string
		if securityContext.SELinuxOptions.User != nil {
			selinuxLabel = append(selinuxLabel, "user:"+*securityContext.SELinuxOptions.User)
		}
		if securityContext.SELinuxOptions.Role != nil {
			selinuxLabel = append(selinuxLabel, "role:"+*securityContext.SELinuxOptions.Role)
		}
		if securityContext.SELinuxOptions.Type != nil {
			selinuxLabel = append(selinuxLabel, "type:"+*securityContext.SELinuxOptions.Type)
		}
		if securityContext.SELinuxOptions.Level != nil {
			selinuxLabel = append(selinuxLabel, "level:"+*securityContext.SELinuxOptions.Level)
		}
		if len(selinuxLabel) > 0 {
			securityOpts = append(securityOpts, "label:"+strings.Join(selinuxLabel, ","))
		}
	}

	// Seccomp profile
	if securityContext.SeccompProfile != nil {
		switch securityContext.SeccompProfile.Type {
		case "RuntimeDefault":
			securityOpts = append(securityOpts, "seccomp:runtime/default")
		case "Unconfined":
			securityOpts = append(securityOpts, "seccomp:unconfined")
		case "Localhost":
			if securityContext.SeccompProfile.LocalhostProfile != nil {
				securityOpts = append(securityOpts, "seccomp:"+*securityContext.SeccompProfile.LocalhostProfile)
			}
		}
	}

	// AppArmor profile
	if securityContext.AppArmorProfile != nil {
		switch securityContext.AppArmorProfile.Type {
		case "RuntimeDefault":
			securityOpts = append(securityOpts, "apparmor:docker-default")
		case "Unconfined":
			securityOpts = append(securityOpts, "apparmor:unconfined")
		case "Localhost":
			if securityContext.AppArmorProfile.LocalhostProfile != nil {
				securityOpts = append(securityOpts, "apparmor:"+*securityContext.AppArmorProfile.LocalhostProfile)
			}
		}
	}

	// Apply security options
	if len(securityOpts) > 0 {
		hostConfig.SecurityOpt = securityOpts
	}

	// RunAsNonRoot validation - we can't enforce this at container creation time
	// but we can add it as a comment or warning in logs
	// Docker will enforce this if the user is set to a non-root user
	_ = securityContext.RunAsNonRoot // We acknowledge the setting but don't need special handling

	return nil
}

// buildHealthCheckConfiguration builds Docker health check configuration from Crossplane health check spec.
func (b *defaultContainerConfigBuilder) buildHealthCheckConfiguration(healthCheck *v1alpha1.HealthCheck, config *container.Config) error {
	if healthCheck == nil {
		return nil
	}

	// Validate that test command is provided
	if len(healthCheck.Test) == 0 {
		return errors.New("health check test command is required")
	}

	dockerHealthCheck := &container.HealthConfig{
		Test: healthCheck.Test,
	}

	// Set interval (default is 30s if not specified)
	if healthCheck.Interval != nil {
		interval, err := parseDuration(healthCheck.Interval.Duration.String())
		if err != nil {
			return errors.Wrap(err, "cannot parse health check interval")
		}
		dockerHealthCheck.Interval = interval
	}

	// Set timeout (default is 30s if not specified)
	if healthCheck.Timeout != nil {
		timeout, err := parseDuration(healthCheck.Timeout.Duration.String())
		if err != nil {
			return errors.Wrap(err, "cannot parse health check timeout")
		}
		dockerHealthCheck.Timeout = timeout
	}

	// Set start period (default is 0s if not specified)
	if healthCheck.StartPeriod != nil {
		startPeriod, err := parseDuration(healthCheck.StartPeriod.Duration.String())
		if err != nil {
			return errors.Wrap(err, "cannot parse health check start period")
		}
		dockerHealthCheck.StartPeriod = startPeriod
	}

	// Set retries (default is 3 if not specified)
	if healthCheck.Retries != nil {
		dockerHealthCheck.Retries = *healthCheck.Retries
	}

	config.Healthcheck = dockerHealthCheck
	return nil
}

// parseDuration parses a duration string into time.Duration.
func parseDuration(durationStr string) (time.Duration, error) {
	// Handle common Kubernetes duration formats
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid duration format: %s", durationStr)
	}
	return duration, nil
}

// parseByteSize parses a size string like "100Mi", "1Gi" into bytes.
func parseByteSize(sizeStr string) (int64, error) {
	// Simple implementation - in production, use a proper parsing library
	if strings.HasSuffix(sizeStr, "Mi") {
		val, err := strconv.ParseInt(strings.TrimSuffix(sizeStr, "Mi"), 10, 64)
		if err != nil {
			return 0, err
		}
		return val * 1024 * 1024, nil
	}
	if strings.HasSuffix(sizeStr, "Gi") {
		val, err := strconv.ParseInt(strings.TrimSuffix(sizeStr, "Gi"), 10, 64)
		if err != nil {
			return 0, err
		}
		return val * 1024 * 1024 * 1024, nil
	}
	return strconv.ParseInt(sizeStr, 10, 64)
}

func (c *external) updateStatus(cr *v1alpha1.Container, containerInfo *container.InspectResponse) {
	// Initialize the observation
	observation := v1alpha1.ContainerObservation{}

	// Basic container information
	observation.ID = containerInfo.ID
	observation.Name = containerInfo.Name

	// Container state
	observation.State = v1alpha1.ContainerState{
		Status:     containerInfo.State.Status,
		Running:    containerInfo.State.Running,
		Paused:     containerInfo.State.Paused,
		Restarting: containerInfo.State.Restarting,
		OOMKilled:  containerInfo.State.OOMKilled,
		Dead:       containerInfo.State.Dead,
		Pid:        int64(containerInfo.State.Pid),
		ExitCode:   int64(containerInfo.State.ExitCode),
		Error:      containerInfo.State.Error,
	}

	// Container timestamps
	if containerInfo.Created != "" {
		if createdTime, err := time.Parse(time.RFC3339Nano, containerInfo.Created); err == nil {
			created := metav1.NewTime(createdTime)
			observation.Created = &created
		}
	}

	if containerInfo.State.StartedAt != "" {
		if startedTime, err := time.Parse(time.RFC3339Nano, containerInfo.State.StartedAt); err == nil {
			started := metav1.NewTime(startedTime)
			observation.Started = &started
			observation.State.StartedAt = &started
		}
	}

	if containerInfo.State.FinishedAt != "" {
		if finishedTime, err := time.Parse(time.RFC3339Nano, containerInfo.State.FinishedAt); err == nil {
			finished := metav1.NewTime(finishedTime)
			observation.State.FinishedAt = &finished
		}
	}

	// Image information
	observation.Image = v1alpha1.ContainerImage{
		Name: containerInfo.Config.Image,
		ID:   containerInfo.Image,
	}

	// Port mappings
	observation.Ports = c.buildObservedPorts(containerInfo)

	// Network information
	observation.Networks = c.buildObservedNetworks(containerInfo)

	// Health check information
	if containerInfo.State.Health != nil {
		observation.State.Health = c.buildObservedHealth(containerInfo.State.Health)
	}

	// Update the status
	cr.Status.AtProvider = observation

	// Set condition based on container state
	if containerInfo.State.Running {
		cr.SetConditions(xpv1.Available())
	} else if containerInfo.State.Dead {
		cr.SetConditions(xpv1.Unavailable().WithMessage("Container is dead"))
	} else if containerInfo.State.OOMKilled {
		cr.SetConditions(xpv1.Unavailable().WithMessage("Container was killed due to OOM"))
	} else if containerInfo.State.Error != "" {
		cr.SetConditions(xpv1.Unavailable().WithMessage(fmt.Sprintf("Container error: %s", containerInfo.State.Error)))
	} else {
		cr.SetConditions(xpv1.Unavailable().WithMessage(fmt.Sprintf("Container is %s", containerInfo.State.Status)))
	}
}

func (c *external) isUpToDate(cr *v1alpha1.Container, containerInfo *container.InspectResponse) bool {
	// Check if the container is based on the desired image
	if containerInfo.Config.Image != cr.Spec.ForProvider.Image {
		if c.logger != nil {
			c.logger.Debug("Container image mismatch", "expected", cr.Spec.ForProvider.Image, "actual", containerInfo.Config.Image)
		}
		return false
	}

	// Check restart policy
	if cr.Spec.ForProvider.RestartPolicy != nil {
		if containerInfo.HostConfig == nil {
			if c.logger != nil {
				c.logger.Debug("Container HostConfig is nil, cannot check restart policy")
			}
			return false
		}
		expectedPolicy := *cr.Spec.ForProvider.RestartPolicy
		actualPolicy := string(containerInfo.HostConfig.RestartPolicy.Name)
		if expectedPolicy != actualPolicy {
			if c.logger != nil {
				c.logger.Debug("Container restart policy mismatch", "expected", expectedPolicy, "actual", actualPolicy)
			}
			return false
		}

		// Check retry count for on-failure policy
		if expectedPolicy == "on-failure" && cr.Spec.ForProvider.MaximumRetryCount != nil {
			if containerInfo.HostConfig.RestartPolicy.MaximumRetryCount != *cr.Spec.ForProvider.MaximumRetryCount {
				if c.logger != nil {
					c.logger.Debug("Container retry count mismatch",
						"expected", *cr.Spec.ForProvider.MaximumRetryCount,
						"actual", containerInfo.HostConfig.RestartPolicy.MaximumRetryCount)
				}
				return false
			}
		}
	}

	// Check environment variables
	if !c.isEnvironmentUpToDate(cr.Spec.ForProvider.Environment, containerInfo.Config.Env) {
		if c.logger != nil {
			c.logger.Debug("Container environment variables mismatch")
		}
		return false
	}

	// Check labels
	if !c.isLabelsUpToDate(cr.Spec.ForProvider.Labels, containerInfo.Config.Labels) {
		if c.logger != nil {
			c.logger.Debug("Container labels mismatch")
		}
		return false
	}

	// Check privileged mode
	if cr.Spec.ForProvider.Privileged != nil {
		if containerInfo.HostConfig == nil {
			if c.logger != nil {
				c.logger.Debug("Container HostConfig is nil, cannot check privileged mode")
			}
			return false
		}
		if containerInfo.HostConfig.Privileged != *cr.Spec.ForProvider.Privileged {
			if c.logger != nil {
				c.logger.Debug("Container privileged mode mismatch",
					"expected", *cr.Spec.ForProvider.Privileged,
					"actual", containerInfo.HostConfig.Privileged)
			}
			return false
		}
	}

	// Check if container is healthy (if health checks are configured)
	if containerInfo.State != nil && containerInfo.State.Health != nil {
		if containerInfo.State.Health.Status == "unhealthy" {
			if c.logger != nil {
				c.logger.Debug("Container is unhealthy")
			}
			return false
		}
	}

	// If all checks pass, the container configuration is up to date
	// Note: We don't require the container to be running to be considered "up to date"
	// since that's a separate concern from configuration matching
	return true
}

// isEnvironmentUpToDate compares expected environment variables with actual container environment.
func (c *external) isEnvironmentUpToDate(expectedEnv []v1alpha1.EnvVar, actualEnv []string) bool {
	// Convert actual environment to a map for easier comparison
	actualEnvMap := make(map[string]string)
	for _, env := range actualEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			actualEnvMap[parts[0]] = parts[1]
		} else if len(parts) == 1 {
			actualEnvMap[parts[0]] = ""
		}
	}

	// Check if expected environment variables match
	for _, envVar := range expectedEnv {
		var expectedValue string

		if envVar.Value != nil {
			expectedValue = *envVar.Value
		} else if envVar.ValueFrom != nil {
			// For valueFrom, we would need to resolve the value
			// For now, skip this comparison as it requires Kubernetes client
			continue
		} else {
			// Invalid env var specification
			continue
		}

		actualValue, exists := actualEnvMap[envVar.Name]
		if !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// isLabelsUpToDate compares expected labels with actual container labels.
func (c *external) isLabelsUpToDate(expectedLabels map[string]string, actualLabels map[string]string) bool {
	if expectedLabels == nil && actualLabels == nil {
		return true
	}

	// Check if all expected labels match
	for key, expectedValue := range expectedLabels {
		actualValue, exists := actualLabels[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// buildEnvironmentConfiguration builds environment variables from Crossplane env var specs.
func (b *defaultContainerConfigBuilder) buildEnvironmentConfiguration(envVars []v1alpha1.EnvVar) ([]string, error) {
	env := make([]string, 0, len(envVars))

	for _, envVar := range envVars {
		var value string
		var err error

		// Handle direct value
		if envVar.Value != nil {
			value = *envVar.Value
		} else if envVar.ValueFrom != nil {
			// Handle valueFrom - ConfigMap or Secret references
			value, err = b.resolveEnvVarValue(envVar.Name, envVar.ValueFrom)
			if err != nil {
				// If optional and not found, skip this env var
				if b.isEnvVarOptional(envVar.ValueFrom) && isNotFound(err) {
					continue
				}
				return nil, errors.Wrapf(err, "cannot resolve value for environment variable %s", envVar.Name)
			}
		} else {
			// Environment variable with no value or valueFrom - invalid
			return nil, errors.Errorf("environment variable %s has no value or valueFrom specified", envVar.Name)
		}

		env = append(env, envVar.Name+"="+value)
	}

	return env, nil
}

// resolveEnvVarValue resolves environment variable value from ConfigMap or Secret.
func (b *defaultContainerConfigBuilder) resolveEnvVarValue(envVarName string, valueFrom *v1alpha1.EnvVarSource) (string, error) {
	// NOTE: This is a placeholder implementation for MVP
	// In a real implementation, we would need:
	// 1. Access to Kubernetes client
	// 2. Namespace context (same as Container resource)
	// 3. Proper error handling for not found vs other errors

	if valueFrom.ConfigMapKeyRef != nil {
		// NOTE: ConfigMap value resolution needs Kubernetes client
		// Implementation: mgr.GetClient(), same namespace, optional handling
		return "", errors.New("ConfigMap valueFrom not yet implemented - requires Kubernetes client integration")
	}

	if valueFrom.SecretKeyRef != nil {
		// NOTE: Secret value resolution needs Kubernetes client
		// Implementation: mgr.GetClient(), same namespace, optional handling
		return "", errors.New("Secret valueFrom not yet implemented - requires Kubernetes client integration")
	}

	return "", errors.New("unknown valueFrom source")
}

// isEnvVarOptional checks if an environment variable source is optional.
func (b *defaultContainerConfigBuilder) isEnvVarOptional(valueFrom *v1alpha1.EnvVarSource) bool {
	if valueFrom.ConfigMapKeyRef != nil && valueFrom.ConfigMapKeyRef.Optional != nil {
		return *valueFrom.ConfigMapKeyRef.Optional
	}
	if valueFrom.SecretKeyRef != nil && valueFrom.SecretKeyRef.Optional != nil {
		return *valueFrom.SecretKeyRef.Optional
	}
	return false
}

// buildObservedPorts builds the observed port mappings from Docker container info.
func (c *external) buildObservedPorts(containerInfo *container.InspectResponse) []v1alpha1.ContainerPort {
	var ports []v1alpha1.ContainerPort

	if containerInfo.NetworkSettings == nil {
		return []v1alpha1.ContainerPort{}
	}

	// If no ports are defined, return empty slice not nil
	if len(containerInfo.NetworkSettings.Ports) == 0 {
		return []v1alpha1.ContainerPort{}
	}

	for port, bindings := range containerInfo.NetworkSettings.Ports {
		if len(bindings) == 0 {
			// Port exposed but not bound to host
			ports = append(ports, v1alpha1.ContainerPort{
				PrivatePort: int32(port.Int()),
				Type:        string(port.Proto()),
			})
			continue
		}

		// Port bound to host
		for _, binding := range bindings {
			containerPort := v1alpha1.ContainerPort{
				PrivatePort: int32(port.Int()),
				Type:        string(port.Proto()),
				IP:          binding.HostIP,
			}

			if binding.HostPort != "" {
				if hostPort, err := strconv.ParseInt(binding.HostPort, 10, 32); err == nil {
					containerPort.PublicPort = int32(hostPort)
				}
			}

			ports = append(ports, containerPort)
		}
	}

	return ports
}

// buildObservedNetworks builds the observed network attachments from Docker container info.
func (c *external) buildObservedNetworks(containerInfo *container.InspectResponse) map[string]v1alpha1.NetworkInfo {
	networks := make(map[string]v1alpha1.NetworkInfo)

	if containerInfo.NetworkSettings == nil {
		return networks
	}

	for networkName, networkSettings := range containerInfo.NetworkSettings.Networks {
		networkInfo := v1alpha1.NetworkInfo{
			NetworkID:           networkSettings.NetworkID,
			EndpointID:          networkSettings.EndpointID,
			Gateway:             networkSettings.Gateway,
			IPAddress:           networkSettings.IPAddress,
			IPPrefixLen:         int32(networkSettings.IPPrefixLen),
			IPv6Gateway:         networkSettings.IPv6Gateway,
			GlobalIPv6Address:   networkSettings.GlobalIPv6Address,
			GlobalIPv6PrefixLen: int32(networkSettings.GlobalIPv6PrefixLen),
			MacAddress:          networkSettings.MacAddress,
		}
		networks[networkName] = networkInfo
	}

	return networks
}

// buildObservedHealth builds the observed health status from Docker health info.
func (c *external) buildObservedHealth(health *container.Health) *v1alpha1.ContainerHealth {
	if health == nil {
		return nil
	}

	containerHealth := &v1alpha1.ContainerHealth{
		Status:        health.Status,
		FailingStreak: health.FailingStreak,
	}

	// Convert health check logs
	if len(health.Log) > 0 {
		containerHealth.Log = make([]v1alpha1.HealthCheckResult, len(health.Log))
		for i, log := range health.Log {
			result := v1alpha1.HealthCheckResult{
				ExitCode: log.ExitCode,
				Output:   log.Output,
			}

			if !log.Start.IsZero() {
				start := metav1.NewTime(log.Start)
				result.Start = &start
			}

			if !log.End.IsZero() {
				end := metav1.NewTime(log.End)
				result.End = &end
			}

			containerHealth.Log[i] = result
		}
	}

	return containerHealth
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check for Docker API not found errors
	errorMessage := strings.ToLower(err.Error())
	return strings.Contains(errorMessage, "not found") ||
		strings.Contains(errorMessage, "no such container")
}

// SetupV1Beta1 creates a controller for the v1beta1 (namespaced) Container resource.
func SetupV1Beta1(mgr ctrl.Manager, o xpcontroller.Options) error {
	name := managed.ControllerName(v1beta1.ContainerGroupKind.Kind + "-v1beta1")

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.ContainerGroupVersionKind),
		managed.WithExternalConnecter(&v1beta1Connector{
			kube:   mgr.GetClient(),
			usage:  resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			logger: o.Logger,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.Container{}).
		Complete(r)
}

// v1beta1Connector creates external connectors for v1beta1 Container resources.
type v1beta1Connector struct {
	kube   client.Client
	usage  resource.Tracker
	logger logging.Logger
}

// Connect returns an ExternalClient capable of interacting with Docker API.
func (c *v1beta1Connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.Container)
	if !ok {
		return nil, errors.New(errNotContainer)
	}

	// Convert v1beta1 to v1alpha1 for business logic compatibility
	v1alpha1Container := convertV1Beta1ToV1Alpha1(cr)

	// Create Docker client (using the managed resource interface)
	dockerClient, err := clients.NewDockerClient(ctx, c.kube, v1alpha1Container)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &v1beta1External{
		external: external{
			client:        dockerClient,
			configBuilder: &defaultContainerConfigBuilder{},
			logger:        c.logger,
		},
		v1beta1Container:  cr,
		v1alpha1Container: v1alpha1Container,
	}, nil
}

// v1beta1External wraps the v1alpha1 external client for v1beta1 compatibility.
type v1beta1External struct {
	external
	v1beta1Container  *v1beta1.Container
	v1alpha1Container *v1alpha1.Container
}

// Observe observes the external resource.
func (e *v1beta1External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	// Use the v1alpha1 logic but update the v1beta1 resource
	obs, err := e.external.Observe(ctx, e.v1alpha1Container)
	if err != nil {
		return obs, err
	}

	// Copy status from v1alpha1 to v1beta1
	if obs.ResourceExists {
		e.v1beta1Container.Status.AtProvider = v1beta1.ContainerObservation(e.v1alpha1Container.Status.AtProvider)
		e.v1beta1Container.Status.ResourceStatus = e.v1alpha1Container.Status.ResourceStatus
	}

	return obs, nil
}

// Create creates the external resource.
func (e *v1beta1External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	return e.external.Create(ctx, e.v1alpha1Container)
}

// Update updates the external resource.
func (e *v1beta1External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	return e.external.Update(ctx, e.v1alpha1Container)
}

// Delete deletes the external resource.
func (e *v1beta1External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	return e.external.Delete(ctx, e.v1alpha1Container)
}

// convertV1Beta1ToV1Alpha1 converts a v1beta1 Container to v1alpha1 for business logic reuse.
func convertV1Beta1ToV1Alpha1(v1beta1Container *v1beta1.Container) *v1alpha1.Container {
	return &v1alpha1.Container{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       v1alpha1.ContainerKind,
		},
		ObjectMeta: v1beta1Container.ObjectMeta,
		Spec: v1alpha1.ContainerSpec{
			ResourceSpec: v1beta1Container.Spec.ResourceSpec,
			ForProvider:  v1alpha1.ContainerParameters(v1beta1Container.Spec.ForProvider),
		},
		Status: v1alpha1.ContainerStatus{
			ResourceStatus: v1beta1Container.Status.ResourceStatus,
			AtProvider:     v1alpha1.ContainerObservation(v1beta1Container.Status.AtProvider),
		},
	}
}
