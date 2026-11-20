# tpl

## Usage

```golang
{{ tpl $template $context }}
```

Examples:

```golang
//context
test:
    user: "travis"

//template
{{- $testuser := .test.user }}
{{ tpl "Welcome, {{$testuser}}" .  }}

//output
Welcome, travis
```

See example composition for more usage examples