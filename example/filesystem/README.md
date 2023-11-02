# The `FileSystem` source

You can't run the example in this directory using `crossplane beta render`
because it loads templates from a ConfigMap.

You can create a ConfigMap with the templates using the following command:

```shell
kubectl create configmap templates --from-file=templates.tmpl -n crossplane-system
```

This ConfigMap will be mounted to the function pod and the templates will be
available in the `/templates` directory. Please see `functions.yaml` for details.
