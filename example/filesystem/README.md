# The `FileSystem` source

The `FileSystem` source loads templates from a directory in the function's
container. In a cluster a ConfigMap provides that directory.

You can create a ConfigMap with the templates using the following command:

```shell
kubectl create configmap templates --from-file=templates.tmpl -n crossplane-system
```

This ConfigMap will be mounted to the function pod and the templates will be
available in the `/templates` directory. Please see `functions.yaml` for details.

## Testing This Function Locally

`crossplane render` starts the function without the templates mounted. Start
the function yourself, with `templates.tmpl` mounted in the `/templates`
directory:

```shell
docker run --rm -p 127.0.0.1:9443:9443 \
  -v "$(pwd)/templates.tmpl:/templates/templates.tmpl:ro" \
  xpkg.crossplane.io/crossplane-contrib/function-go-templating:v0.13.1 --insecure
```

While the function is running in one terminal, open another terminal window and
run `crossplane render`:

```shell
crossplane render xr.yaml composition-fs.yaml functions-render.yaml
```

`functions-render.yaml` sets the `render.crossplane.io/runtime: Development`
annotation so that `crossplane render` connects to the function you started.
