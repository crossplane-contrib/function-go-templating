package main

import (
	"math/rand"
	"text/template"
	"time"

	sprig "github.com/go-task/slim-sprig"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

var funcMaps = []template.FuncMap{
	{
		"randomChoice": func(choices ...string) string {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			return choices[r.Intn(len(choices))]
		},
	},
	{
		"getComposedResource": func(m map[string]interface{}, name string) map[string]interface{} {
			paved := fieldpath.Pave(m)

			r, err := paved.GetValue(name)
			if err != nil {
				return nil
			}

			return r.(map[string]interface{})
		},
	},
	{
		"getEnvVar": func(m map[string]interface{}, key string) string {
			env := m["apiextensions.crossplane.io/environment"].(map[string]interface{})
			paved := fieldpath.Pave(env)

			r, err := paved.GetValue(key)
			if err != nil {
				return ""
			}

			return r.(string)
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
