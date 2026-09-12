# Volume

**API Version**: `volume.docker.m.crossplane.io/v1beta1`

Named Docker volumes.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.name` | string | no | Volume name (defaults to resource name) |
| `forProvider.driver` | string | no | Volume driver |
| `forProvider.driverOpts` | map | no | Driver options |
| `forProvider.labels` | map | no | Labels |

## Example

```yaml
apiVersion: volume.docker.m.crossplane.io/v1beta1
kind: Volume
metadata:
  name: example-volume
  namespace: default
spec:
  forProvider:
    name: <value>
  providerConfigRef:
    name: default
```

## Behavior

- **Create/Update/Delete**: Full managed lifecycle against the Docker daemon.
