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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/pkg/errors"

	containerv1alpha1 "github.com/rossigee/provider-docker/apis/container/v1alpha1"
)

// Parser handles parsing Docker Compose files and converting them to Crossplane resources.
type Parser struct {
	projectName string
	workingDir  string
	environment map[string]string
}

// NewParser creates a new compose parser with the given configuration.
func NewParser(projectName, workingDir string, environment map[string]string) *Parser {
	return &Parser{
		projectName: projectName,
		workingDir:  workingDir,
		environment: environment,
	}
}

// ParseResult contains the results of parsing a compose file.
type ParseResult struct {
	Project    *types.Project
	Containers []containerv1alpha1.Container
	Networks   []NetworkDefinition
	Volumes    []VolumeDefinition
}

// NetworkDefinition represents a Docker network to be created.
type NetworkDefinition struct {
	Name   string
	Driver string
	IPAM   *types.IPAMConfig
	Labels map[string]string
}

// VolumeDefinition represents a Docker volume to be created.
type VolumeDefinition struct {
	Name       string
	Driver     string
	DriverOpts map[string]string
	Labels     map[string]string
}

// ParseCompose parses a Docker Compose file content and returns Crossplane resources.
func (p *Parser) ParseCompose(ctx context.Context, composeContent string) (*ParseResult, error) {
	// Create a temporary file with the compose content
	tmpDir, err := os.MkdirTemp("", "compose-parse-*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary directory")
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	err = os.WriteFile(composeFile, []byte(composeContent), 0644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write compose file")
	}

	// Set working directory to tmp if not specified
	workingDir := p.workingDir
	if workingDir == "" {
		workingDir = tmpDir
	}

	// Create project options
	options, err := cli.NewProjectOptions(
		[]string{composeFile},
		cli.WithName(p.projectName),
		cli.WithWorkingDirectory(workingDir),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create project options")
	}

	// Set environment variables
	if p.environment != nil {
		options.Environment = p.environment
	}

	// Parse the compose content
	project, err := cli.ProjectFromOptions(ctx, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse compose content")
	}

	// Convert to Crossplane resources
	result := &ParseResult{
		Project: project,
	}

	// Convert services to Container resources
	containers, err := p.convertServices(project.Services)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert services")
	}
	result.Containers = containers

	// Convert networks
	networks := p.convertNetworks(project.Networks)
	result.Networks = networks

	// Convert volumes
	volumes := p.convertVolumes(project.Volumes)
	result.Volumes = volumes

	return result, nil
}

// convertServices converts Docker Compose services to Container resources.
func (p *Parser) convertServices(services types.Services) ([]containerv1alpha1.Container, error) {
	var containers []containerv1alpha1.Container

	for _, service := range services {
		container, err := p.convertService(service)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert service %s", service.Name)
		}
		containers = append(containers, *container)
	}

	return containers, nil
}

// convertService converts a single Docker Compose service to a Container resource.
func (p *Parser) convertService(service types.ServiceConfig) (*containerv1alpha1.Container, error) {
	container := &containerv1alpha1.Container{}
	container.SetName(fmt.Sprintf("%s-%s", p.projectName, service.Name))

	// Basic container configuration
	params := containerv1alpha1.ContainerParameters{
		Image: service.Image,
		Name:  &service.Name,
	}

	// Convert command and args
	if len(service.Command) > 0 {
		params.Command = service.Command
	}

	// Convert environment variables
	if len(service.Environment) > 0 {
		params.Environment = p.convertEnvironment(service.Environment)
	}

	// Convert ports
	if len(service.Ports) > 0 {
		params.Ports = p.convertPorts(service.Ports)
	}

	// Convert volumes
	if len(service.Volumes) > 0 {
		params.Volumes = p.convertServiceVolumes(service.Volumes)
	}

	// Convert networks
	if len(service.Networks) > 0 {
		params.Networks = p.convertServiceNetworks(service.Networks)
	}

	// Convert restart policy
	if service.Restart != "" {
		params.RestartPolicy = &service.Restart
	}

	// Convert working directory
	if service.WorkingDir != "" {
		params.WorkingDir = &service.WorkingDir
	}

	// Convert user
	if service.User != "" {
		params.User = &service.User
	}

	// Convert hostname
	if service.Hostname != "" {
		params.Hostname = &service.Hostname
	}

	// Convert labels
	if len(service.Labels) > 0 {
		params.Labels = service.Labels
	}

	container.Spec.ForProvider = params

	return container, nil
}

// convertEnvironment converts Docker Compose environment variables to Container environment.
func (p *Parser) convertEnvironment(env types.MappingWithEquals) []containerv1alpha1.EnvVar {
	var envVars []containerv1alpha1.EnvVar

	for key, value := range env {
		if value == nil {
			// Environment variable without value (will be inherited from host)
			envVars = append(envVars, containerv1alpha1.EnvVar{
				Name: key,
			})
		} else {
			envVars = append(envVars, containerv1alpha1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	return envVars
}

// convertPorts converts Docker Compose ports to Container ports.
func (p *Parser) convertPorts(ports []types.ServicePortConfig) []containerv1alpha1.PortSpec {
	var portSpecs []containerv1alpha1.PortSpec

	for _, port := range ports {
		portSpec := containerv1alpha1.PortSpec{
			ContainerPort: int32(port.Target),
		}

		if port.Published != "" {
			if published, err := strconv.Atoi(port.Published); err == nil {
				hostPort := int32(published)
				portSpec.HostPort = &hostPort
			}
		}

		if port.Protocol != "" {
			protocol := strings.ToUpper(port.Protocol)
			portSpec.Protocol = &protocol
		}

		if port.HostIP != "" {
			portSpec.HostIP = &port.HostIP
		}

		portSpecs = append(portSpecs, portSpec)
	}

	return portSpecs
}

// convertServiceVolumes converts Docker Compose volume mounts to Container volumes.
func (p *Parser) convertServiceVolumes(volumes []types.ServiceVolumeConfig) []containerv1alpha1.VolumeMount {
	var volumeMounts []containerv1alpha1.VolumeMount

	for _, volume := range volumes {
		volumeMount := containerv1alpha1.VolumeMount{
			MountPath: volume.Target,
		}

		if volume.Source != "" {
			// This is a bind mount or named volume
			volumeMount.Name = volume.Source

			// Set the volume source based on the type
			if volume.Type == "bind" || strings.HasPrefix(volume.Source, "/") {
				// Host path mount
				volumeMount.VolumeSource = containerv1alpha1.VolumeSource{
					HostPath: &containerv1alpha1.HostPathVolumeSource{
						Path: volume.Source,
						Type: containerv1alpha1.HostPathTypePtr(containerv1alpha1.HostPathDirectoryOrCreate),
					},
				}
			} else {
				// Named volume
				volumeMount.VolumeSource = containerv1alpha1.VolumeSource{
					Volume: &containerv1alpha1.VolumeVolumeSource{
						VolumeName: volume.Source,
					},
				}
			}
		}

		if volume.ReadOnly {
			volumeMount.ReadOnly = &volume.ReadOnly
		}

		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

// convertServiceNetworks converts Docker Compose service networks to Container networks.
func (p *Parser) convertServiceNetworks(networks map[string]*types.ServiceNetworkConfig) []containerv1alpha1.NetworkAttachment {
	var networkAttachments []containerv1alpha1.NetworkAttachment

	for networkName, config := range networks {
		attachment := containerv1alpha1.NetworkAttachment{
			Name: networkName,
		}

		if config != nil {
			if len(config.Ipv4Address) > 0 {
				attachment.IPAddress = &config.Ipv4Address
			}
			if len(config.Ipv6Address) > 0 {
				attachment.IPv6Address = &config.Ipv6Address
			}
			if len(config.Aliases) > 0 {
				attachment.Aliases = config.Aliases
			}
		}

		networkAttachments = append(networkAttachments, attachment)
	}

	return networkAttachments
}

// convertNetworks converts Docker Compose networks to NetworkDefinitions.
func (p *Parser) convertNetworks(networks types.Networks) []NetworkDefinition {
	var networkDefs []NetworkDefinition

	for name, network := range networks {
		networkDef := NetworkDefinition{
			Name:   fmt.Sprintf("%s_%s", p.projectName, name),
			Driver: network.Driver,
			Labels: network.Labels,
		}

		if network.Ipam.Driver != "" || len(network.Ipam.Config) > 0 {
			// For now, store basic IPAM info. In a full implementation,
			// we would convert types.IPAMConfig to types.IPAMConfig
			networkDef.IPAM = &types.IPAMConfig{
				Driver: network.Ipam.Driver,
			}
		}

		networkDefs = append(networkDefs, networkDef)
	}

	return networkDefs
}

// convertVolumes converts Docker Compose volumes to VolumeDefinitions.
func (p *Parser) convertVolumes(volumes types.Volumes) []VolumeDefinition {
	var volumeDefs []VolumeDefinition

	for name, volume := range volumes {
		volumeDef := VolumeDefinition{
			Name:       fmt.Sprintf("%s_%s", p.projectName, name),
			Driver:     volume.Driver,
			DriverOpts: volume.DriverOpts,
			Labels:     volume.Labels,
		}

		volumeDefs = append(volumeDefs, volumeDef)
	}

	return volumeDefs
}

// ValidateCompose validates a Docker Compose file content.
func (p *Parser) ValidateCompose(ctx context.Context, composeContent string) error {
	_, err := p.ParseCompose(ctx, composeContent)
	return err
}

// GetServiceDependencies analyzes service dependencies from the compose file.
func (p *Parser) GetServiceDependencies(project *types.Project) map[string][]string {
	dependencies := make(map[string][]string)

	for _, service := range project.Services {
		var deps []string

		// Add explicit depends_on dependencies
		for dep := range service.DependsOn {
			deps = append(deps, dep)
		}

		if len(deps) > 0 {
			dependencies[service.Name] = deps
		}
	}

	return dependencies
}
