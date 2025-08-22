# provider-docker

[![CI](https://img.shields.io/github/actions/workflow/status/rossigee/provider-docker/ci.yml?branch=master)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/rossigee/provider-docker)
[![Version](https://img.shields.io/github/v/release/rossigee/provider-docker)][releases]
[![GitHub downloads](https://img.shields.io/github/downloads/rossigee/provider-docker/total)][releases]

[build]: https://github.com/rossigee/provider-docker/actions/workflows/ci.yml
[releases]: https://github.com/rossigee/provider-docker/releases

**🚧 STATUS: IN DEVELOPMENT** - Native Go Crossplane provider for Docker resource management

A native Go-based Crossplane provider for managing Docker resources, designed to replace complex Terraform-based compositions with clean, efficient resource management.

## Features

- **Container Management**: Create, configure, and manage Docker containers
- **Volume Management**: Docker volume lifecycle and storage management
- **Network Management**: Custom Docker network creation and configuration
- **Service Management**: Docker Compose-style multi-container services
- **Provider Status**: 🚧 Design and implementation phase

## Container Registry

- **Primary**: `ghcr.io/rossigee/provider-docker:latest`
- **Harbor**: Available via environment configuration
- **Upbound**: Available via environment configuration

## Architecture

This provider implements native Go controllers for Docker resources:

- **Container**: Individual Docker container management with full lifecycle support
- **Volume**: Docker volume creation, mounting, and cleanup
- **Network**: Custom network creation and container networking
- **Service**: Multi-container service orchestration (Docker Compose compatible)

### Core Resources

```yaml
# Container Resource
apiVersion: docker.crossplane.io/v1alpha1
kind: Container
metadata:
  name: my-app
spec:
  providerConfigRef:
    name: docker-config
  image: nginx:latest
  ports:
  - containerPort: 80
    hostPort: 8080
  environment:
    SERVER_NAME: my-app
  volumes:
  - name: data
    mountPath: /var/www/html
    volumeRef:
      name: my-app-data
```

## Local Development

### Requirements

- `docker`
- `go` (1.24+)
- `kubectl`
- `make`
- `git` (with submodules)
- `pre-commit` (optional, for development with quality gates)

### Build System Setup

This provider uses the standardized Crossplane build system. After cloning:

```bash
# Initialize build submodule (required)
git submodule update --init --recursive

# Verify build system works
make lint
```

### Common make targets

- `make build` to build the binary and docker image
- `make generate` to (re)generate additional code artifacts
- `make lint` to run linting and code quality checks
- `make test` run test suite with coverage
- `make reviewable` to run full pre-commit validation
- `make local-install` to install the provider in local cluster
- `make clean` to clean build artifacts

See all targets with `make help`

### Development Quality Gates

This project uses comprehensive quality gates to ensure code quality:

- **Pre-commit hooks**: Automatically run linting, formatting, and security checks
- **Go linting**: golangci-lint with strict rules
- **YAML/Markdown linting**: yamllint and markdownlint
- **Security scanning**: Private key detection and secret validation
- **Code formatting**: Automatic go fmt, go imports
- **Test coverage**: Unit and integration test coverage tracking

To set up pre-commit hooks:

```bash
pip install pre-commit
pre-commit install
```

### Testing

The provider includes comprehensive test coverage:

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run integration tests
make test-integration

# Check test coverage
go tool cover -html=coverage.out
```

### QuickStart Development

1. Make sure you have a kind cluster running and the config exported
2. `make local-install`
3. Apply sample resources from `examples/`

### Provider Configuration

```yaml
apiVersion: docker.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: docker-config
spec:
  host: tcp://docker.example.com:2376
  tlsVerify: true
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: docker-creds
      key: config
```

### Run operator in debugger

- `make crossplane-setup install-crds` to install crossplane in the kind cluster
- `kubectl apply -f examples/providerconfig.yaml`
- `export KUBECONFIG=.work/kind/kind-kubeconfig`
- `go run ./cmd/provider --debug`

### Crossplane Provider Mechanics

For detailed information on how Crossplane Provider works from a development perspective, see the [Crossplane documentation](https://docs.crossplane.io/knowledge-base/guides/provider-development/).

## Migration from Terraform

This provider is designed to replace complex Terraform-based Docker compositions:

### Before (Terraform-based)
```yaml
# Complex composition with multiple providers
apiVersion: docker.golder.io/v1alpha1  
kind: DockerService
metadata:
  name: my-service
spec:
  configBundle: configs.tar.gz.gpg  # Complex encryption pipeline
  composeTemplate: |
    # Embedded Docker Compose template
```

### After (Native Provider)
```yaml
# Clean, native Crossplane resource
apiVersion: docker.crossplane.io/v1alpha1
kind: Container
metadata:
  name: my-service
spec:
  providerConfigRef:
    name: docker-config
  image: my-app:latest
  ports:
  - containerPort: 8080
    hostPort: 8080
  environment:
    DATABASE_URL: postgres://db:5432/myapp
  volumes:
  - name: app-data
    mountPath: /app/data
    volumeRef:
      name: my-service-data
```

## Examples

See the `examples/` directory for comprehensive usage examples:

- `examples/container.yaml` - Basic container deployment
- `examples/service.yaml` - Multi-container service
- `examples/volumes.yaml` - Volume management
- `examples/networks.yaml` - Custom networking

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make reviewable` to ensure quality gates pass
5. Submit a pull request

### Development Principles

- Follow existing code patterns and conventions
- Add comprehensive tests for new functionality
- Update documentation for API changes
- Ensure all quality gates pass before submission

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/rossigee/provider-docker/issues)
- **Discussions**: [GitHub Discussions](https://github.com/rossigee/provider-docker/discussions)
- **Documentation**: See `docs/` directory for detailed guides

## Roadmap

- [x] Provider scaffolding and foundation
- [x] Container resource implementation
- [x] Docker client integration
- [x] Comprehensive test coverage
- [ ] Volume resource implementation
- [ ] Network resource implementation
- [ ] Service resource (Docker Compose compatibility)
- [ ] Production security features
- [ ] Migration tooling from Terraform providers
- [ ] Performance optimization and benchmarking