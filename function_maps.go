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
	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/json"
)

const recursionMaxNums = 1000

var funcMaps = []template.FuncMap{
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

func GetNewTemplateWithFunctionMaps(delims *v1beta1.Delims) *template.Template {
	tpl := template.New("manifests")

	if delims != nil {
		if delims.Left != nil && delims.Right != nil {
			tpl = tpl.Delims(*delims.Left, *delims.Right)
		}
	}

	for _, f := range funcMaps {
		tpl.Funcs(f)
	}
	tpl.Funcs(template.FuncMap{
		"include": initInclude(tpl),
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

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

func initInclude(t *template.Template) func(string, interface{}) (string, error) {

	includedNames := make(map[string]int)

	return func(name string, data interface{}) (string, error) {
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
		return nil
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
