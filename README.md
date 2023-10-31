# function-go-templating

A [Crossplane] Composition Function for Golang Templating.

## What is this?

This is a composition function which allows users to render Crossplane resources
using Go templating capabilities. With this function, users can use features like
conditionals, loops, and they can use values in the environment configs or other
resource fields to render Crossplane resources.

Currently, users can provider inputs in two different ways: inline and file system.

Here's an example of a Composition that uses a Composition Function with inline input.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xusers.aws.platformref.upbound.io
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: aws.platformref.upbound.io/v1alpha1
    kind: XUser
  mode: Pipeline
  pipeline:
    - step: render-templates
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline: |
          {{- range $i := until ( .observed.composite.resource.spec.count | int ) }}
          ---
          apiVersion: iam.aws.upbound.io/v1beta1
          kind: User
          metadata:
            name: test-user-{{ $i }}
            labels:
              testing.upbound.io/example-name: test-user-{{ $i }}
            {{ if eq $.observed.resources nil }}
              dummy: {{ randomChoice "foo" "bar" "baz" }}
            {{ else }}
              dummy: {{ ( index $.observed.resources ( print "test-user-" $i ) ).resource.metadata.labels.dummy }}
            {{ end }}
          {{-end}}
```

Notice that it has a `pipeline` (of Composition Functions) instead of an array
of `resources`.

[Crossplane]: https://crossplane.io