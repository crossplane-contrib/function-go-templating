# getObservedResourceCondition

## Usage

`getObservedResourceCondition $request $resourceName $conditionType`

Examples:

```golang
// Print whole condition
{{ getObservedResourceCondition . "project" "Ready" | toYaml }}

// Check status
{{ if eq (getObservedResourceCondition . "project" "Ready").Status "True" }}
    // do something
{{ end }}
```

## Example Outputs

Requested resource does not exist or does not have the requested condition

´´´yaml
lasttransitiontime: "0001-01-01T00:00:00Z"
message: ""
reason: ""
status: Unknown
type: Ready
´´´

Requested resource does have the requested condition

´´´yaml
lasttransitiontime: "2023-11-03T10:07:31+01:00"
message: "custom message"
reason: foo
status: "True"
type: Ready
´´´
