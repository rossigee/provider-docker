# Crossplane Docker Provider

A native Go-based Crossplane provider for managing Docker resources, replacing complex Terraform-based compositions with clean, efficient resource management.

## Project Overview

**Status**: Design Phase  
**Target**: Replace terraform-based docker-stacks system  
**Timeline**: 8-10 weeks to MVP  
**Language**: Go + Crossplane SDK  

## Background & Motivation

The current docker-stacks implementation uses a complex Terraform composition with multiple failure points:
- Terraform → Docker → Kubernetes provider chain
- GPG encryption/decryption pipeline 
- Multiple workspace orchestration
- Difficult debugging and troubleshooting
- Resource overhead from Terraform executions

A native Go provider eliminates these issues while providing better performance, reliability, and Crossplane integration.

## Architecture

### Core Resources

```go
// Container - Individual Docker container management
type Container struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ContainerSpec   `json:"spec"`
    Status            ContainerStatus `json:"status,omitempty"`
}

type ContainerSpec struct {
    // Docker connection
    ProviderConfigRef xpv1.Reference `json:"providerConfigRef"`
    
    // Container configuration
    Image           string                 `json:"image"`
    Command         []string              `json:"command,omitempty"`
    Args            []string              `json:"args,omitempty"`
    Environment     map[string]string     `json:"environment,omitempty"`
    Ports           []PortSpec            `json:"ports,omitempty"`
    Volumes         []VolumeMount         `json:"volumes,omitempty"`
    NetworkMode     string                `json:"networkMode,omitempty"`
    RestartPolicy   string                `json:"restartPolicy,omitempty"`
    
    // Security
    SecurityOptions []string              `json:"securityOptions,omitempty"`
    Capabilities    *SecurityCapabilities `json:"capabilities,omitempty"`
    ReadOnlyRootFS  *bool                `json:"readOnlyRootFS,omitempty"`
    RunAsUser       *int64               `json:"runAsUser,omitempty"`
    
    // Resources
    Resources       *ResourceRequirements `json:"resources,omitempty"`
    
    // Health check
    HealthCheck     *HealthCheckSpec     `json:"healthCheck,omitempty"`
    
    // Labels and metadata
    Labels          map[string]string    `json:"labels,omitempty"`
}

// Service - Docker Compose-style multi-container services
type Service struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ServiceSpec   `json:"spec"`
    Status            ServiceStatus `json:"status,omitempty"`
}

type ServiceSpec struct {
    ProviderConfigRef xpv1.Reference `json:"providerConfigRef"`
    
    // Service definition (Docker Compose compatible)
    Services map[string]ServiceDefinition `json:"services"`
    Networks map[string]NetworkDefinition `json:"networks,omitempty"`
    Volumes  map[string]VolumeDefinition  `json:"volumes,omitempty"`
    
    // Global configuration
    Suspended *bool `json:"suspended,omitempty"`
}

// Volume - Docker volume management
type Volume struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              VolumeSpec   `json:"spec"`
    Status            VolumeStatus `json:"status,omitempty"`
}

// Network - Docker network management  
type Network struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              NetworkSpec   `json:"spec"`
    Status            NetworkStatus `json:"status,omitempty"`
}
```

### Provider Configuration

```go
type ProviderConfig struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ProviderConfigSpec   `json:"spec"`
    Status            ProviderConfigStatus `json:"status,omitempty"`
}

type ProviderConfigSpec struct {
    // Docker daemon connection
    Host         string `json:"host"`                    // tcp://host:2376, unix:///var/run/docker.sock
    TLSVerify    *bool  `json:"tlsVerify,omitempty"`    // Enable TLS verification
    CertPath     string `json:"certPath,omitempty"`     // Path to certificates
    
    // Authentication via Kubernetes secrets
    Credentials ProviderCredentials `json:"credentials,omitempty"`
    
    // Connection settings
    APIVersion   string        `json:"apiVersion,omitempty"`   // Docker API version
    Timeout      *metav1.Duration `json:"timeout,omitempty"`   // Connection timeout
    Registry     *RegistryAuth    `json:"registry,omitempty"`  // Default registry auth
}

type ProviderCredentials struct {
    Source xpv1.CredentialsSource `json:"source"`
    
    // Secret reference for TLS certs, registry auth, etc.
    SecretRef *xpv1.SecretKeySelector `json:"secretRef,omitempty"`
}
```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-2)
- Provider scaffolding with crossplane/upjet or native controller-runtime
- ProviderConfig resource with Docker client connection
- Basic Container resource (create, read, update, delete)
- Unit tests and integration test framework

**Deliverables:**
- Working provider scaffold
- Container CRUD operations
- Docker client abstraction layer
- Basic CI/CD pipeline

### Phase 2: Core Resources (Weeks 3-4)
- Volume resource implementation
- Network resource implementation  
- Container volume mounting and networking
- Resource status reporting and conditions

**Deliverables:**
- Complete Container, Volume, Network resources
- Cross-resource dependencies working
- Comprehensive status reporting
- Resource cleanup and garbage collection

### Phase 3: Service Resource (Weeks 5-6)
- Service resource for Docker Compose-style deployments
- Multi-container orchestration
- Service-level networking and volume management
- Health check integration and monitoring

**Deliverables:**
- Service resource with compose compatibility
- Health check framework
- Monitoring/metrics integration
- Service lifecycle management

### Phase 4: Production Features (Weeks 7-8)
- Security hardening (capabilities, seccomp profiles)
- Resource constraints and quotas
- Log aggregation integration
- Documentation and examples

**Deliverables:**
- Production-ready security features
- Complete documentation
- Migration guide from terraform-provider-docker
- Performance benchmarks

### Phase 5: Migration & Polish (Weeks 9-10)
- Migration tooling from existing docker-stacks
- Advanced composition patterns
- Community feedback integration
- Release preparation

## Technical Decisions

### Provider Architecture
- **Native controller-runtime** vs upjet: Choose controller-runtime for full control over Docker API integration
- **Docker client**: Use official `docker/docker` Go client for maximum compatibility
- **Resource modeling**: Follow Kubernetes patterns (PodSpec-like) for familiarity

### Docker API Integration
```go
// Core client abstraction
type DockerClient interface {
    // Container operations
    ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (container.CreateResponse, error)
    ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
    ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
    ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
    
    // Volume operations  
    VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error)
    VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error)
    VolumeRemove(ctx context.Context, volumeID string, force bool) error
    
    // Network operations
    NetworkCreate(ctx context.Context, name string, options types.NetworkCreate) (types.NetworkCreateResponse, error)
    NetworkInspect(ctx context.Context, networkID string, options types.NetworkInspectOptions) (types.NetworkResource, error)
    NetworkRemove(ctx context.Context, networkID string) error
}
```

### Security Model
- TLS-first approach for remote Docker daemons
- Kubernetes Secret integration for credentials
- Security context enforcement (no-new-privileges, seccomp, capabilities)
- Resource quotas and limits

### Composition Patterns
```yaml
# Example: Replace complex terraform composition with simple XR
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: dockerservices.platform.golder.io
spec:
  group: platform.golder.io
  names:
    kind: DockerService
    plural: dockerservices
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              serviceName:
                type: string
              dockerHost:
                type: string
              composeTemplate:
                type: string
              configBundle:
                type: string
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: docker-service-composition
spec:
  compositeTypeRef:
    apiVersion: platform.golder.io/v1alpha1
    kind: DockerService
  resources:
  - name: container
    base:
      apiVersion: docker.crossplane.io/v1alpha1
      kind: Container
    patches:
    - type: FromCompositeFieldPath
      fromFieldPath: spec.dockerHost
      toFieldPath: spec.providerConfigRef.name
```

## Migration Strategy

### From Current docker-stacks System

1. **Resource Mapping**:
   - `DockerService` CRD → `Service` resource
   - Terraform compositions → Native Crossplane compositions  
   - Config bundles → ConfigMap/Secret references

2. **Deployment Approach**:
   - Parallel deployment during transition
   - Service-by-service migration (start with kubeftpd)
   - Automated migration tooling

3. **Configuration Migration**:
   ```bash
   # Migration tool pseudocode
   ./migrate-docker-stacks \
     --from-composition encrypted-config-composition.yaml \
     --to-xr dockerservice-xr.yaml \
     --provider-config assange-docker-config.yaml
   ```

## Use Case Examples

### KubeFTPD Migration
**Current** (complex Terraform composition):
```yaml
# 538 lines of complex Terraform with GPG decryption
apiVersion: docker.golder.io/v1alpha1  
kind: DockerService
metadata:
  name: kubeftpd-stack
spec:
  configBundle: kubeftpd-configs.tar.gz.gpg  # Complex encryption
  composeTemplate: |
    # Embedded Docker Compose template
```

**Future** (clean native resource):
```yaml
apiVersion: docker.crossplane.io/v1alpha1
kind: Container
metadata:
  name: kubeftpd
spec:
  providerConfigRef:
    name: assange-docker
  image: ghcr.io/rossigee/kubeftpd:test12
  ports:
  - containerPort: 2121
    hostPort: 2121
  - containerPort: 10000-10019  
    hostPort: 10000-10019
  environment:
    FTP_PORT: "2121"
    FTP_PUBLIC_IP: "172.16.2.3"
    KUBECONFIG: "/etc/kubernetes/kubeconfig"
  volumes:
  - name: kubeconfig
    mountPath: /etc/kubernetes/kubeconfig
    secretRef:
      name: kubeftpd-kubeconfig
      key: config
  - name: data
    mountPath: /data
    volumeRef:
      name: kubeftpd-data
  healthCheck:
    test: ["CMD", "wget", "--spider", "http://localhost:8080/healthz"]
    interval: 30s
    retries: 3
  securityOptions:
  - no-new-privileges:true
  - seccomp:runtime/default
  capabilities:
    add: ["NET_BIND_SERVICE", "DAC_OVERRIDE", "CHOWN", "FOWNER"]
    drop: ["ALL"]
```

### Multi-Service Deployment
```yaml
apiVersion: docker.crossplane.io/v1alpha1
kind: Service
metadata:
  name: loki-stack
spec:
  providerConfigRef:
    name: assange-docker
  services:
    loki:
      image: grafana/loki:2.9.2
      ports:
      - "3100:3100"
      volumes:
      - config:/etc/loki:ro
      - data:/loki
      command: ["-config.file=/etc/loki/loki.yaml"]
      
    fluentbit:
      image: fluent/fluent-bit:2.2
      ports:
      - "2020:2020"
      - "514:514/udp"
      volumes:
      - fluentbit-config:/fluent-bit/etc:ro
      depends_on:
      - loki
        
  volumes:
    config:
      external: true
      name: loki-config
    data:
      driver: local
      driver_opts:
        type: bind
        o: bind
        device: /share/CACHEDEV1_DATA/dockervols/loki/data
```

## Testing Strategy

### Unit Tests
- Docker client mock interface
- Resource controller logic
- Status calculation and reporting
- Error handling and retry logic

### Integration Tests  
- Real Docker daemon interaction
- Multi-resource dependencies
- Failure recovery scenarios
- Performance and resource usage

### E2E Tests
- Complete service deployments
- Migration from terraform-provider-docker
- Complex composition scenarios
- Production-like environments

## Community & Ecosystem

### Open Source Strategy
- MIT license for maximum adoption
- Clear contribution guidelines
- Comprehensive documentation
- Active community engagement

### Crossplane Ecosystem Integration
- Follow Crossplane provider standards
- Integration with popular compositions
- Marketplace presence
- Conference presentations and blogs

### Comparison with Alternatives

| Feature | provider-docker (native) | terraform-provider-docker | docker/docker API |
|---------|-------------------------|--------------------------|------------------|
| Performance | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| Crossplane Integration | ⭐⭐⭐⭐⭐ | ⭐⭐ | ❌ |
| Debugging | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| Resource Management | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ❌ |
| Composition Support | ⭐⭐⭐⭐⭐ | ⭐⭐ | ❌ |
| Community Support | 🆕 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

## Getting Started (Post-Implementation)

### Installation
```bash
# Install provider
kubectl apply -f https://raw.githubusercontent.com/crossplane-contrib/provider-docker/main/package/crds
kubectl apply -f https://raw.githubusercontent.com/crossplane-contrib/provider-docker/main/examples/provider.yaml

# Configure provider
kubectl apply -f - <<EOF
apiVersion: docker.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  host: tcp://docker.example.com:2376
  tlsVerify: true
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: docker-creds
      key: config
EOF
```

### First Container
```bash
kubectl apply -f - <<EOF
apiVersion: docker.crossplane.io/v1alpha1
kind: Container
metadata:
  name: hello-world
spec:
  providerConfigRef:
    name: default
  image: hello-world
  restartPolicy: "no"
EOF
```

## Next Steps

1. **Create GitHub repository**: `crossplane-contrib/provider-docker`
2. **Set up development environment**: Go workspace, Docker testing setup
3. **Implement Phase 1**: Provider scaffold and basic Container resource
4. **Community engagement**: Crossplane Slack, design reviews, RFC process
5. **Integration with existing systems**: Migration from docker-stacks terraform

This provider will significantly simplify Docker resource management in Crossplane environments while providing better performance, reliability, and debugging capabilities than the current Terraform-based approach.