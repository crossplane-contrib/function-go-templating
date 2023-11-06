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

* `{{ .observed.composite.resource.metadata.name }}`
* `{{ .desired.composite.resource.status.widgets }}`
* `{{ (index .desired.composed "resource-name").resource.spec.widgets }}`
* `{{ index .context "apiextensions.crossplane.io/environment" }}`

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

To mark a desired composed resource as ready, use the
`gotemplating.fn.crossplane.io/ready` annotation:

```yaml
apiVersion: s3.aws.upbound.io/v1beta1
kind: Bucket
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: bucket
    gotemplating.fn.crossplane.io/ready: True
spec: {}
```

See the [example](example) directory for examples that you can run locally using
the Crossplane CLI:

```shell
$ crossplane beta render xr.yaml composition.yaml functions.yaml
```

See the [composition functions documentation][docs-functions] to learn more
about `crossplane beta render`.

## Additional functions

| Name           | Description                             |
| -------------- | --------------------------------------- |
| `randomChoice` | Randomly selects one of a given strings |
| `toYaml`       | Marshals any object into a YAML string  |
| `fromYaml'     | Unmarshals a YAML string into an object |

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
