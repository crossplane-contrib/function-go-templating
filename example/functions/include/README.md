# getResourceCondition

## Usage

```golang
{{ include $name $context }}
```

Examples:

```golang
// Print whole condition
{{ include "template-name" . | nindent 4  }}
{{ $output:= include "template-name" . }}

```

See example composition for more usage examples