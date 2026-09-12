# ComposeStack

**API Version**: `compose.docker.m.crossplane.io/v1beta1`

Compose-style multi-container services.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.compose` | string | no | Inline Compose YAML |
| `forProvider.composeRef` | ref | no | Compose file reference |
| `forProvider.projectName` | string | no | Project name |
| `forProvider.environment` | array | no | Env vars |
| `forProvider.serviceOverrides` | map | no | Per-service overrides |

## Example

```yaml
apiVersion: compose.docker.m.crossplane.io/v1beta1
kind: ComposeStack
metadata:
  name: example-compose
  namespace: default
spec:
  forProvider:
    compose: <value>
  providerConfigRef:
    name: default
```

## Behavior

- **Create/Update/Delete**: Full managed lifecycle against the Docker daemon.
