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

package network

import (
	"context"
	"net/netip"
	"strings"

	dockerclient "github.com/moby/moby/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/moby/moby/api/types/network"
	"github.com/pkg/errors"
	networkv1beta1 "github.com/rossigee/provider-docker/apis/network/v1beta1"
	"github.com/rossigee/provider-docker/internal/clients"
	"github.com/rossigee/provider-docker/internal/tracing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errNotNetwork   = "managed resource is not a Network custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Docker client"

	errNetworkInspect = "cannot inspect network"
	errNetworkCreate  = "cannot create network"
	errNetworkRemove  = "cannot remove network"
)

// SetupNetwork adds a controller that reconciles Network managed resources.
func SetupNetwork(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1beta1.NetworkGroupKind.Kind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			kube:   mgr.GetClient(),
			usage:  resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			logger: o.Logger,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(nil),
	}

	if o.Features != nil && o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(networkv1beta1.NetworkGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1beta1.Network{}).
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
	_, ok := mg.(*networkv1beta1.Network)
	if !ok {
		return nil, errors.New(errNotNetwork)
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
	_, span := tracing.StartSpan(ctx, "network.observe",
		tracing.SpanAttrs("network", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*networkv1beta1.Network)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNetwork)
	}

	networkName := meta.GetExternalName(cr)
	if networkName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	netInspect, err := c.client.NetworkInspect(ctx, networkName, dockerclient.NetworkInspectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errNetworkInspect)
	}

	// Update observed state
	c.updateStatus(cr, netInspect)

	// Check if network is up to date
	upToDate := c.isUpToDate(cr, netInspect)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "network.create",
		tracing.SpanAttrs("network", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*networkv1beta1.Network)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNetwork)
	}

	c.logger.Debug("Creating network", "name", cr.Name)

	name, opts, err := c.buildCreateOptions(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNetworkCreate)
	}

	resp, err := c.client.NetworkCreate(ctx, name, opts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNetworkCreate)
	}

	meta.SetExternalName(cr, resp.ID)

	// Inspect the created network to get full details
	netInspect, err := c.client.NetworkInspect(ctx, resp.ID, dockerclient.NetworkInspectOptions{})
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNetworkInspect)
	}

	c.updateStatus(cr, netInspect)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// Docker networks have limited update capabilities
	// Most changes require recreation
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "network.delete",
		tracing.SpanAttrs("network", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*networkv1beta1.Network)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotNetwork)
	}

	networkName := meta.GetExternalName(cr)
	if networkName == "" {
		return managed.ExternalDelete{}, nil
	}

	c.logger.Debug("Deleting network", "name", networkName)

	err := c.client.NetworkRemove(ctx, networkName)
	if err != nil && !isNotFoundError(err) {
		return managed.ExternalDelete{}, errors.Wrap(err, errNetworkRemove)
	}

	return managed.ExternalDelete{}, nil
}

// buildCreateOptions constructs network creation options from the managed resource spec.
func (c *external) buildCreateOptions(cr *networkv1beta1.Network) (string, dockerclient.NetworkCreateOptions, error) {
	spec := cr.Spec.ForProvider

	opts := dockerclient.NetworkCreateOptions{
		Driver:     getStringValue(spec.Driver, "bridge"),
		Internal:   getBoolValue(spec.Internal, false),
		Attachable: getBoolValue(spec.Attachable, false),
		Ingress:    getBoolValue(spec.Ingress, false),
		EnableIPv6: getBoolPtr(spec.EnableIPv6, false),
		Options:    spec.Options,
		Labels:     spec.Labels,
	}

	// Use specified name or resource name
	name := cr.Name
	if spec.Name != nil {
		name = *spec.Name
	}

	// Build IPAM configuration
	if spec.IPAM != nil {
		opts.IPAM = &network.IPAM{
			Driver:  getStringValue(spec.IPAM.Driver, "default"),
			Options: spec.IPAM.Options,
		}

		// Convert IPAM config entries
		for _, entry := range spec.IPAM.Config {
			ipamConfig := network.IPAMConfig{}

			if entry.Subnet != nil {
				subnet, err := netip.ParsePrefix(*entry.Subnet)
				if err != nil {
					return "", opts, errors.Wrapf(err, "invalid IPAM subnet %q", *entry.Subnet)
				}
				ipamConfig.Subnet = subnet
			}
			if entry.IPRange != nil {
				ipRange, err := netip.ParsePrefix(*entry.IPRange)
				if err != nil {
					return "", opts, errors.Wrapf(err, "invalid IPAM ipRange %q", *entry.IPRange)
				}
				ipamConfig.IPRange = ipRange
			}
			if entry.Gateway != nil {
				gateway, err := netip.ParseAddr(*entry.Gateway)
				if err != nil {
					return "", opts, errors.Wrapf(err, "invalid IPAM gateway %q", *entry.Gateway)
				}
				ipamConfig.Gateway = gateway
			}
			if entry.AuxAddresses != nil {
				ipamConfig.AuxAddress = make(map[string]netip.Addr, len(entry.AuxAddresses))
				for key, value := range entry.AuxAddresses {
					addr, err := netip.ParseAddr(value)
					if err != nil {
						return "", opts, errors.Wrapf(err, "invalid IPAM auxiliary address %q", value)
					}
					ipamConfig.AuxAddress[key] = addr
				}
			}

			opts.IPAM.Config = append(opts.IPAM.Config, ipamConfig)
		}
	}

	return name, opts, nil
}

// updateStatus updates the network status with observed values.
func (c *external) updateStatus(cr *networkv1beta1.Network, netInspect network.Inspect) {
	cr.Status.AtProvider.ID = netInspect.ID
	cr.Status.AtProvider.Name = netInspect.Name
	cr.Status.AtProvider.Driver = netInspect.Driver
	cr.Status.AtProvider.Scope = netInspect.Scope
	cr.Status.AtProvider.Internal = netInspect.Internal
	cr.Status.AtProvider.Attachable = netInspect.Attachable
	cr.Status.AtProvider.Ingress = netInspect.Ingress
	cr.Status.AtProvider.EnableIPv6 = netInspect.EnableIPv6
	cr.Status.AtProvider.Options = netInspect.Options
	cr.Status.AtProvider.Labels = netInspect.Labels

	// Parse CreatedAt
	if !netInspect.Created.IsZero() {
		cr.Status.AtProvider.CreatedAt = &metav1.Time{Time: netInspect.Created}
	}

	// Set IPAM configuration
	if netInspect.IPAM.Driver != "" {
		cr.Status.AtProvider.IPAM = &networkv1beta1.IPAMConfig{
			Driver:  &netInspect.IPAM.Driver,
			Options: netInspect.IPAM.Options,
		}

		for _, cfg := range netInspect.IPAM.Config {
			ipamEntry := networkv1beta1.IPAMConfigEntry{}
			if cfg.Subnet.IsValid() {
				subnet := cfg.Subnet.String()
				ipamEntry.Subnet = &subnet
			}
			if cfg.IPRange.IsValid() {
				ipRange := cfg.IPRange.String()
				ipamEntry.IPRange = &ipRange
			}
			if cfg.Gateway.IsValid() {
				gateway := cfg.Gateway.String()
				ipamEntry.Gateway = &gateway
			}
			if cfg.AuxAddress != nil {
				ipamEntry.AuxAddresses = make(map[string]string, len(cfg.AuxAddress))
				for key, addr := range cfg.AuxAddress {
					ipamEntry.AuxAddresses[key] = addr.String()
				}
			}
			cr.Status.AtProvider.IPAM.Config = append(cr.Status.AtProvider.IPAM.Config, ipamEntry)
		}
	}

	// Set container information
	if netInspect.Containers != nil {
		cr.Status.AtProvider.Containers = make(map[string]*networkv1beta1.NetworkContainer)
		for id, container := range netInspect.Containers {
			cr.Status.AtProvider.Containers[id] = &networkv1beta1.NetworkContainer{
				Name:        container.Name,
				EndpointID:  container.EndpointID,
				MacAddress:  container.MacAddress.String(),
				IPv4Address: clients.PrefixString(container.IPv4Address),
				IPv6Address: clients.PrefixString(container.IPv6Address),
			}
		}
	}

	cr.SetConditions(xpv1.Available())
}

// isUpToDate checks if the current network matches the desired specification.
func (c *external) isUpToDate(cr *networkv1beta1.Network, netInspect network.Inspect) bool {
	spec := cr.Spec.ForProvider

	// Check driver
	expectedDriver := getStringValue(spec.Driver, "bridge")
	if netInspect.Driver != expectedDriver {
		return false
	}

	// Check boolean flags
	if netInspect.Internal != getBoolValue(spec.Internal, false) {
		return false
	}
	if netInspect.Attachable != getBoolValue(spec.Attachable, false) {
		return false
	}
	if netInspect.EnableIPv6 != getBoolValue(spec.EnableIPv6, false) {
		return false
	}

	// Check labels
	if !mapsEqual(netInspect.Labels, spec.Labels) {
		return false
	}

	return true
}

// getStringValue returns the string value or default if nil.
func getStringValue(s *string, defaultValue string) string {
	if s == nil {
		return defaultValue
	}
	return *s
}

// getBoolValue returns the bool value or default if nil.
func getBoolValue(b *bool, defaultValue bool) bool {
	if b == nil {
		return defaultValue
	}
	return *b
}

// getBoolPtr returns a pointer to bool value or default if nil.
func getBoolPtr(b *bool, defaultValue bool) *bool {
	if b == nil {
		result := defaultValue
		return &result
	}
	return b
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
