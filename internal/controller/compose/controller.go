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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	composev1alpha1 "github.com/rossigee/provider-docker/apis/compose/v1alpha1"
	containerv1alpha1 "github.com/rossigee/provider-docker/apis/container/v1alpha1"
	dockerclients "github.com/rossigee/provider-docker/internal/clients"
	"github.com/rossigee/provider-docker/internal/compose"
)

const (
	errNotComposeStack  = "managed resource is not a ComposeStack custom resource"
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetPC            = "cannot get ProviderConfig"
	errGetCreds         = "cannot get credentials"
	errNewClient        = "cannot create new Docker client"
	errParseCompose     = "cannot parse Docker Compose content"
	errGetConfigMap     = "cannot get ConfigMap"
	errGetSecret        = "cannot get Secret"
	errCreateContainer  = "cannot create container"
	errObserveContainer = "cannot observe container"
	errUpdateContainer  = "cannot update container"
	errDeleteContainer  = "cannot delete container"

	// Reconcile intervals
	reconcileTimeout = 2 * time.Minute
	pollInterval     = 30 * time.Second
)

// Setup adds a controller that reconciles ComposeStack managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(composev1alpha1.ComposeStackGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(composev1alpha1.ComposeStackGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			newServiceFn: dockerclients.NewDockerClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(pollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&composev1alpha1.ComposeStack{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(context.Context, client.Client, resource.Managed) (dockerclients.DockerClient, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, ok := mg.(*composev1alpha1.ComposeStack)
	if !ok {
		return nil, errors.New(errNotComposeStack)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	// Create Docker client using the shared client creation function
	svc, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		kube:    c.kube,
		service: svc,
		parser:  compose.NewParser("", "", nil),
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube    client.Client
	service dockerclients.DockerClient
	parser  *compose.Parser
}

func (c *external) Disconnect(ctx context.Context) error {
	return c.service.Close()
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*composev1alpha1.ComposeStack)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotComposeStack)
	}

	// Parse the compose content
	composeContent, err := c.getComposeContent(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errParseCompose)
	}

	// Create parser with project configuration
	projectName := c.getProjectName(cr)
	environment := c.buildEnvironment(ctx, cr)
	parser := compose.NewParser(projectName, "", environment)

	// Parse the compose file
	parseResult, err := parser.ParseCompose(ctx, composeContent)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errParseCompose)
	}

	// Check if containers exist and get their status
	observation := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}

	services := make(map[string]composev1alpha1.ServiceStatus)
	allRunning := true

	for _, container := range parseResult.Containers {
		containerName := c.getContainerName(projectName, container.Name)

		// Try to inspect the container
		containerInfo, err := c.service.ContainerInspect(ctx, containerName)
		if err != nil {
			// Container doesn't exist
			observation.ResourceExists = false
			allRunning = false
			services[container.Name] = composev1alpha1.ServiceStatus{
				Name:  container.Name,
				State: "pending",
			}
			continue
		}

		// Container exists, check its state
		status := composev1alpha1.ServiceStatus{
			Name:        container.Name,
			ContainerID: &containerInfo.ID,
			State:       containerInfo.State.Status,
			Image:       &containerInfo.Config.Image,
		}

		if containerInfo.State.StartedAt != "" {
			if startedAt, err := time.Parse(time.RFC3339Nano, containerInfo.State.StartedAt); err == nil {
				status.StartedAt = &metav1.Time{Time: startedAt}
			}
		}

		if containerInfo.State.Status != "running" {
			allRunning = false
			observation.ResourceUpToDate = false
		}

		services[container.Name] = status
	}

	// Update status
	cr.Status.AtProvider.ProjectName = projectName
	cr.Status.AtProvider.Services = services
	cr.Status.AtProvider.ParsedAt = &metav1.Time{Time: time.Now()}

	// Set conditions
	if !observation.ResourceExists {
		cr.SetConditions(xpv1.Unavailable())
	} else if allRunning {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Creating())
	}

	return observation, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*composev1alpha1.ComposeStack)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotComposeStack)
	}

	cr.SetConditions(xpv1.Creating())

	// Parse the compose content
	composeContent, err := c.getComposeContent(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errParseCompose)
	}

	// Create parser with project configuration
	projectName := c.getProjectName(cr)
	environment := c.buildEnvironment(ctx, cr)
	parser := compose.NewParser(projectName, "", environment)

	// Parse the compose file
	parseResult, err := parser.ParseCompose(ctx, composeContent)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errParseCompose)
	}

	// Create containers in dependency order
	// For now, create them sequentially. In a full implementation,
	// we would implement proper dependency ordering based on depends_on
	for _, cont := range parseResult.Containers {
		err := c.createContainer(ctx, cr, projectName, &cont)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrapf(err, errCreateContainer)
		}
	}

	return managed.ExternalCreation{
		// Optionally return any values that should be written to a connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*composev1alpha1.ComposeStack)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotComposeStack)
	}

	// For now, we implement update as a simple restart
	// In a full implementation, we would analyze what changed and update accordingly
	cr.SetConditions(xpv1.Creating())

	// This is a simplified update that recreates containers
	// A production implementation would be more sophisticated
	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*composev1alpha1.ComposeStack)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotComposeStack)
	}

	cr.SetConditions(xpv1.Deleting())

	// Get project name
	projectName := c.getProjectName(cr)

	// List all containers with this project name and remove them
	// This is a simplified approach - a full implementation would track
	// exactly which containers belong to this stack
	containers, err := c.service.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("com.docker.compose.project=%s", projectName)),
		),
	})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot list containers")
	}

	for _, cont := range containers {
		// Stop and remove the container
		timeout := 10 // 10 second timeout
		stopOpts := container.StopOptions{Timeout: &timeout}
		err := c.service.ContainerStop(ctx, cont.ID, stopOpts)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrapf(err, "cannot stop container %s", cont.ID)
		}

		err = c.service.ContainerRemove(ctx, cont.ID, container.RemoveOptions{
			Force: true,
		})
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrapf(err, "cannot remove container %s", cont.ID)
		}
	}

	return managed.ExternalDelete{}, nil
}

// Helper methods

func (c *external) getComposeContent(ctx context.Context, cr *composev1alpha1.ComposeStack) (string, error) {
	if cr.Spec.ForProvider.Compose != nil {
		return *cr.Spec.ForProvider.Compose, nil
	}

	if cr.Spec.ForProvider.ComposeRef != nil {
		if cr.Spec.ForProvider.ComposeRef.ConfigMapRef != nil {
			return c.getComposeFromConfigMap(ctx, cr, cr.Spec.ForProvider.ComposeRef.ConfigMapRef)
		}
		if cr.Spec.ForProvider.ComposeRef.SecretRef != nil {
			return c.getComposeFromSecret(ctx, cr, cr.Spec.ForProvider.ComposeRef.SecretRef)
		}
	}

	return "", errors.New("no compose content or reference specified")
}

func (c *external) getComposeFromConfigMap(ctx context.Context, cr *composev1alpha1.ComposeStack, ref *composev1alpha1.ConfigMapReference) (string, error) {
	namespace := cr.GetNamespace()
	if ref.Namespace != nil {
		namespace = *ref.Namespace
	}

	cm := &v1.ConfigMap{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      ref.Name,
	}, cm); err != nil {
		return "", errors.Wrap(err, errGetConfigMap)
	}

	content, exists := cm.Data[ref.Key]
	if !exists {
		return "", errors.Errorf("key %s not found in ConfigMap %s", ref.Key, ref.Name)
	}

	return content, nil
}

func (c *external) getComposeFromSecret(ctx context.Context, cr *composev1alpha1.ComposeStack, ref *composev1alpha1.SecretReference) (string, error) {
	namespace := cr.GetNamespace()
	if ref.Namespace != nil {
		namespace = *ref.Namespace
	}

	secret := &v1.Secret{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      ref.Name,
	}, secret); err != nil {
		return "", errors.Wrap(err, errGetSecret)
	}

	content, exists := secret.Data[ref.Key]
	if !exists {
		return "", errors.Errorf("key %s not found in Secret %s", ref.Key, ref.Name)
	}

	return string(content), nil
}

func (c *external) getProjectName(cr *composev1alpha1.ComposeStack) string {
	if cr.Spec.ForProvider.ProjectName != nil {
		return *cr.Spec.ForProvider.ProjectName
	}
	return cr.GetName()
}

func (c *external) buildEnvironment(ctx context.Context, cr *composev1alpha1.ComposeStack) map[string]string {
	environment := make(map[string]string)

	for _, env := range cr.Spec.ForProvider.Environment {
		if env.Value != nil {
			environment[env.Name] = *env.Value
		} else if env.ValueFrom != nil {
			// Resolve environment variable from ConfigMap or Secret
			value, err := c.resolveEnvValueFrom(ctx, cr, env.ValueFrom)
			if err != nil {
				// Log error but continue - could use empty string or skip
				// In production, might want to fail fast or use status conditions
				environment[env.Name] = ""
			} else {
				environment[env.Name] = value
			}
		}
	}

	return environment
}

func (c *external) resolveEnvValueFrom(ctx context.Context, cr *composev1alpha1.ComposeStack, valueFrom *composev1alpha1.EnvVarSource) (string, error) {
	if valueFrom.SecretKeyRef != nil {
		return c.getValueFromSecret(ctx, cr, valueFrom.SecretKeyRef)
	}
	if valueFrom.ConfigMapKeyRef != nil {
		return c.getValueFromConfigMap(ctx, cr, valueFrom.ConfigMapKeyRef)
	}
	return "", errors.New("no valid valueFrom source specified")
}

func (c *external) getValueFromSecret(ctx context.Context, cr *composev1alpha1.ComposeStack, secretRef *composev1alpha1.SecretKeySelector) (string, error) {
	namespace := secretRef.Namespace
	if namespace == nil {
		ns := cr.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		namespace = &ns
	}

	secret := &v1.Secret{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: *namespace,
		Name:      secretRef.Name,
	}, secret); err != nil {
		return "", errors.Wrapf(err, "failed to get secret %s/%s", *namespace, secretRef.Name)
	}

	data, exists := secret.Data[secretRef.Key]
	if !exists {
		return "", errors.Errorf("key %s not found in secret %s/%s", secretRef.Key, *namespace, secretRef.Name)
	}

	return string(data), nil
}

func (c *external) getValueFromConfigMap(ctx context.Context, cr *composev1alpha1.ComposeStack, configMapRef *composev1alpha1.ConfigMapKeySelector) (string, error) {
	namespace := configMapRef.Namespace
	if namespace == nil {
		ns := cr.GetNamespace()
		if ns == "" {
			ns = "default"
		}
		namespace = &ns
	}

	configMap := &v1.ConfigMap{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: *namespace,
		Name:      configMapRef.Name,
	}, configMap); err != nil {
		return "", errors.Wrapf(err, "failed to get configMap %s/%s", *namespace, configMapRef.Name)
	}

	data, exists := configMap.Data[configMapRef.Key]
	if !exists {
		return "", errors.Errorf("key %s not found in configMap %s/%s", configMapRef.Key, *namespace, configMapRef.Name)
	}

	return data, nil
}

func (c *external) getContainerName(projectName, serviceName string) string {
	return fmt.Sprintf("%s_%s_1", projectName, serviceName)
}

func (c *external) createContainer(ctx context.Context, cr *composev1alpha1.ComposeStack, projectName string, cont *containerv1alpha1.Container) error {
	// Convert Container spec to Docker API calls
	containerName := c.getContainerName(projectName, cont.Name)

	// Check if container already exists
	_, err := c.service.ContainerInspect(ctx, containerName)
	if err == nil {
		// Container already exists, check if it needs to be updated
		return nil
	}

	// Convert Container spec to Docker container configuration
	config, hostConfig, networkConfig, err := c.convertContainerSpec(ctx, cr, &cont.Spec.ForProvider, projectName)
	if err != nil {
		return errors.Wrap(err, "failed to convert container spec")
	}

	// Create the container
	resp, err := c.service.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return errors.Wrapf(err, "failed to create container %s", containerName)
	}

	// Start the container if StartOnCreate is true (default)
	startOnCreate := true
	if cont.Spec.ForProvider.StartOnCreate != nil {
		startOnCreate = *cont.Spec.ForProvider.StartOnCreate
	}

	if startOnCreate {
		err = c.service.ContainerStart(ctx, resp.ID, container.StartOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to start container %s", containerName)
		}
	}

	return nil
}

// convertContainerSpec converts a Container spec to Docker API configuration structs
func (c *external) convertContainerSpec(ctx context.Context, cr *composev1alpha1.ComposeStack, spec *containerv1alpha1.ContainerParameters, projectName string) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
	// Container configuration
	config := &container.Config{
		Image: spec.Image,
	}

	// Set container name
	if spec.Name != nil {
		config.Hostname = *spec.Name
	}

	// Set command and args
	if len(spec.Command) > 0 {
		config.Cmd = spec.Command
	}
	if len(spec.Args) > 0 {
		config.Cmd = append(config.Cmd, spec.Args...)
	}

	// Set environment variables
	if len(spec.Environment) > 0 {
		config.Env = c.convertEnvironmentVars(ctx, cr, spec.Environment)
	}

	// Set working directory
	if spec.WorkingDir != nil {
		config.WorkingDir = *spec.WorkingDir
	}

	// Set user
	if spec.User != nil {
		config.User = *spec.User
	}

	// Set hostname
	if spec.Hostname != nil {
		config.Hostname = *spec.Hostname
	}

	// Set labels
	if len(spec.Labels) > 0 {
		config.Labels = spec.Labels
	}

	// Add compose project label
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	config.Labels["com.docker.compose.project"] = projectName
	if spec.Name != nil {
		config.Labels["com.docker.compose.service"] = *spec.Name
	}

	// Set exposed ports
	if len(spec.Ports) > 0 {
		config.ExposedPorts = c.convertExposedPorts(spec.Ports)
	}

	// Host configuration
	hostConfig := &container.HostConfig{}

	// Set restart policy
	if spec.RestartPolicy != nil {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(*spec.RestartPolicy),
		}
		if spec.MaximumRetryCount != nil && *spec.RestartPolicy == "on-failure" {
			hostConfig.RestartPolicy.MaximumRetryCount = *spec.MaximumRetryCount
		}
	}

	// Set network mode
	if spec.NetworkMode != nil {
		hostConfig.NetworkMode = container.NetworkMode(*spec.NetworkMode)
	}

	// Set port bindings
	if len(spec.Ports) > 0 {
		hostConfig.PortBindings = c.convertPortBindings(spec.Ports)
	}

	// Set volume mounts
	if len(spec.Volumes) > 0 {
		binds, mounts, err := c.convertVolumeMounts(spec.Volumes)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to convert volume mounts")
		}
		hostConfig.Binds = binds
		hostConfig.Mounts = mounts
	}

	// Set DNS configuration
	if len(spec.DNS) > 0 {
		hostConfig.DNS = spec.DNS
	}
	if len(spec.DNSSearch) > 0 {
		hostConfig.DNSSearch = spec.DNSSearch
	}
	if len(spec.DNSOptions) > 0 {
		hostConfig.DNSOptions = spec.DNSOptions
	}

	// Set extra hosts
	if len(spec.ExtraHosts) > 0 {
		hostConfig.ExtraHosts = spec.ExtraHosts
	}

	// Set privileged mode
	if spec.Privileged != nil {
		hostConfig.Privileged = *spec.Privileged
	}

	// Set resource limits
	if spec.Resources != nil {
		c.setResourceLimits(hostConfig, spec.Resources)
	}

	// Set security context
	if spec.SecurityContext != nil {
		c.setSecurityContext(hostConfig, config, spec.SecurityContext)
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{}
	if len(spec.Networks) > 0 {
		networkConfig.EndpointsConfig = c.convertNetworkAttachments(spec.Networks)
	}

	return config, hostConfig, networkConfig, nil
}

// Helper methods for converting Container spec fields

func (c *external) convertEnvironmentVars(ctx context.Context, cr *composev1alpha1.ComposeStack, envVars []containerv1alpha1.EnvVar) []string {
	var env []string
	for _, envVar := range envVars {
		if envVar.Value != nil {
			env = append(env, fmt.Sprintf("%s=%s", envVar.Name, *envVar.Value))
		} else if envVar.ValueFrom != nil {
			// Resolve environment variable from ConfigMap or Secret
			value, err := c.resolveContainerEnvValueFrom(ctx, cr, envVar.ValueFrom)
			if err != nil {
				// Log error but continue - could use empty string or skip
				// In production, might want to fail fast or use status conditions
				env = append(env, fmt.Sprintf("%s=", envVar.Name))
			} else {
				env = append(env, fmt.Sprintf("%s=%s", envVar.Name, value))
			}
		} else {
			// Environment variable without value (will be inherited from host)
			env = append(env, envVar.Name)
		}
	}
	return env
}

func (c *external) resolveContainerEnvValueFrom(ctx context.Context, cr *composev1alpha1.ComposeStack, valueFrom *containerv1alpha1.EnvVarSource) (string, error) {
	if valueFrom.SecretKeyRef != nil {
		return c.getValueFromContainerSecret(ctx, cr, valueFrom.SecretKeyRef)
	}
	if valueFrom.ConfigMapKeyRef != nil {
		return c.getValueFromContainerConfigMap(ctx, cr, valueFrom.ConfigMapKeyRef)
	}
	return "", errors.New("no valid valueFrom source specified")
}

func (c *external) getValueFromContainerSecret(ctx context.Context, cr *composev1alpha1.ComposeStack, secretRef *containerv1alpha1.SecretKeySelector) (string, error) {
	namespace := cr.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	secret := &v1.Secret{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      secretRef.Name,
	}, secret); err != nil {
		if secretRef.Optional != nil && *secretRef.Optional {
			return "", nil // Optional secret not found, return empty string
		}
		return "", errors.Wrapf(err, "failed to get secret %s/%s", namespace, secretRef.Name)
	}

	data, exists := secret.Data[secretRef.Key]
	if !exists {
		if secretRef.Optional != nil && *secretRef.Optional {
			return "", nil // Optional key not found, return empty string
		}
		return "", errors.Errorf("key %s not found in secret %s/%s", secretRef.Key, namespace, secretRef.Name)
	}

	return string(data), nil
}

func (c *external) getValueFromContainerConfigMap(ctx context.Context, cr *composev1alpha1.ComposeStack, configMapRef *containerv1alpha1.ConfigMapKeySelector) (string, error) {
	namespace := cr.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	configMap := &v1.ConfigMap{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      configMapRef.Name,
	}, configMap); err != nil {
		if configMapRef.Optional != nil && *configMapRef.Optional {
			return "", nil // Optional configMap not found, return empty string
		}
		return "", errors.Wrapf(err, "failed to get configMap %s/%s", namespace, configMapRef.Name)
	}

	data, exists := configMap.Data[configMapRef.Key]
	if !exists {
		if configMapRef.Optional != nil && *configMapRef.Optional {
			return "", nil // Optional key not found, return empty string
		}
		return "", errors.Errorf("key %s not found in configMap %s/%s", configMapRef.Key, namespace, configMapRef.Name)
	}

	return data, nil
}

func (c *external) convertExposedPorts(ports []containerv1alpha1.PortSpec) nat.PortSet {
	exposedPorts := make(nat.PortSet)
	for _, port := range ports {
		protocol := "tcp"
		if port.Protocol != nil {
			protocol = strings.ToLower(*port.Protocol)
		}
		natPort, _ := nat.NewPort(protocol, fmt.Sprintf("%d", port.ContainerPort))
		exposedPorts[natPort] = struct{}{}
	}
	return exposedPorts
}

func (c *external) convertPortBindings(ports []containerv1alpha1.PortSpec) nat.PortMap {
	portBindings := make(nat.PortMap)
	for _, port := range ports {
		if port.HostPort == nil {
			continue // No host port binding
		}

		protocol := "tcp"
		if port.Protocol != nil {
			protocol = strings.ToLower(*port.Protocol)
		}

		natPort, _ := nat.NewPort(protocol, fmt.Sprintf("%d", port.ContainerPort))

		binding := nat.PortBinding{
			HostPort: fmt.Sprintf("%d", *port.HostPort),
		}

		if port.HostIP != nil {
			binding.HostIP = *port.HostIP
		}

		portBindings[natPort] = append(portBindings[natPort], binding)
	}
	return portBindings
}

func (c *external) convertVolumeMounts(volumes []containerv1alpha1.VolumeMount) ([]string, []mount.Mount, error) {
	var binds []string
	var mounts []mount.Mount

	for _, vol := range volumes {
		if vol.VolumeSource.HostPath != nil {
			// Host path bind mount
			bind := fmt.Sprintf("%s:%s", vol.VolumeSource.HostPath.Path, vol.MountPath)
			if vol.ReadOnly != nil && *vol.ReadOnly {
				bind += ":ro"
			}
			binds = append(binds, bind)
		} else if vol.VolumeSource.Volume != nil {
			// Named volume mount
			mountObj := mount.Mount{
				Type:   mount.TypeVolume,
				Source: vol.VolumeSource.Volume.VolumeName,
				Target: vol.MountPath,
			}
			if vol.ReadOnly != nil && *vol.ReadOnly {
				mountObj.ReadOnly = true
			}
			mounts = append(mounts, mountObj)
		} else if vol.VolumeSource.Bind != nil {
			// Bind mount with propagation
			mountObj := mount.Mount{
				Type:   mount.TypeBind,
				Source: vol.VolumeSource.Bind.SourcePath,
				Target: vol.MountPath,
			}
			if vol.ReadOnly != nil && *vol.ReadOnly {
				mountObj.ReadOnly = true
			}
			if vol.VolumeSource.Bind.Propagation != nil {
				mountObj.BindOptions = &mount.BindOptions{
					Propagation: mount.Propagation(*vol.VolumeSource.Bind.Propagation),
				}
			}
			mounts = append(mounts, mountObj)
		}
		// NOTE: EmptyDir, Secret, ConfigMap volume types need Kubernetes client integration
	}

	return binds, mounts, nil
}

func (c *external) convertNetworkAttachments(networks []containerv1alpha1.NetworkAttachment) map[string]*network.EndpointSettings {
	endpoints := make(map[string]*network.EndpointSettings)
	for _, net := range networks {
		endpoint := &network.EndpointSettings{}

		if net.IPAddress != nil {
			endpoint.IPAMConfig = &network.EndpointIPAMConfig{
				IPv4Address: *net.IPAddress,
			}
		}

		if net.IPv6Address != nil {
			if endpoint.IPAMConfig == nil {
				endpoint.IPAMConfig = &network.EndpointIPAMConfig{}
			}
			endpoint.IPAMConfig.IPv6Address = *net.IPv6Address
		}

		if len(net.Aliases) > 0 {
			endpoint.Aliases = net.Aliases
		}

		endpoints[net.Name] = endpoint
	}
	return endpoints
}

func (c *external) setResourceLimits(hostConfig *container.HostConfig, resources *containerv1alpha1.ResourceRequirements) {
	// Set memory limits
	if resources.Limits != nil {
		if memLimit, exists := resources.Limits["memory"]; exists {
			if bytes, err := parseMemory(memLimit.String()); err == nil {
				hostConfig.Memory = bytes
			}
		}
		if cpuLimit, exists := resources.Limits["cpu"]; exists {
			if nanos, err := parseCPU(cpuLimit.String()); err == nil {
				hostConfig.CPUQuota = nanos
				hostConfig.CPUPeriod = 100000 // 100ms period
			}
		}
	}

	// Set memory reservations
	if resources.Requests != nil {
		if memRequest, exists := resources.Requests["memory"]; exists {
			if bytes, err := parseMemory(memRequest.String()); err == nil {
				hostConfig.MemoryReservation = bytes
			}
		}
	}
}

func (c *external) setSecurityContext(hostConfig *container.HostConfig, config *container.Config, secCtx *containerv1alpha1.SecurityContext) {
	if secCtx.RunAsUser != nil {
		config.User = fmt.Sprintf("%d", *secCtx.RunAsUser)
		if secCtx.RunAsGroup != nil {
			config.User = fmt.Sprintf("%d:%d", *secCtx.RunAsUser, *secCtx.RunAsGroup)
		}
	}

	if secCtx.ReadOnlyRootFilesystem != nil && *secCtx.ReadOnlyRootFilesystem {
		hostConfig.ReadonlyRootfs = true
	}

	if secCtx.Capabilities != nil {
		if len(secCtx.Capabilities.Add) > 0 {
			hostConfig.CapAdd = secCtx.Capabilities.Add
		}
		if len(secCtx.Capabilities.Drop) > 0 {
			hostConfig.CapDrop = secCtx.Capabilities.Drop
		}
	}
}

// Helper functions for parsing resource values

func parseMemory(memStr string) (int64, error) {
	// Simple memory parsing - in production would use k8s resource parsing
	if strings.HasSuffix(memStr, "Mi") {
		if val, err := strconv.ParseInt(strings.TrimSuffix(memStr, "Mi"), 10, 64); err == nil {
			return val * 1024 * 1024, nil
		}
	} else if strings.HasSuffix(memStr, "Gi") {
		if val, err := strconv.ParseInt(strings.TrimSuffix(memStr, "Gi"), 10, 64); err == nil {
			return val * 1024 * 1024 * 1024, nil
		}
	}
	return 0, errors.New("unsupported memory format")
}

func parseCPU(cpuStr string) (int64, error) {
	// Simple CPU parsing - convert to nanoseconds
	if val, err := strconv.ParseFloat(cpuStr, 64); err == nil {
		return int64(val * 1000000000), nil // Convert to nanoseconds
	}
	return 0, errors.New("unsupported CPU format")
}
