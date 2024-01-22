package fn

import (
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/function-sdk-go/errors"
)

const recursionMaxNums = 1000

var funcMaps = []template.FuncMap{
	{
		"randomChoice":              randomChoice,
		"toYaml":                    toYaml,
		"fromYaml":                  fromYaml,
		"getResourceCondition":      getResourceCondition,
		"setResourceNameAnnotation": setResourceNameAnnotation,
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
	tpl.Funcs(sprig.FuncMap())

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
		conditioned = xpv1.ConditionedStatus{}
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
