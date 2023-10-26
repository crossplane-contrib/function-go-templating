package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane/function-go-templating/input/v1beta1"
)

var (
	input        = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd"}}`
	inputTmpl    = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.observed.composite.resource.metadata.name|quote}}}}}`
	wrongTmpl    = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.invalid-key}}}}}`
	missingKind  = `{"apiVersion":"example.org/v1"}`
	xr           = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	xrWithStatus = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
)

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoInput": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf(invalidFunctionFmt, noTempError),
						},
					},
				},
			},
		},
		"CannotDecodeManifest": {
			reason: "The Function should return a fatal result if the manifest cannot be decoded",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InputSourceInline,
							Inline: &missingKind,
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf("cannot decode manifest: Object 'Kind' is missing in '%s'", missingKind),
						},
					},
				},
			},
		},
		"CannotParseTemplate": {
			reason: "The Function should return a fatal result if the template cannot be parsed",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InputSourceInline,
							Inline: &wrongTmpl,
						},
					),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot parse the provided templates: template: manifests:1: bad character U+002D '-'",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithNoChange": {
			reason: "The Function should return the desired composite resource and input composed resource without any changes.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "nochange"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InputSourceInline,
							Inline: &input,
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(input),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "nochange", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  fmt.Sprintf("I was run with input source %q", v1beta1.InputSourceInline),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(input),
							},
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithTemplating": {
			reason: "The Function should return the desired composite resource and the desired number of composed resources.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InputSourceInline,
							Inline: &inputTmpl,
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "templates", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  fmt.Sprintf("I was run with input source %q", v1beta1.InputSourceInline),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
							},
						},
					},
				},
			},
		},
		"UpdateDesiredCompositeStatus": {
			reason: "The Function should update the desired composite resource status.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "status"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InputSourceInline,
							Inline: &xrWithStatus,
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "status", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  fmt.Sprintf("I was run with input source %q", v1beta1.InputSourceInline),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xrWithStatus),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
