# Network

**API Version**: `network.docker.m.crossplane.io/v1beta1`

Custom Docker networks.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.name` | string | no | Network name |
| `forProvider.driver` | string | no | Network driver |
| `forProvider.internal` | bool | no | Internal-only |
| `forProvider.attachable` | bool | no | Attachable |
| `forProvider.enableIPv6` | bool | no | IPv6 |
| `forProvider.ipam` | object | no | IPAM config |

## Example

```yaml
apiVersion: network.docker.m.crossplane.io/v1beta1
kind: Network
metadata:
  name: example-network
  namespace: default
spec:
  forProvider:
    name: <value>
  providerConfigRef:
    name: default
```

## Behavior

- **Create/Update/Delete**: Full managed lifecycle against the Docker daemon.
