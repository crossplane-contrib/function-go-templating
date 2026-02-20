package main

import (
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

const recursionMaxNums = 1000

func getFunctions() []template.FuncMap {
	return []template.FuncMap{
		{
			"randomChoice":                 randomChoice,
			"toYaml":                       toYaml,
			"fromYaml":                     fromYaml,
			"getResourceCondition":         getResourceCondition,
			"setResourceNameAnnotation":    setResourceNameAnnotation,
			"getComposedResource":          getComposedResource,
			"getCompositeResource":         getCompositeResource,
			"getExtraResources":            getExtraResources,
			"getExtraResourcesFromContext": getExtraResourcesFromContext,
			"getCredentialData":            getCredentialData,
		},
	}
}

func GetNewTemplateWithFunctionMaps(delims *v1beta1.Delims) *template.Template {
	tpl := template.New("manifests")
	includedNames := make(map[string]int)

	if delims != nil {
		if delims.Left != nil && delims.Right != nil {
			tpl = tpl.Delims(*delims.Left, *delims.Right)
		}
	}

	for _, f := range getFunctions() {
		tpl.Funcs(f)
	}
	tpl.Funcs(template.FuncMap{
		"include": initInclude(tpl, includedNames),
		"tpl":     initTpl(tpl, includedNames),
	})
	// Sprig's env and expandenv can lead to information leakage (injected tokens/passwords).
	// Both Helm and ArgoCD remove these due to security implications.
	// see: https://masterminds.github.io/sprig/os.html
	sprigFuncs := sprig.FuncMap()
	delete(sprigFuncs, "env")
	delete(sprigFuncs, "expandenv")
	tpl.Funcs(sprigFuncs)

	return tpl
}

func randomChoice(choices ...string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // strong random number generation is not required

	return choices[r.Intn(len(choices))]
}

func toYaml(val any) (string, error) {
	res, err := yaml.Marshal(val)
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func fromYaml(val string) (any, error) {
	var res any
	err := yaml.Unmarshal([]byte(val), &res)

	return res, err
}

func getResourceCondition(ct string, res map[string]any) xpv1.Condition {
	var conditioned xpv1.ConditionedStatus
	if err := fieldpath.Pave(res).GetValueInto("resource.status", &conditioned); err != nil {
		if err := fieldpath.Pave(res).GetValueInto("status", &conditioned); err != nil {
			conditioned = xpv1.ConditionedStatus{}
		}
	}

	// Return either found condition or empty one with "Unknown" status
	return conditioned.GetCondition(xpv1.ConditionType(ct))
}

func setResourceNameAnnotation(name string) string {
	return fmt.Sprintf("gotemplating.fn.crossplane.io/composition-resource-name: %s", name)
}

func initTpl(parent *template.Template, includedNames map[string]int) func(string, any) (string, error) {
	// see https://github.com/helm/helm/blob/261233caec499c18602c61ac32507fa4656ebc9b/pkg/engine/engine.go#L148
	return func(tpl string, vals interface{}) (string, error) {
		t, err := parent.Clone()
		t.Option("missingkey=zero")
		if err != nil {
			return "", errors.Wrapf(err, "cannot clone template")
		}

		t.Funcs(template.FuncMap{
			"include": initInclude(t, includedNames),
			"tpl":     initTpl(t, includedNames),
		})

		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", errors.Wrapf(err, "cannot parse template %q", tpl)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}

		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

func initInclude(t *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(name string, data any) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
}

func getComposedResource(req map[string]any, name string) map[string]any {
	var cr map[string]any
	path := fmt.Sprintf("observed.resources[%s]resource", name)
	if err := fieldpath.Pave(req).GetValueInto(path, &cr); err != nil {
		return nil
	}

	return cr
}

func getCompositeResource(req map[string]any) map[string]any {
	var cr map[string]any
	if err := fieldpath.Pave(req).GetValueInto("observed.composite.resource", &cr); err != nil {
		return nil
	}

	return cr
}

func getExtraResources(req map[string]any, name string) []any {
	var ers []any
	path := fmt.Sprintf("requiredResources[%s].items", name)
	if err := fieldpath.Pave(req).GetValueInto(path, &ers); err != nil {
		path := fmt.Sprintf("extraResources[%s].items", name)
		if err := fieldpath.Pave(req).GetValueInto(path, &ers); err != nil {
			return nil
		}
	}

	return ers
}

func getExtraResourcesFromContext(req map[string]any, name string) []any {
	var ers []any
	path := fmt.Sprintf("context[%s][%s].items", extraResourcesContextKey, name)
	if err := fieldpath.Pave(req).GetValueInto(path, &ers); err != nil {
		return nil
	}

	return ers
}

func getCredentialData(mReq map[string]any, credName string) map[string][]byte {
	req, err := convertFromMap(mReq)
	if err != nil {
		return nil
	}

	switch req.GetCredentials()[credName].GetSource().(type) {
	case *fnv1.Credentials_CredentialData:
		return req.GetCredentials()[credName].GetCredentialData().GetData()
	default:
		return nil
	}
}

func convertFromMap(mReq map[string]any) (*fnv1.RunFunctionRequest, error) {
	jReq, err := json.Marshal(&mReq)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal map[string]any to json")
	}

	req := &fnv1.RunFunctionRequest{}
	if err := protojson.Unmarshal(jReq, req); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal request from json to proto")
	}

	return req, nil
}
