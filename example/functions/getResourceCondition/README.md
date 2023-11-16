# getResourceCondition

## Usage

```golang
{{ getResourceCondition $conditionType $resource }}
{{ $resource | getResourceCondition $conditionType }}
```

Examples:

```golang
// Print whole condition
{{ .observed.resources.project | getResourceCondition "Ready" | toYaml }}

// Check status
{{ if eq (.observed.resources.project | getResourceCondition "Ready").Status "True" }}
    // do something
{{ end }}
```

See example composition for more usage examples

## Example Outputs

Requested resource does not exist or does not have the requested condition

```yaml
lasttransitiontime: "0001-01-01T00:00:00Z"
message: ""
reason: ""
status: Unknown
type: Ready
```

Requested resource does have the requested condition

```yaml
lasttransitiontime: "2023-11-03T10:07:31+01:00"
message: "custom message"
reason: foo
status: "True"
type: Ready
```
