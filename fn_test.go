package main

import (
	"context"
	"embed"
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
	cd                    = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd"}}`
	cdTmpl                = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd","labels":{"belongsTo":{{.observed.composite.resource.metadata.name|quote}}}}}`
	cdWrongTmpl           = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.invalid-key}}}}}`
	cdMissingKind         = `{"apiVersion":"example.org/v1"}`
	cdMissingResourceName = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd"}}`
	cdWithReadyWrong      = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd","gotemplating.fn.crossplane.io/ready":"wrongValue"},"name":"cool-cd"}}`
	cdWithReadyTrue       = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd","gotemplating.fn.crossplane.io/ready":"True"},"name":"cool-cd"}}`

	metaResourceInvalid        = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"InvalidMeta"}`
	metaResourceConDet         = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"CompositeConnectionDetails","data":{"key":"dmFsdWU="}}` // encoded string "value"
	metaResourceContextInvalid = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"Context","data": 1 }`
	metaResourceContext        = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"Context","data":{"apiextensions.crossplane.io/environment":{ "new":"value"}}}`

	xr                    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	xrWithStatus          = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"ready":"true"}}`
	xrWithNestedStatusFoo = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"foo":"bar"}}}`
	xrWithNestedStatusBaz = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"baz":"qux"}}}`

	extraResources = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"cool-extra-resource"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"another-cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchLabels":{"key": "value"}},"yet-another-cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"foo"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"all-cool-resources":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchLabels":{}}}}`
	extraResourcesDuplicatedKey = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"cool-extra-resource"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"another-cool-extra-resource"}}}`

	path      = "testdata/templates"
	wrongPath = "testdata/wrong"

	//go:embed testdata
	testdataFS embed.FS
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
						&v1beta1.GoTemplate{
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
							Message:  "invalid function input: invalid source: wrong",
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
							Message:  "invalid function input: source is required",
						},
					},
				},
			},
		},
		"WrongInlineInput": {
			reason: "The Function should return a fatal result if there is no inline template provided",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: inline.template should be provided",
						},
					},
				},
			},
		},
		"WrongFileSystemInput": {
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.FileSystemSource,
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: fileSystem.dirPath should be provided",
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdMissingResourceName},
						}),
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "\"CD\" template is missing required \"" + annotationKeyCompositionResourceName + "\" annotation",
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdMissingKind},
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWrongTmpl},
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cd},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{},"name":"cool-cd"}}`),
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdTmpl},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"annotations":{},"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrWithStatus},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xrWithStatus),
						},
					},
				},
			},
		},
		"UpdateDesiredCompositeNestedStatus": {
			reason: "The Function should update the desired composite resource nested status.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "status"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrWithNestedStatusBaz},
						}),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xrWithNestedStatusFoo),
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xrWithNestedStatusFoo),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "status", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"foo":"bar","baz":"qux"}}}`),
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
						&v1beta1.GoTemplate{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.TemplateSourceFileSystem{DirPath: path},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"annotations":{},"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
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
						&v1beta1.GoTemplate{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.TemplateSourceFileSystem{DirPath: wrongPath},
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
							Message:  "invalid function input: cannot read tmpl from the folder {testdata/wrong}: open testdata/wrong: file does not exist",
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWithReadyWrong},
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
							Message:  "invalid function input: invalid \"" + annotationKeyReady + "\" annotation value \"wrongValue\": must be True, False, or Unspecified",
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWithReadyTrue},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{},"name":"cool-cd"}}`),
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceInvalid},
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
							Message:  "invalid kind \"InvalidMeta\" for apiVersion \"" + metaApiVersion + "\" - must be one of CompositeConnectionDetails, Context or ExtraResources",
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
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceConDet},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource:          resource.MustStructJSON(xr),
							ConnectionDetails: map[string][]byte{"key": []byte("value")},
						},
					},
				},
			},
		},
		"ContextInvalidData": {
			reason: "The Function should return an error if he context data is invalid.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceContextInvalid},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "cannot get Contexts from input: cannot unmarshal value from JSON: json: cannot unmarshal number into Go value of type map[string]interface {}",
						},
					},
				},
			},
		},
		"Context": {
			reason: "The Function should return the desired composite with updated context.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceContext},
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
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Context: resource.MustStructJSON(
						`{
							"apiextensions.crossplane.io/environment": {
							  "new": "value"
							}
						  }`,
					),
				},
			},
		},
		"ExtraResources": {
			reason: "The Function should return the desired composite with extra resources.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResources},
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
					Meta:    &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Requirements: &fnv1beta1.Requirements{
						ExtraResources: map[string]*fnv1beta1.ResourceSelector{
							"cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "cool-extra-resource",
								},
							},
							"another-cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
										Labels: map[string]string{"key": "value"},
									},
								},
							},
							"yet-another-cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "foo",
								},
							},
							"all-cool-resources": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
										Labels: map[string]string{},
									},
								},
							},
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
		"DuplicateExtraResourceKey": {
			reason: "The Function should return a fatal result if the extra resource key is duplicated.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResourcesDuplicatedKey},
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
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "duplicate extra resource key \"cool-extra-resource\"",
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{
				log:  logging.NewNopLogger(),
				fsys: testdataFS,
			}
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
