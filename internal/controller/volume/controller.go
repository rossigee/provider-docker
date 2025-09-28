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

package volume

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types/volume"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	volumev1alpha1 "github.com/rossigee/provider-docker/apis/volume/v1alpha1"
	"github.com/rossigee/provider-docker/internal/clients"
)

const (
	errNotVolume    = "managed resource is not a Volume custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Docker client"

	errVolumeInspect = "cannot inspect volume"
	errVolumeCreate  = "cannot create volume"
	errVolumeRemove  = "cannot remove volume"
)

// SetupVolume adds a controller that reconciles Volume managed resources.
func SetupVolume(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(volumev1alpha1.VolumeGroupKind.Kind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(volumev1alpha1.VolumeGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:   mgr.GetClient(),
			usage:  resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			logger: o.Logger,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&volumev1alpha1.Volume{}).
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
	_, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return nil, errors.New(errNotVolume)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	client, err := clients.NewDockerClient(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{client: client, logger: c.logger}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client clients.DockerClient
	logger logging.Logger
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotVolume)
	}

	volumeName := meta.GetExternalName(cr)
	if volumeName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	vol, err := c.client.VolumeInspect(ctx, volumeName)
	if err != nil {
		if isNotFoundError(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errVolumeInspect)
	}

	// Update observed state
	c.updateStatus(cr, vol)

	// Check if volume is up to date
	upToDate := c.isUpToDate(cr, vol)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotVolume)
	}

	c.logger.Debug("Creating volume", "name", cr.Name)

	opts := c.buildCreateOptions(cr)

	vol, err := c.client.VolumeCreate(ctx, opts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errVolumeCreate)
	}

	meta.SetExternalName(cr, vol.Name)
	c.updateStatus(cr, vol)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// Docker volumes cannot be updated - they are immutable
	// Any changes require recreation
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotVolume)
	}

	volumeName := meta.GetExternalName(cr)
	if volumeName == "" {
		return managed.ExternalDelete{}, nil
	}

	c.logger.Debug("Deleting volume", "name", volumeName)

	err := c.client.VolumeRemove(ctx, volumeName, true) // force=true
	if err != nil && !isNotFoundError(err) {
		return managed.ExternalDelete{}, errors.Wrap(err, errVolumeRemove)
	}

	return managed.ExternalDelete{}, nil
}

// buildCreateOptions constructs volume creation options from the managed resource spec.
func (c *external) buildCreateOptions(cr *volumev1alpha1.Volume) volume.CreateOptions {
	spec := cr.Spec.ForProvider

	opts := volume.CreateOptions{
		Driver:     getStringValue(spec.Driver, "local"),
		DriverOpts: spec.DriverOpts,
		Labels:     spec.Labels,
	}

	// Use specified name or resource name
	if spec.Name != nil {
		opts.Name = *spec.Name
	} else {
		opts.Name = cr.Name
	}

	return opts
}

// updateStatus updates the volume status with observed values.
func (c *external) updateStatus(cr *volumev1alpha1.Volume, vol volume.Volume) {
	cr.Status.AtProvider.Name = vol.Name
	cr.Status.AtProvider.Driver = vol.Driver
	cr.Status.AtProvider.Mountpoint = vol.Mountpoint
	cr.Status.AtProvider.Scope = vol.Scope
	cr.Status.AtProvider.Options = vol.Options
	cr.Status.AtProvider.Labels = vol.Labels

	// Parse CreatedAt
	if vol.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, vol.CreatedAt); err == nil {
			cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: t}
		}
	}

	// Set usage data if available
	if vol.UsageData != nil {
		cr.Status.AtProvider.UsageData = &volumev1alpha1.VolumeUsageData{
			Size:     vol.UsageData.Size,
			RefCount: vol.UsageData.RefCount,
		}
	}

	cr.SetConditions(xpv1.Available())
}

// isUpToDate checks if the current volume matches the desired specification.
func (c *external) isUpToDate(cr *volumev1alpha1.Volume, vol volume.Volume) bool {
	spec := cr.Spec.ForProvider

	// Check driver
	expectedDriver := getStringValue(spec.Driver, "local")
	if vol.Driver != expectedDriver {
		return false
	}

	// Check labels
	if !mapsEqual(vol.Labels, spec.Labels) {
		return false
	}

	// Docker volumes are immutable, so if basic properties match, it's up to date
	return true
}

// getStringValue returns the string value or default if nil.
func getStringValue(s *string, defaultValue string) string {
	if s == nil {
		return defaultValue
	}
	return *s
}

// mapsEqual compares two string maps for equality.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Disconnect is called when the controller is shutting down.
func (c *external) Disconnect(_ context.Context) error {
	return c.client.Close()
}

// isNotFoundError checks if the error indicates a resource was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Docker client returns errors that contain "not found" or "no such"
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "no such") ||
		strings.Contains(errStr, "404")
}
