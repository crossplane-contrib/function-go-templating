package main

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

func Test_fromYaml(t *testing.T) {
	type args struct {
		val string
	}
	type want struct {
		rsp any
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnmarshalYaml": {
			reason: "Should return unmarshalled yaml",
			args: args{
				val: `
complexDictionary:
  scalar1: true
  list:
  - abc
  - def`,
			},
			want: want{
				rsp: map[string]interface{}{
					"complexDictionary": map[string]interface{}{
						"scalar1": true,
						"list": []interface{}{
							"abc",
							"def",
						},
					},
				},
			},
		},
		"UnmarshalYamlError": {
			reason: "Should return error when unmarshalling yaml",
			args: args{
				val: `
complexDictionary:
	  scalar1: true
`,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rsp, err := fromYaml(tc.args.val)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nfromYaml(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfromYaml(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_toYaml(t *testing.T) {
	type args struct {
		val any
	}
	type want struct {
		rsp any
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MarshalYaml": {
			reason: "Should return marshalled yaml",
			args: args{
				val: map[string]interface{}{
					"complexDictionary": map[string]interface{}{
						"scalar1": true,
						"list": []interface{}{
							"abc",
							"def",
						},
					},
				},
			},
			want: want{
				rsp: `complexDictionary:
    list:
        - abc
        - def
    scalar1: true
`,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rsp, err := toYaml(tc.args.val)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\ntoYaml(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\ntoYaml(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getResourceCondition(t *testing.T) {
	type args struct {
		ct  string
		res map[string]any
	}

	type want struct {
		rsp v1.Condition
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetCondition": {
			reason: "Should return condition",
			args: args{
				ct: "Ready",
				res: map[string]any{
					"resource": map[string]any{
						"status": map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   "Ready",
									"status": "True",
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: v1.Condition{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		"GetConditionObservedResource": {
			reason: "Should return condition, even if not wrapped in 'resource'",
			args: args{
				ct: "Ready",
				res: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":   "Ready",
								"status": "True",
							},
						},
					},
				},
			},
			want: want{
				rsp: v1.Condition{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		"GetConditionUnknown": {
			reason: "Should return an Unknown status",
			args: args{
				ct: "Ready",
				res: map[string]any{
					"resource": map[string]any{},
				},
			},
			want: want{
				rsp: v1.Condition{
					Type:   "Ready",
					Status: "Unknown",
				},
			},
		},
		"GetConditionNotFound": {
			reason: "Should return an Unknown condition when not found",
			args: args{
				ct: "Ready",
				res: map[string]any{
					"resource": map[string]any{
						"status": map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   "NotReady",
									"status": "True",
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: v1.Condition{
					Type:   "Ready",
					Status: "Unknown",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rsp := getResourceCondition(tc.args.ct, tc.args.res)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\ngetResourceCondition(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_setResourceNameAnnotation(t *testing.T) {
	type args struct {
		name string
	}
	type want struct {
		rsp string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SetAnnotationWithGivenName": {
			reason: "Should return composition resource name annotation with given name",
			args: args{
				name: "test",
			},
			want: want{
				rsp: "gotemplating.fn.crossplane.io/composition-resource-name: test",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rsp := setResourceNameAnnotation(tc.args.name)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nsetResourceNameAnnotation(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_include(t *testing.T) {
	type args struct {
		val string
	}
	type want struct {
		rsp string
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ExecTemplate": {
			reason: "Should return the executed template",
			args: args{
				val: `
{{- define "test-template" -}}
value: {{.}}
{{- end }}
{{- $var:= include "test-template" "val" -}}
Must capture output: {{$var}}`,
			},
			want: want{
				rsp: `Must capture output: value: val`,
			},
		},
		"TemplateErrorCtxNotSet": {
			reason: "Should return error if ctx not set",
			args: args{
				val: `
{{- define "test-template" -}}
value: {{.}}
{{- end }}
{{- $var:= include "test-template" -}}
Must capture output: {{$var}}
`,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"TemplateErrorTemplateNameNotSet": {
			reason: "Should return error if template name not set",
			args: args{
				val: `
{{- define "test-template" -}}
value: {{.}}
{{- end }}
{{- $var:= include -}}
Must capture output: {{$var}}
`,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	tpl := template.New("")
	tpl.Funcs(template.FuncMap{
		"include": initInclude(tpl),
	})

	for name, tc := range cases {
		_tpl := template.Must(tpl.Parse(tc.args.val))
		t.Run(name, func(t *testing.T) {
			rsp := &bytes.Buffer{}
			err := _tpl.Execute(rsp, nil)
			if diff := cmp.Diff(tc.want.rsp, rsp.String(), protocmp.Transform()); diff != "" {
				t.Errorf("%s\nfromYaml(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfromYaml(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getComposedResource(t *testing.T) {
	type args struct {
		req  map[string]any
		name string
	}

	type want struct {
		rsp map[string]any
	}

	completeResource := map[string]any{
		"apiVersion": "dbforpostgresql.azure.upbound.io/v1beta1",
		"kind":       "FlexibleServer",
		"spec": map[string]any{
			"forProvider": map[string]any{
				"storageMb": "32768",
			},
		},
		"status": map[string]any{
			"atProvider": map[string]any{
				"id": "abcdef",
			},
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetrieveCompleteResource": {
			reason: "Should successfully retrieve the complete resource",
			args: args{
				req: map[string]any{
					"observed": map[string]any{
						"resources": map[string]any{
							"flexserver": map[string]any{
								"resource": completeResource,
							},
						},
					},
				},
				name: "flexserver",
			},
			want: want{rsp: completeResource},
		},
		"RetrieveCompleteResourceWithDots": {
			reason: "Should successfully retrieve the complete resource when identifier contains dots",
			args: args{
				req: map[string]any{
					"observed": map[string]any{
						"resources": map[string]any{
							"flex.server": map[string]any{
								"resource": completeResource,
							},
						},
					},
				},
				name: "flex.server",
			},
			want: want{rsp: completeResource},
		},
		"ResourceNotFound": {
			reason: "Should return nil if the resource is not found",
			args: args{
				req: map[string]any{
					"observed": map[string]any{
						"resources": map[string]any{},
					},
				},
				name: "missingResource",
			},
			want: want{rsp: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getComposedResource(tc.args.req, tc.args.name)
			if diff := cmp.Diff(tc.want.rsp, got); diff != "" {
				t.Errorf("%s\ngetComposedResource(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getCompositeResource(t *testing.T) {
	type args struct {
		req map[string]any
	}

	type want struct {
		rsp map[string]any
	}

	compositeResource := map[string]any{
		"apiVersion": "example.crossplane.io/v1beta1",
		"kind":       "XR",
		"metadata": map[string]any{
			"name": "example",
		},
		"spec": map[string]any{
			"key": "value",
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetrieveCompositeResource": {
			reason: "Should successfully retrieve the composite resource",
			args: args{
				req: map[string]any{
					"observed": map[string]any{
						"composite": map[string]any{
							"resource": compositeResource,
						},
					},
				},
			},
			want: want{rsp: compositeResource},
		},
		"ResourceNotFound": {
			reason: "Should return nil if the composite resource is not found",
			args: args{
				req: map[string]any{
					"observed": map[string]any{
						"composite": map[string]any{},
					},
				},
			},
			want: want{rsp: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getCompositeResource(tc.args.req)
			if diff := cmp.Diff(tc.want.rsp, got); diff != "" {
				t.Errorf("%s\ngetCompositeResource(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getExtraResources(t *testing.T) {
	type args struct {
		req  map[string]any
		name string
	}

	type want struct {
		rsp []any
	}

	completeResource := map[string]any{
		"apiVersion": "dbforpostgresql.azure.upbound.io/v1beta1",
		"kind":       "FlexibleServer",
		"spec": map[string]any{
			"forProvider": map[string]any{
				"storageMb": "32768",
			},
		},
		"status": map[string]any{
			"atProvider": map[string]any{
				"id": "abcdef",
			},
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetrieveCompleteResource": {
			reason: "Should successfully retrieve the complete resource",
			args: args{
				req: map[string]any{
					"requiredResources": map[string]any{
						"flexserver": map[string]any{
							"items": []any{
								completeResource,
							},
						},
					},
				},
				name: "flexserver",
			},
			want: want{
				rsp: []any{
					completeResource,
				},
			},
		},
		"ResourceNotFound": {
			reason: "Should return empty list if no extra resources are found",
			args: args{
				req: map[string]any{
					"requiredResources": map[string]any{
						"flexserver": map[string]any{
							"items": []any{},
						},
					},
				},
				name: "flexserver",
			},
			want: want{rsp: []any{}},
		},
		"NoExtraResources": {
			reason: "Should return nil if no extra resources are available",
			args: args{
				req:  map[string]any{},
				name: "flexserver",
			},
			want: want{rsp: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getExtraResources(tc.args.req, tc.args.name)
			if diff := cmp.Diff(tc.want.rsp, got); diff != "" {
				t.Errorf("%s\ngetExtraResources(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getExtraResourcesFromContext(t *testing.T) {
	type args struct {
		req  map[string]any
		name string
	}

	type want struct {
		rsp []any
	}

	completeResource := map[string]any{
		"apiVersion": "dbforpostgresql.azure.upbound.io/v1beta1",
		"kind":       "FlexibleServer",
		"spec": map[string]any{
			"forProvider": map[string]any{
				"storageMb": "32768",
			},
		},
		"status": map[string]any{
			"atProvider": map[string]any{
				"id": "abcdef",
			},
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetrieveCompleteResource": {
			reason: "Should successfully retrieve the complete resource",
			args: args{
				req: map[string]any{
					"context": map[string]any{
						"apiextensions.crossplane.io/extra-resources": map[string]any{
							"flexserver": map[string]any{
								"items": []any{
									completeResource,
								},
							},
						},
					},
				},
				name: "flexserver",
			},
			want: want{
				rsp: []any{
					completeResource,
				},
			},
		},
		"ResourceNotFound": {
			reason: "Should return empty list if no extra resources are found",
			args: args{
				req: map[string]any{
					"context": map[string]any{
						"apiextensions.crossplane.io/extra-resources": map[string]any{
							"flexserver": map[string]any{
								"items": []any{},
							},
						},
					},
				},
				name: "flexserver",
			},
			want: want{rsp: []any{}},
		},
		"NoExtraResources": {
			reason: "Should return nil if no extra resources are available",
			args: args{
				req:  map[string]any{},
				name: "flexserver",
			},
			want: want{rsp: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getExtraResourcesFromContext(tc.args.req, tc.args.name)
			if diff := cmp.Diff(tc.want.rsp, got); diff != "" {
				t.Errorf("%s\ngetExtraResourcesFromContext(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_getCredentialData(t *testing.T) {
	type args struct {
		req *fnv1.RunFunctionRequest
	}

	type want struct {
		data map[string][]byte
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetrieveFunctionCredential": {
			reason: "Should successfully retrieve the function credential",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Credentials: map[string]*fnv1.Credentials{
						"foo-creds": {
							Source: &fnv1.Credentials_CredentialData{
								CredentialData: &fnv1.CredentialData{
									Data: map[string][]byte{
										"password": []byte("secret"),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				data: map[string][]byte{
					"password": []byte("secret"),
				},
			},
		},
		"FunctionCredentialNotFound": {
			reason: "Should return nil if the function credential is not found",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Credentials: map[string]*fnv1.Credentials{},
				},
			},
			want: want{data: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req, _ := convertToMap(tc.args.req)
			got := getCredentialData(req, "foo-creds")
			if diff := cmp.Diff(tc.want.data, got); diff != "" {
				t.Errorf("%s\ngetCredentialData(...): -want data, +got data:\n%s", tc.reason, diff)
			}
		})
	}
}
