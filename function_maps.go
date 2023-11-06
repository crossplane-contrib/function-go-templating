package main

import (
	"gopkg.in/yaml.v3"
	"math/rand"
	"text/template"
	"time"

	sprig "github.com/Masterminds/sprig/v3"
)

var funcMaps = []template.FuncMap{
	{
		"randomChoice": func(choices ...string) string {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			return choices[r.Intn(len(choices))]
		},

		"toYaml": func(val any) (string, error) {
			res, err := yaml.Marshal(val)
			if err != nil {
				return "", err
			}
			return string(res), nil
		},

		"fromYaml": func(val string) (any, error) {
			var res any
			err := yaml.Unmarshal([]byte(val), &res)
			return res, err
		},
	},
}

func GetNewTemplateWithFunctionMaps() *template.Template {
	tpl := template.New("manifests")

	for _, f := range funcMaps {
		tpl.Funcs(f)
	}
	tpl.Funcs(sprig.FuncMap())

	return tpl
}
