# getComposedConnectionDetails
The getComposedConnectionDetails function is a utility function used to facilitate the retrieval of connection details for a named observed composed resource. By accepting a function request map and a resource name, it navigates the request structure to fetch the specified composed resource's connection details. If the resource is found, it returns a map containing the connection details; if not, it returns nil, indicating the resource does not exist or its connection details are inaccessible.

## Usage

> **Note:** Connection details are populated by the provider and are only present in real observed state. They cannot be simulated with `crossplane render`.

Examples:

```golang
// Retrieve the connection details of the observed resource named "flexServer"
{{ $flexServerCD := getComposedConnectionDetails . "flexServer" }}

// Extract a value from the connection details
{{ $host := index $flexServerCD "host" }}
```
