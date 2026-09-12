# Container

**API Version**: `container.docker.m.crossplane.io/v1beta1`

Docker container lifecycle.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.image` | string | yes | Image reference |
| `forProvider.name` | string | no | Container name |
| `forProvider.command` | array | no | Override entrypoint |
| `forProvider.environment` | array | no | Env vars |
| `forProvider.ports` | array | no | Port mappings |
| `forProvider.volumes` | array | no | Volume mounts |
| `forProvider.restartPolicy` | string | no | Restart policy |

## Example

```yaml
apiVersion: container.docker.m.crossplane.io/v1beta1
kind: Container
metadata:
  name: example-container
  namespace: default
spec:
  forProvider:
    image: <value>
  providerConfigRef:
    name: default
```

## Behavior

- **Create/Update/Delete**: Full managed lifecycle against the Docker daemon.
