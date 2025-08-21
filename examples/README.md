# Provider Docker Examples

This directory contains example configurations demonstrating how to use the Crossplane Docker Provider.

## Overview

The Docker provider enables declarative management of Docker resources through Kubernetes manifests, replacing complex Terraform-based compositions with clean, efficient native Go implementations.

## Examples

### Basic Configuration

- **[provider-config.yaml](./provider-config.yaml)** - ProviderConfig examples for different connection types
- **[basic-container.yaml](./basic-container.yaml)** - Simple container deployments with common configurations

### Migration Examples

- **[migration-example.yaml](./migration-example.yaml)** - KubeFTPD migration from Terraform to native provider

## Quick Start

1. **Install the provider:**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: pkg.crossplane.io/v1
   kind: Provider
   metadata:
     name: provider-docker
   spec:
     package: ghcr.io/rossigee/provider-docker:v0.1.0
   EOF
   ```

2. **Configure provider connection:**
   ```bash
   kubectl apply -f examples/provider-config.yaml
   ```

3. **Deploy a container:**
   ```bash
   kubectl apply -f examples/basic-container.yaml
   ```

## Configuration Types

### Local Docker (Unix Socket)
```yaml
apiVersion: docker.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: local
spec:
  host: unix:///var/run/docker.sock
```

### Remote Docker (TCP with TLS)
```yaml
apiVersion: docker.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: remote
spec:
  host: tcp://docker.example.com:2376
  tlsConfig:
    verify: true
  credentials:
    source: Secret
    secretRef:
      name: docker-creds
      key: config
```

## Container Features

### Basic Container
- Image specification
- Port mappings
- Environment variables
- Labels and annotations
- Restart policies

### Advanced Features
- Volume mounts (host paths, secrets, configmaps)
- Security contexts and capabilities
- Resource limits and requests
- Health checks
- Network attachments
- Init containers

### Security Best Practices
- Non-root user execution
- Read-only root filesystems
- Dropped capabilities
- Security contexts
- Resource constraints

## Migration Benefits

Migrating from terraform-provider-docker provides:

1. **Performance**: Direct Docker API integration eliminates Terraform overhead
2. **Reliability**: Native Go implementation with robust error handling
3. **Debugging**: Standard Kubernetes tooling (kubectl, events, logs)
4. **Security**: Proper capability management and security contexts
5. **Maintainability**: Simplified configuration without complex compositions
6. **Integration**: Native Crossplane resource lifecycle management

## Troubleshooting

### Common Issues

1. **Connection failed**: Check Docker daemon accessibility and credentials
2. **Image pull failed**: Verify registry authentication in provider config
3. **Port binding failed**: Ensure host ports are available
4. **Permission denied**: Check security context and capabilities

### Debugging Commands

```bash
# Check provider status
kubectl get providers

# Check provider config
kubectl get providerconfigs

# Check container resource
kubectl get containers
kubectl describe container <name>

# Check provider logs
kubectl logs -n crossplane-system deployment/provider-docker
```

For more detailed troubleshooting, see the main project documentation.