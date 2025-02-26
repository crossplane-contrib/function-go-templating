# getCredentialData
The getCredentialData function is a utility function used to facilitate the retrieval of a function credential. Upon successful retrieval, the function returns the data of the credential. If the credential cannot be located or is unreachable, it returns nil.

## Testing This Function Locally

You can run your function locally and test it with [`crossplane render`](https://docs.crossplane.io/v1.18/cli/command-reference/#render/)

```shell {copy-lines="1-3"}
crossplane render xr.yaml composition.yaml functions.yaml \
    --function-credentials=credentials.yaml \
    --include-context
---
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
---
apiVersion: render.crossplane.io/v1beta1
fields:
  password: bar
  username: foo
kind: Context
```
