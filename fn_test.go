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

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
)

var (
	cd                    = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd"}}`
	cdTmpl                = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd","labels":{"belongsTo":{{.observed.composite.resource.metadata.name|quote}}}}}`
	cdWrongTmpl           = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.invalid-key}}}}}`
	cdMissingKind         = `{"apiVersion":"example.org/v1"}`
	cdMissingResourceName = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd"}}`
	cdWithReadyWrong      = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd","meta.gotemplating.fn.crossplane.io/ready":"wrongValue"},"name":"cool-cd"}}`
	cdWithReadyTrue       = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd","meta.gotemplating.fn.crossplane.io/ready":"True"},"name":"cool-cd"}}`

	metaResourceInvalid = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"InvalidMeta"}`
	metaResourceConDet  = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"CompositeConnectionDetails","data":{"key":"dmFsdWU="}}` // encoded string "value"

	xr           = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	xrWithStatus = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"ready":"true"}}`

	path      = "testdata/templates"
	wrongPath = "testdata/wrong"
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
		"WrongInputSourceType": {
			reason: "The Function should return a fatal result if the cd source type is wrong",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: "wrong",
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot get the function input: invalid input source: wrong",
						},
					},
				},
			},
		},
		"NoInput": {
			reason: "The Function should return a fatal result if no cd was specified",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot get the function input: invalid input source: ",
						},
					},
				},
			},
		},
		"NoResourceNameAnnotation": {
			reason: "The Function should return a fatal result if the cd does not have a composition-resource-name annotation",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdMissingResourceName},
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf("cannot get composition resource name of cool-cd: %s annotation not found", annotationKeyCompositionResourceName),
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
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdMissingKind},
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf("cannot decode manifest: Object 'Kind' is missing in '%s'", cdMissingKind),
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
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdWrongTmpl},
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
			reason: "The Function should return the desired composite resource and cd composed resource without any changes.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "nochange"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cd},
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
								Resource: resource.MustStructJSON(cd),
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
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.InlineSource),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithTemplating": {
			reason: "The Function should return the desired composite resource and the templated composed resources.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdTmpl},
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
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.InlineSource),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
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
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: xrWithStatus},
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
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.InlineSource),
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
		"ResponseIsReturnedWithTemplatingFS": {
			reason: "The Function should return the desired composite resource and the templated composed resources with FileSystem cd.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.InputSourceFileSystem{DirPath: path},
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
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.FileSystemSource),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"annotations":{"crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
							},
						},
					},
				},
			},
		},
		"CannotReadTemplatesFromFS": {
			reason: "The Function should return a fatal result if the templates cannot be read from the filesystem.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.InputSourceFileSystem{DirPath: wrongPath},
						},
					),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot get the function input: cannot read tmpl from the folder {testdata/wrong}: lstat testdata/wrong: no such file or directory",
						},
					},
				},
			},
		},
		"ReadyStatusAnnotationNotValid": {
			reason: "The Function should return a fatal result if the ready annotation is not valid.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdWithReadyWrong},
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
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf(errFmtInvalidFunction, fmt.Sprintf(errFmtInvalidReadyValue, "wrongValue")),
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
		"ReadyStatusAnnotation": {
			reason: "The Function should return desired composed resource with True ready state.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: cdWithReadyTrue},
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
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.InlineSource),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
								Ready:    1,
							},
						},
					},
				},
			},
		},
		"InvalidMetaKind": {
			reason: "The Function should return a fatal result if the meta kind is invalid.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: metaResourceInvalid},
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
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf(errFmtInvalidMetaType, "InvalidMeta"),
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
		"CompositeConnectionDetails": {
			reason: "The Function should return the desired composite with CompositeConnectionDetails.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.Input{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.InputSourceInline{Template: metaResourceConDet},
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
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  fmt.Sprintf("I was run with cd source %q", v1beta1.InlineSource),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource:          resource.MustStructJSON(xr),
							ConnectionDetails: map[string][]byte{"key": []byte("value")},
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
