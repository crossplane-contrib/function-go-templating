package main

import (
	"math/rand"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
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

		"getResourceCondition": func(ct string, res map[string]any) (xpv1.Condition, error) {
			var conditioned xpv1.ConditionedStatus
			if err := fieldpath.Pave(res).GetValueInto("resource.status", &conditioned); err != nil {
				conditioned = xpv1.ConditionedStatus{}
			}

			// Return either found condition or empty one with "Unknown" status
			cond := conditioned.GetCondition(xpv1.ConditionType(ct))
			return cond, nil
		},
	},
}

func GetNewTemplateWithFunctionMaps(cfg *v1beta1.Config) *template.Template {
	tpl := template.New("manifests")

	if cfg.Delims != nil {
		if cfg.Delims.Left != nil && cfg.Delims.Right != nil {
			tpl = tpl.Delims(*cfg.Delims.Left, *cfg.Delims.Right)
		}
	}

	for _, f := range funcMaps {
		tpl.Funcs(f)
	}
	tpl.Funcs(sprig.FuncMap())

	return tpl
}
