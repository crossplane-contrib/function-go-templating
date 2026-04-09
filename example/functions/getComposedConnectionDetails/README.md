# getComposedConnectionDetails
The getComposedConnectionDetails function is a utility function used to facilitate the retrieval of connection details for a named observed composed resource. By accepting a function request map and a resource name, it navigates the request structure to fetch the specified composed resource's connection details. If the resource is found, it returns a map containing the connection details; if not, it returns nil, indicating the resource does not exist or its connection details are inaccessible.

> **Note:** Connection details are populated by the provider and are only present in real observed state. They cannot be simulated with `crossplane render`. See [crossplane/crossplane#4808](https://github.com/crossplane/crossplane/issues/4808) for related upstream work.

## Usage

Examples:

```golang
// Retrieve the connection details for observed resources named "accesskey-0" and "accesskey-1"
{{ $accesskey0 := getComposedConnectionDetails . "accesskey-0" }}
{{ $accesskey1 := getComposedConnectionDetails . "accesskey-1" }}

// Extract values from the connection details
{{ index $accesskey0 "username" }}
{{ index $accesskey1 "password" }}
```
