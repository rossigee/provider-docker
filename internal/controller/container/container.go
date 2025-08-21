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
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpcontroller "github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-docker/apis/container/v1alpha1"
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
)

// Setup adds a controller that reconciles Container managed resources.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	name := managed.ControllerName(v1alpha1.ContainerGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ContainerGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:   mgr.GetClient(),
			usage:  resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
			logger: o.Logger,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

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
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
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
		client: dockerClient,
		logger: c.logger,
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client clients.DockerClient
	logger logging.Logger
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotContainer)
	}

	// Get the container ID from external name or status
	containerID := cr.GetAnnotations()[xpv1.AnnotationKeyExternalName]
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
	containerConfig, hostConfig, networkingConfig, platform := c.buildContainerConfig(cr)

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
		if err := c.client.ContainerStart(ctx, response.ID, types.ContainerStartOptions{}); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot start container")
		}
	}

	return managed.ExternalCreation{
		ExternalNameAssigned: true,
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotContainer)
	}

	containerID := cr.GetAnnotations()[xpv1.AnnotationKeyExternalName]
	if containerID == "" {
		return managed.ExternalUpdate{}, errors.New("external name not set")
	}

	c.logger.Debug("Updating container", "container", cr.Name, "id", containerID)

	// TODO: Implement container updates
	// This could include updating resource limits, environment variables, etc.
	// For now, we'll just return success as basic container config is immutable

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Container)
	if !ok {
		return errors.New(errNotContainer)
	}

	containerID := cr.GetAnnotations()[xpv1.AnnotationKeyExternalName]
	if containerID == "" {
		return nil // Nothing to delete
	}

	c.logger.Debug("Deleting container", "container", cr.Name, "id", containerID)

	// Stop the container first
	timeout := 10 * time.Second
	if err := c.client.ContainerStop(ctx, containerID, types.ContainerStopOptions{Timeout: &timeout}); err != nil {
		if !isNotFound(err) {
			return errors.Wrap(err, "cannot stop container")
		}
	}

	// Remove the container
	if err := c.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
		if !isNotFound(err) {
			return errors.Wrap(err, errDeleteFailed)
		}
	}

	return nil
}

// Helper functions

func (c *external) buildContainerConfig(cr *v1alpha1.Container) (*container.Config, *container.HostConfig, *network.NetworkingConfig, *image.Platform) {
	// TODO: Implement full container configuration mapping
	// This is a minimal implementation for the MVP

	config := &container.Config{
		Image: cr.Spec.ForProvider.Image,
	}

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
		env := make([]string, 0, len(cr.Spec.ForProvider.Environment))
		for _, envVar := range cr.Spec.ForProvider.Environment {
			if envVar.Value != nil {
				env = append(env, envVar.Name+"="+*envVar.Value)
			}
			// TODO: Handle valueFrom
		}
		config.Env = env
	}

	// Labels
	if len(cr.Spec.ForProvider.Labels) > 0 {
		config.Labels = cr.Spec.ForProvider.Labels
	}

	hostConfig := &container.HostConfig{}

	// Restart policy
	if cr.Spec.ForProvider.RestartPolicy != nil {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(*cr.Spec.ForProvider.RestartPolicy),
		}
		if cr.Spec.ForProvider.MaximumRetryCount != nil {
			hostConfig.RestartPolicy.MaximumRetryCount = *cr.Spec.ForProvider.MaximumRetryCount
		}
	}

	// TODO: Add port mappings, volume mounts, networks, etc.

	return config, hostConfig, nil, nil
}

func (c *external) updateStatus(cr *v1alpha1.Container, containerInfo *types.ContainerJSON) {
	// TODO: Update the Container status with observed state
	// This is a placeholder for MVP

	cr.Status.AtProvider = v1alpha1.ContainerObservation{}

	cr.Status.AtProvider.ID = containerInfo.ID
	cr.Status.AtProvider.Name = containerInfo.Name

	// Update state
	cr.Status.AtProvider.State = v1alpha1.ContainerState{
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

	// Set condition based on container state
	if containerInfo.State.Running {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Unavailable().WithMessage("Container is not running"))
	}
}

func (c *external) isUpToDate(cr *v1alpha1.Container, containerInfo *types.ContainerJSON) bool {
	// TODO: Implement proper up-to-date checking
	// For MVP, assume container is up to date if it exists and is running
	return containerInfo.State.Running
}

func isNotFound(err error) bool {
	// TODO: Implement proper Docker API error checking
	return false
}
