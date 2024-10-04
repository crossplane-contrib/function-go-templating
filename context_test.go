package main

import (
	"testing"

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	environmentKey                  = "apiextensions.crossplane.io/environment"
	contextFromEnvironment          = `{"apiextensions.crossplane.io/environment":{"complex":{"a":"b","c":{"d":"e","f":"1","overWrite": "fromContext"}}}}`
	malformedContextFromEnvironment = `{"apiextensions.crossplane.io/environment":{"badkey":,"complex":{"a":"b","c":{"d":"e","f":"1","overWrite": "fromContext"}}}}`
	contextNew                      = `apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
	kind: Context
	data:
	  newKey:
		hello: world
      overWrite: fromFunction`

	contextWithMergeKey = `apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: Context
data:
  "apiextensions.crossplane.io/environment":
	 update: environment
	 nestedEnvUpdate:
	   hello: world
	complex:
	  c:
	    overWrite: fromFunction
  "other-context-key":
	complex: {{ index .context "apiextensions.crossplane.io/environment" "complex" | toYaml | nindent 6 }}
  newkey:
	hello: world`
)

func TestMergeContextKeys(t *testing.T) {
	type args struct {
		key string
		val map[string]interface{}
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		us  *unstructured.Unstructured
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoContextAtKey": {
			reason: "When there is no existing context data at the key to merge, return the value",
			args: args{
				key: "newkey",
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: contextNew},
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
				val: map[string]interface{}{"hello": "world"},
			},
			want: want{
				us: &unstructured.Unstructured{
					Object: map[string]interface{}{"hello": "world"},
				},
				err: nil,
			},
		},
		"SuccessfulMerge": {
			reason: "Confirm that keys are merged with source overwriting destination",
			args: args{
				key: environmentKey,
				req: &fnv1beta1.RunFunctionRequest{
					Context: resource.MustStructJSON(contextFromEnvironment),
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: contextWithMergeKey},
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
				val: map[string]interface{}{
					"newKey": "newValue",
					"complex": map[string]any{
						"c": map[string]any{
							"overWrite": "fromFunction",
						},
					},
				},
			},
			want: want{
				us: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"complex": map[string]any{
							"a": "b",
							"c": map[string]any{
								"d":         "e",
								"f":         "1",
								"overWrite": "fromFunction",
							},
						},
						"newKey": "newValue"},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{
				log: logging.NewNopLogger(),
			}
			rsp, err := f.MergeContextKey(tc.args.key, tc.args.val, tc.args.req)

			if diff := cmp.Diff(tc.want.us, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.MergeContextKey(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}

}
