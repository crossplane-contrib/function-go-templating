# getCompositeResource
The getCompositeResource function is a utility function used to facilitate the retrieval of a composite resources (XR) within templated configurations. Upon successful retrieval, the function returns a map containing the observed composite resource's manifest. If the resource cannot be located or is unreachable, it returns nil, indicating the absence or inaccessibility of the composite resource.
## Usage


Examples:
Given the following XR spec 
```yaml
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
spec:
  name: "example"
  location: "eastus"

```
```golang
// Retrieve the observed composite resource (XR) from the function request
{{ $xr := getCompositeResource . }}

apiVersion: example.crossplane.io/v1beta1
kind: ExampleResource
// 'Patch' values from the composite resource into the composed resource
spec:
  forProvider:
    name: {{ get $xr.spec "name" }}
    location: {{ get $xr.spec "location" }}

```
