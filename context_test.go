package main

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestMergeContext(t *testing.T) {
	type args struct {
		val map[string]interface{}
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		us  map[string]any
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
				req: &fnv1.RunFunctionRequest{
					Context: nil,
				},
				val: map[string]interface{}{"hello": "world"},
			},
			want: want{
				us:  map[string]interface{}{"hello": "world"},
				err: nil,
			},
		},
		"SuccessfulMerge": {
			reason: "Confirm that keys are merged with source overwriting destination",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Context: resource.MustStructJSON(`{"apiextensions.crossplane.io/environment":{"complex":{"a":"b","c":{"d":"e","f":"1","overWrite": "fromContext"}}}}`),
				},
				val: map[string]interface{}{
					"newKey": "newValue",
					"apiextensions.crossplane.io/environment": map[string]any{
						"complex": map[string]any{
							"c": map[string]any{
								"overWrite": "fromFunction",
							},
						},
					},
				},
			},
			want: want{
				us: map[string]interface{}{
					"apiextensions.crossplane.io/environment": map[string]any{
						"complex": map[string]any{
							"a": "b",
							"c": map[string]any{
								"d":         "e",
								"f":         "1",
								"overWrite": "fromFunction",
							},
						},
					},
					"newKey": "newValue"},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{
				log: logging.NewNopLogger(),
			}
			rsp, err := f.MergeContext(tc.args.req, tc.args.val)

			if diff := cmp.Diff(tc.want.us, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.MergeContext(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}

}
