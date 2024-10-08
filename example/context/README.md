# Writing to the Function Context

function-go-templating can write to the Function Context

## Testing This Function Locally

You can run your function locally and test it using [`crossplane render`](https://docs.crossplane.io/latest/cli/command-reference/#render)
with these example manifests.

```shell
crossplane render \
  --extra-resources environmentConfigs.yaml \
  --include-context \
  xr.yaml composition.yaml functions.yaml
```

Will produce an output like:

```shell
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
  fromEnv: e
---
apiVersion: render.crossplane.io/v1beta1
fields:
  apiextensions.crossplane.io/environment:
    apiVersion: internal.crossplane.io/v1alpha1
    array:
    - "1"
    - "2"
    complex:
      a: b
      c:
        d: e
        f: "1"
    kind: Environment
    nestedEnvUpdate:
      hello: world
    update: environment
  newkey:
    hello: world
  other-context-key:
    complex:
      a: b
      c:
        d: e
        f: "1"
kind: Context
```

## Debugging This Function

First we need to run the command in debug mode. In a terminal Window Run:

```shell
# Run the function locally
$ go run . --insecure --debug
```

Next, set the go-templating function `render.crossplane.io/runtime: Development` annotation so that
`crossplane render` communicates with the local process instead of downloading an image:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: crossplane-contrib-function-go-templating
  annotations: 
    render.crossplane.io/runtime: Development
spec:
  package: xpkg.upbound.io/crossplane-contrib/function-go-templating:v0.6.0
```

While the function is running in one terminal, open another terminal window and run `crossplane render`.
The function should output debug-level logs in the terminal.
