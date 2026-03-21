# Olares App Development

General rules and patterns for building Olares marketplace apps, learned from shipping multiple apps.

## Olares App Structure (Helm Chart)

```
<appname>/
  Chart.yaml
  OlaresManifest.yaml
  values.yaml
  owners
  .helmignore
  templates/deployment.yaml   # ConfigMap + Deployment + Service
  i18n/en-US/OlaresManifest.yaml
  i18n/zh-CN/OlaresManifest.yaml
```

## OlaresManifest.yaml Template

```yaml
olaresManifest.version: '0.11.0'
olaresManifest.type: app
metadata:
  name: <appname>
  appid: <appname>
  title: <Display Title — max 30 chars, [a-z0-9A-Z- ] only, no dots/underscores>
  icon: <must not be empty — valid URL>
  description: <one-line summary>
  version: <X.Y.Z>
  versionName: '<X.Y.Z>'
  categories:
    - <category>
entrances:
  - name: <appname>
    host: <appname>
    port: <app port>
    title: <Display Title — same rules as metadata.title>
    authLevel: private
spec:
  versionName: '<X.Y.Z>'
  fullDescription: |
    <detailed description>
  developer: <developer name>
  website: <project URL>
  sourceCode: <repo URL>
  submitter: <your username>
  locale:
    - en-US
    - zh-CN
  license:
    - text: <license identifier>
  category: <category>
  requiredMemory: <must be >= sum of container memory requests>
  limitedMemory: <actual memory ceiling>
  requiredCpu: <must be >= sum of container CPU requests>
  limitedCpu: <actual CPU ceiling>
  requiredGpu: <0 or 1Gi>
  limitedGpu: <0 or 24Gi>
  requiredDisk: <min disk>
  limitedDisk: <max disk>
  supportArch:
    - amd64
permission:
  appData: true
middleware: {}
options:
  apiTimeout: 0
  dependencies:
    - type: system
      name: olares
      version: '>=1.12.3-0'
```

## Chart.yaml Template

```yaml
apiVersion: v2
appVersion: '<app-specific version or identifier>'
description: '<one-line description>'
name: <appname>
type: application
version: '<X.Y.Z>'
```

## values.yaml Template

```yaml
admin: ""
bfl:
  username: ""
userspace:
  appData: ""
  appCache: ""
  userData: ""
```

## owners Template

```yaml
owners:
- '<your-github-username>'
```

## Version Sync (4 fields — miss one and the store won't update)

When bumping a version, you MUST update ALL FOUR of these to the same value:
1. `Chart.yaml` → `version`
2. `OlaresManifest.yaml` → `metadata.version`
3. `OlaresManifest.yaml` → `metadata.versionName`
4. `OlaresManifest.yaml` → `spec.versionName`

Out-of-sync versions cause apps to silently not appear or not update in the marketplace. This is the most common mistake — always grep for the old version to confirm all four are changed.

## Olares Linter Rules

These will cause silent failures or rejection if violated:

- **Name consistency**: `appid` = folder name = `name` in Chart.yaml = deployment name = service name = entrance name = entrance host. All must match exactly.
- **metadata.name** must exist in OlaresManifest.yaml (same value as appid)
- **App ID format**: all lowercase, no hyphens, no dots, no underscores
- **Title constraints**: `metadata.title` and `entrances[].title` — max 30 chars, only `[a-z0-9A-Z- ]` allowed (no dots, underscores, or special chars)
- **Icon required**: `metadata.icon` must not be empty — must be a valid URL
- **Resource floor**: `requiredMemory` >= sum of all container memory `requests`. `requiredCpu` >= sum of all container CPU `requests`. The linter rejects charts where the manifest claims less than the containers actually request.
- **Hardcoded names**: Deployment and Service names must be hardcoded to `<appname>` (NOT `{{ .Release.Name }}`)
- **Volume paths**: use `{{ .Values.userspace.appData }}` for persistent data
- **i18n**: both `i18n/en-US/OlaresManifest.yaml` and `i18n/zh-CN/OlaresManifest.yaml` must exist

## GPU Apps

If the app needs GPU access:
- Add annotation on Deployment metadata: `applications.app.bytetrade.io/gpu-inject: "true"`
- Add `nvidia.com/gpu: "1"` in container resource limits
- Some backends also need it in resource requests (test to confirm)
- Add `runtimeClassName: nvidia` to pod spec if the container requires the NVIDIA runtime

## Deployment Template Pattern

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: <appname>-env
  namespace: "{{ .Release.Namespace }}"
data:
  KEY: "value"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    io.kompose.service: <appname>
  name: <appname>
  namespace: "{{ .Release.Namespace }}"
  annotations:
    # Add gpu-inject annotation here if GPU app
spec:
  replicas: 1
  selector:
    matchLabels:
      io.kompose.service: <appname>
  strategy:
    type: Recreate
  template:
    metadata:
      creationTimestamp: null
      labels:
        io.kompose.network/chrome-default: "true"
        io.kompose.service: <appname>
    spec:
      containers:
        - name: <container-name>
          image: "<image>"
          # ... args, env, ports, probes, resources, volumeMounts
          resources:
            limits:
              cpu: "<max cpu>"
              memory: "<max memory>"
            requests:
              cpu: "<min cpu>"
              memory: "<min memory>"
          volumeMounts:
            - mountPath: "/data"
              name: appdata
      volumes:
        - name: appdata
          hostPath:
            path: "{{ .Values.userspace.appData }}/<subdir>"
            type: DirectoryOrCreate
      restartPolicy: Always
status: {}
---
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    io.kompose.service: <appname>
  name: <appname>
  namespace: "{{ .Release.Namespace }}"
spec:
  ports:
    - name: "<port-name>"
      port: <port>
      targetPort: <port>
  selector:
    io.kompose.service: <appname>
status:
  loadBalancer: {}
```

## Packaging and Validation

```bash
helm lint <appname>/                    # Always lint before packaging
helm package <appname>/ -d charts/      # Package into charts/
```

Before packaging, verify:
- All 4 version fields match
- All name fields are consistent (appid, folder, Chart.yaml, deployment, service, entrance)
- Resource requests don't exceed manifest's required* values
- i18n directories exist with valid YAML

## Common Mistakes

1. **Version desync** — forgetting one of the 4 version fields (especially `spec.versionName`)
2. **Name mismatch** — using `{{ .Release.Name }}` instead of hardcoded appname in deployment/service
3. **Title too long or has dots** — Olares silently rejects titles with `.` or over 30 chars
4. **Missing metadata.name** — appid alone is not enough, metadata.name must also be set
5. **Resource mismatch** — manifest says `requiredMemory: 8Gi` but containers request 16Gi total
