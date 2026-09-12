# Provider Docker Documentation

A Crossplane v2 provider for managing Docker resources. All managed resources are namespaced (`*.docker.m.crossplane.io/v1beta1`) with full multi-tenancy support.

## Resource Documentation

| Resource | API Group | Description |
|----------|-----------|-------------|
| [Container](resources/container.md) | `container.docker.m.crossplane.io/v1beta1` | Container lifecycle |
| [Volume](resources/volume.md) | `volume.docker.m.crossplane.io/v1beta1` | Named volumes |
| [Network](resources/network.md) | `network.docker.m.crossplane.io/v1beta1` | Custom networks |
| [ComposeStack](resources/compose.md) | `compose.docker.m.crossplane.io/v1beta1` | Compose-style services |
| ProviderConfig | `docker.m.crossplane.io/v1beta1` | Daemon connection (cluster-scoped) |

## API Coverage Gaps

Docker Engine API surface not yet modeled: images (pull/build/push/prune as resources), Swarm services/tasks/networks, secrets and configs objects, container exec/attach sessions, log export, checkpoint/restore, and daemon event subscriptions (observe is poll-based).
