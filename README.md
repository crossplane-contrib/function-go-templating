# function-go-templating

[![CI](https://github.com/crossplane-contrib/function-go-templating/actions/workflows/ci.yml/badge.svg)](https://github.com/crossplane-contrib/function-go-templating/actions/workflows/ci.yml) ![GitHub release (latest SemVer)](https://img.shields.io/github/release/crossplane-contrib/function-go-templating)

This [composition function][docs-functions] allows you to compose Crossplane
resources using [Go templates][go-templates]. If you've written a [Helm
chart][helm-chart] before, using this function will be a familiar experience.

Here's an example:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1beta1
    kind: XR
  mode: Pipeline
  pipeline:
    - step: create-a-bucket
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline:
          template: |
            apiVersion: s3.aws.upbound.io/v1beta1
            kind: Bucket
            metadata:
              annotations:
                gotemplating.fn.crossplane.io/composition-resource-name: bucket
            spec:
              forProvider:
                region: {{ .observed.composite.resource.spec.region }}
    - step: automatically-detect-ready-composed-resources
      functionRef:
        name: function-auto-ready
```

## Using this function

This function can load templates from two sources: `Inline` and `FileSystem`.

Use the `Inline` source to specify a simple template inline in your Composition.
Multiple YAML manifests can be specified using the `---` document separator.

Use the `FileSystem` source to specify a directory of templates. The
`FileSystem` source treats all files under the specified directory as templates.

The templates are passed a [`RunFunctionRequest`][bsr] as data. This means that
you can access the composite resource, any composed resources, and the function
pipeline context using notation like:

- `{{ .observed.composite.resource.metadata.name }}`
- `{{ .desired.composite.resource.status.widgets }}`
- `{{ (index .desired.composed "resource-name").resource.spec.widgets }}`
- `{{ index .context "apiextensions.crossplane.io/environment" }}`
- `{{ index .extraResources "some-bucket-by-name" }}`

This function supports all of Go's [built-in template functions][builtin]. The
above examples use the `index` function to access keys like `resource-name` that
contain periods, hyphens and other special characters. Like Helm, this function
also supports [Sprig template functions][sprig] as well as [additional functions](#additional-functions).

To return desired composite resource connection details, include a template that
produces the special `CompositeConnectionDetails` resource:

```yaml
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: CompositeConnectionDetails
data:
  connection-secret-key: connection-secret-value
```

> Note: The value of the connection secret value must be base64 encoded. This is already the case if you are referencing a key from a mananged resource's `connectionDetails` field. However, if you want to include a connection secret value from somewhere else, you will need to use the `b64enc` Sprig function:
```yaml
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: CompositeConnectionDetails
data:
  server-endpoint: {{ (index $.observed.resources "my-server").resource.status.atProvider.endpoint | b64enc }}
```

To mark a desired composed resource as ready, use the
`gotemplating.fn.crossplane.io/ready` annotation:

```yaml
apiVersion: s3.aws.upbound.io/v1beta1
kind: Bucket
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: bucket
    gotemplating.fn.crossplane.io/ready: "True"
spec: {}
```

See the [example](example) directory for examples that you can run locally using
the Crossplane CLI:

```shell
$ crossplane beta render xr.yaml composition.yaml functions.yaml
```

See the [composition functions documentation][docs-functions] to learn more
about `crossplane beta render`.

### ExtraResources

By defining one or more special `ExtraResources`, you can ask Crossplane to
retrieve additional resources from the local cluster and make them available to
your templates. See the [docs][extra-resources] for more information.

```yaml
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: ExtraResources
requirements:
  some-foo-by-name:
    # Resources can be requested either by name
    apiVersion: example.com/v1beta1
    kind: Foo
    matchName: "some-extra-foo"
  some-foo-by-labels:
    # Or by label.
    apiVersion: example.com/v1beta1
    kind: Foo
    matchLabels:
      app: my-app
  some-bar-by-a-computed-label:
    # But you can also generate them dynamically using the template, for example:
    apiVersion: example.com/v1beta1
    kind: Bar
    matchLabels:
      foo: {{ .observed.composite.resource.name }}
```

This will result in Crossplane retrieving the requested resources and making
them available to your templates under the `extraResources` key, with the
following format:

```json5
{
  "extraResources": {
    "some-foo-by-name": [
      // ... the requested bucket if found, empty otherwise ...
    ],
    "some-foo-by-labels": [
      // ... the requested buckets if found, empty otherwise ...
    ],
    // ... any other requested extra resources ...
  }
}
```

So, you can access the retrieved resources in your templates like this, for
example:

```yaml
{{- $someExtraResources := index .extraResources "some-extra-resources-key" }}
{{- range $i, $extraResource := $someExtraResources.items }}
#
# Do something for each retrieved extraResource
#
{{- end }}
```

### Writing to the Context

This function can write to the Composition [Context](https://docs.crossplane.io/latest/concepts/compositions/#function-pipeline-context). Subsequent pipeline steps will be able to access the data.

```yaml
---
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: Context
data:
  region: {{ $spec.region }}
  id: field
  array:
  - "1" 
  - "2"
```

To update Context data, match an existing key. For example, [function-environment-configs](https://github.com/crossplane-contrib/function-environment-configs)
stores data under the key `apiextensions.crossplane.io/environment`.

In this case, Environment fields `update` and `nestedEnvUpdate.hello` would be updated with new values.

```yaml
---
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: Context
data:
  "apiextensions.crossplane.io/environment":
      kind: Environment
      apiVersion: internal.crossplane.io/v1alpha1
      update: environment
      nestedEnvUpdate:
        hello: world
  otherContextData:
    test: field
```

For more information, see the example in [context](example/context).

### Updating status or creating composed resources with the composite resource's type

This function applies special logic if a resource with the composite resource's type is found in the template.

If the resource name is not set (the `gotemplating.fn.crossplane.io/composition-resource-name` meta annotation is not present), then the function **does not create composed resources** with the composite resource's type. In this case only the composite resource's **status is updated**.

For example, the following composition does not create composed resources. Rather, it updates the composite resource's status to include `dummy: cool-status`.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-update-status
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1beta1
    kind: XR
  mode: Pipeline
  pipeline:
    - step: render-templates
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline:
          template: |
            apiVersion: example.crossplane.io/v1beta1
            kind: XR
            status:
              dummy: cool-status
```

On the other hand, if the resource name is set (using the `gotemplating.fn.crossplane.io/composition-resource-name` meta annotation), then the function **creates composed resources** with the composite resource's type.

For example, the following composition will create a composed resource:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-allow-recursion
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1beta1
    kind: XR
  mode: Pipeline
  pipeline:
    - step: render-templates
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline:
          template: |
            apiVersion: example.crossplane.io/v1beta1
            kind: XR
            metadata:
              annotations:
                {{ setResourceNameAnnotation "recursive-xr" }}
            spec:
              compositionRef:
                name: example-other # make sure to avoid infinite recursion
```

> [!WARNING]
> This can lead to infinite recursion. Make sure to terminate the recursion by specifying a different `compositionRef` at some point.

For more information, see the example in [recursive](example/recursive).

## Setting Conditions on the Claim and Composite

Starting with Crossplane 1.17, Composition authors can set custom Conditions on the
Composite and the Claim.

Add a `ClaimConditions` to your template to set Conditions:

```yaml
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: ClaimConditions
conditions:
# Guide to ClaimConditions fields:
# Type of the condition, e.g. DatabaseReady.
# 'Healthy', 'Ready' and 'Synced' are reserved for use by Crossplane and this function will raise an error if used
# - type:  
# Status of the condition. String of "True"/"False"/"Unknown"
#   status:
# Machine-readable PascalCase reason, for example "ErrorProvisioning"
#   reason:
# Optional Target. Publish Condition only to the Composite, or the Composite and the Claim (CompositeAndClaim). 
# Defaults to Composite
#   target: 
# Optional message:
#   message: 
- type: TestCondition
  status: "False"
  reason: InstallFail
  message: "failed to install"
  target: CompositeAndClaim
- type: ConditionTrue
  status: "True"
  reason: TrueCondition 
  message: we are true
  target: Composite
- type: DatabaseReady
  status: "True"
  reason: Ready
  message: Database is ready
  target: CompositeAndClaim
```

## Additional functions

| Name                                                             | Description                                                  |
|------------------------------------------------------------------|--------------------------------------------------------------|
| [`randomChoice`](example/inline)                                 | Randomly selects one of a given strings                      |
| [`toYaml`](example/functions/toYaml)                             | Marshals any object into a YAML string                       |
| [`fromYaml`](example/functions/fromYaml)                         | Unmarshals a YAML string into an object                      |
| [`getResourceCondition`](example/functions/getResourceCondition) | Helper function to retreive conditions of resources          |
| [`getComposedResource`](example/functions/getComposedResource)    | Helper function to retrieve observed composed resources      |
| [`getCompositeResource`](example/functions/getCompositeResource) | Helper function to retreive the observed composite resource |
| [`getExtraResources`](example/functions/getExtraResources)       | Helper function to retreive extra resources                  |
| [`setResourceNameAnnotation`](example/inline)                    | Returns the special resource-name annotation with given name |
| [`include`](example/functions/include)                           | Outputs template as a string                                 |

## Developing this function

This function uses [Go][go], [Docker][docker], and the [Crossplane CLI][cli] to
build functions.

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime
```

[docs-functions]: https://docs.crossplane.io/v1.14/concepts/composition-functions/
[go-templates]: https://pkg.go.dev/text/template
[helm-chart]: https://helm.sh/docs/chart_template_guide/getting_started/
[bsr]: https://buf.build/crossplane/crossplane/docs/main:apiextensions.fn.proto.v1beta1#apiextensions.fn.proto.v1beta1.RunFunctionRequest
[builtin]: https://pkg.go.dev/text/template#hdr-Functions
[sprig]: http://masterminds.github.io/sprig/
[go]: https://go.dev
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli
[extra-resources]: https://docs.crossplane.io/latest/concepts/composition-functions/#how-composition-functions-work
