package main

import (
	"encoding/json"
	"math/rand"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"

	sprig "github.com/Masterminds/sprig/v3"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/errors"
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

		"getResourceCondition": func(ct string, res map[string]any) (*xpv1.Condition, error) {
			resource, ok := res["resource"]
			if !ok {
				return nil, errors.New("input is not a resource")
			}

			status, ok := resource.(map[string]any)["status"]
			if !ok {
				// Just return a unknown condition for resources that do not have a status (yet)
				return &xpv1.Condition{
					Type:   xpv1.ConditionType(ct),
					Status: v1.ConditionUnknown,
				}, nil
			}

			// Convert map to JSON string
			reqJson, err := json.Marshal(status)
			if err != nil {
				return nil, errors.Wrap(err, "cannot marshal into json")
			}

			// Unmarshal JSON string into struct
			var conditioned xpv1.ConditionedStatus
			err = json.Unmarshal(reqJson, &conditioned)
			if err != nil {
				return nil, errors.Wrap(err, "cannot unmarshal into ConditionedStatus")
			}

			// Return either found condition or empty one with "Unknown" status
			cond := conditioned.GetCondition(xpv1.ConditionType(ct))
			return &cond, nil
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
