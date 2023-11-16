package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
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
