# getComposedResource
The getComposedResource function is a utility function used to facilitate the retrieval of composed resources within templated configurations, specifically targeting observed resources. By accepting a function request map and a resource name, it navigates the complex structure of a request to fetch the specified composed resource, making it easier and more user-friendly to access nested data.  If the resource is found, it returns a map containing the resource's manifest; if not, it returns nil, indicating the resource does not exist or is inaccessible through the given path.
## Usage


Examples:

```golang
// Retrieve the observed resource named "flexServer" from the function request
{{ $flexServer := getComposedResource . "flexServer" }}

// Extract values from the observed resource 
{{ $flexServerID := get $flexServer.status.atProvider "id" }}


```
