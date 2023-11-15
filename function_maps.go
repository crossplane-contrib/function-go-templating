package main

import (
	"encoding/json"
	"math/rand"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
	k8sv1 "k8s.io/api/core/v1"

	sprig "github.com/Masterminds/sprig/v3"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/errors"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
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

		"getObservedResourceCondition": func(req map[string]any, resName string, condName string) (*xpv1.Condition, error) {
			// Convert map to JSON string
			reqJson, err := json.Marshal(req)
			if err != nil {
				return nil, errors.Wrap(err, "cannot marshal into json")
			}

			// Unmarshal JSON string into struct
			var rfr *fnv1beta1.RunFunctionRequest
			err = json.Unmarshal(reqJson, &rfr)
			if err != nil {
				return nil, errors.Wrap(err, "cannot unmarshal into RunFunctionRequest")
			}

			// Get observed composed resources, if any
			ocr, err := request.GetObservedComposedResources(rfr)
			if err != nil {
				return &xpv1.Condition{Status: k8sv1.ConditionUnknown, Type: xpv1.ConditionType(condName)}, nil
			}

			// Find searched resource
			res, ok := ocr[resource.Name(resName)]
			if !ok {
				return &xpv1.Condition{Status: k8sv1.ConditionUnknown, Type: xpv1.ConditionType(condName)}, nil
			}

			// Return either found condition or empty one with "Unknown" status
			cond := res.Resource.GetCondition(xpv1.ConditionType(condName))
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
