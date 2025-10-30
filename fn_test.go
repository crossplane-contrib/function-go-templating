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
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
)

var (
	invalidYaml = `
---
apiVersion: example.org/v1
kind: CD
metadata:
  name: %!@#$%^&*()_+
`

	cd                    = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd"}}`
	cdTmpl                = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd"},"name":"cool-cd","labels":{"belongsTo":{{.observed.composite.resource.metadata.name|quote}}}}}`
	cdMissingKeyTmpl      = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.missing | quote }}}}}`
	cdWrongTmpl           = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd","labels":{"belongsTo":{{.invalid-key}}}}}`
	cdMissingKind         = `{"apiVersion":"example.org/v1"}`
	cdMissingResourceName = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"name":"cool-cd"}}`
	cdWithReadyWrong      = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd","gotemplating.fn.crossplane.io/ready":"wrongValue"},"name":"cool-cd"}}`
	cdWithReadyTrue       = `{"apiVersion":"example.org/v1","kind":"CD","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"cool-cd","gotemplating.fn.crossplane.io/ready":"True"},"name":"cool-cd"}}`

	metaResourceInvalid        = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"InvalidMeta"}`
	metaResourceConDet         = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"CompositeConnectionDetails","data":{"key":"dmFsdWU="}}`  // encoded string "value"
	metaResourceConDet2        = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"CompositeConnectionDetails","data":{"key2":"ZXVsYXY="}}` // encoded string "eulav"
	metaResourceContextInvalid = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"Context","data": 1 }`
	metaResourceContext        = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"Context","data":{"apiextensions.crossplane.io/environment":{ "new":"value"}}}`

	xr                    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	xrWithStatus          = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"ready":"true"}}`
	xrWithNestedStatusFoo = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"foo":"bar"}}}`
	xrWithNestedStatusBaz = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"baz":"qux"}}}`
	xrRecursiveTmpl       = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"annotations":{"gotemplating.fn.crossplane.io/composition-resource-name":"recursive-xr"},"name":"recursive-xr","labels":{"belongsTo":{{.observed.composite.resource.metadata.name|quote}}}},"spec":{"count":2}}`

	claimConditions            = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ClaimConditions","conditions":[{"type":"TestCondition","status":"False","reason":"InstallFail","message":"failed to install","target":"ClaimAndComposite"},{"type":"ConditionTrue","status":"True","reason":"this condition is true","message":"we are true","target":"Composite"},{"type":"DatabaseReady","status":"True","reason":"Ready","message":"Database is ready"}]}`
	claimConditionsReservedKey = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ClaimConditions","conditions":[{"type":"Ready","status":"False","reason":"InstallFail","message":"I am using a reserved Condition","target":"ClaimAndComposite"}]}`

	extraResource  = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"cool-extra-resource"}}}`
	extraResources = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"cool-extra-resource"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"another-cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchLabels":{"key": "value"}},"yet-another-cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"foo"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"all-cool-resources":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchLabels":{}}}}`
	extraResourcesDuplicatedKey = `{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"cool-extra-resource"}}}
{"apiVersion":"meta.gotemplating.fn.crossplane.io/v1alpha1","kind":"ExtraResources","requirements":{"cool-extra-resource":{"apiVersion":"example.org/v1","kind":"CoolExtraResource","matchName":"another-cool-extra-resource"}}}`

	key       = "userkey/go-template"
	path      = "testdata/templates"
	wrongPath = "testdata/wrong"

	//go:embed testdata
	testdataFS embed.FS
)

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
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
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: "wrong",
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: invalid source: wrong",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"NoInput": {
			reason: "The Function should return a fatal result if no cd was specified",
			args: args{
				req: &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: source is required",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"WrongInlineInput": {
			reason: "The Function should return a fatal result if there is no inline template provided",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: inline.template should be provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"WrongFileSystemInput": {
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.FileSystemSource,
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: fileSystem.dirPath should be provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"WrongEnvironmentInput": {
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.EnvironmentSource,
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: environment.key should be provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"NoResourceNameAnnotation": {
			reason: "The Function should return a fatal result if the cd does not have a composition-resource-name annotation",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdMissingResourceName},
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "\"CD\" template is missing required \"" + annotationKeyCompositionResourceName + "\" annotation",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"CannotDecodeManifest": {
			reason: "The Function should return a fatal result if the manifest cannot be decoded",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdMissingKind},
						}),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  fmt.Sprintf("cannot decode manifest: Object 'Kind' is missing in '%s'", cdMissingKind),
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"CannotParseTemplate": {
			reason: "The Function should return a fatal result if the template cannot be parsed",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWrongTmpl},
						},
					),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot parse the provided templates: template: manifests:1: bad character U+002D '-'",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithNoChange": {
			reason: "The Function should return the desired composite resource and cd composed resource without any changes.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "nochange"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cd},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "nochange", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdTmpl},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "templates", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "status"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrWithStatus},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "status", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xrWithStatus),
						},
					},
				},
			},
		},
		"UpdateDesiredCompositeNestedStatus": {
			reason: "The Function should update the desired composite resource nested status.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "status"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrWithNestedStatusBaz},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xrWithNestedStatusFoo),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xrWithNestedStatusFoo),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "status", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"foo":"bar","baz":"qux"}}}`),
						},
					},
				},
			},
		},
		"MergeDesiredCompositeStatus": {
			reason: "The Function should merge all the desired composite resources.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrWithNestedStatusFoo + xrWithNestedStatusBaz},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2},"status":{"state":{"foo":"bar","baz":"qux"}}}`),
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithTemplatedXR": {
			reason: "The Function should return the desired composite resource and the composed templated XR resource.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "status"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: xrRecursiveTmpl},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "status", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"recursive-xr": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"XR","metadata":{"annotations":{},"name":"recursive-xr","labels":{"belongsTo":"cool-xr"}},"spec":{"count":2}}`),
							},
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithTemplatingFS": {
			reason: "The Function should return the desired composite resource and the templated composed resources with FileSystem cd.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.TemplateSourceFileSystem{DirPath: path},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "templates", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:     v1beta1.FileSystemSource,
							FileSystem: &v1beta1.TemplateSourceFileSystem{DirPath: wrongPath},
						},
					),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot read tmpl from the folder {testdata/wrong}: open testdata/wrong: file does not exist",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"ResponseIsReturnedWithTemplatingEnvironment": {
			reason: "The Function should return the desired composite resource and the templated composed resources with Environment cd.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Context: resource.MustStructJSON(`{"apiextensions.crossplane.io/environment": {"userkey/go-template": "apiVersion: example.org/v1\nkind: CD\nmetadata:\n  name: cool-cd\n  annotations:\n    gotemplating.fn.crossplane.io/composition-resource-name: cool-cd\n  labels:\n    belongsTo: {{ .observed.composite.resource.metadata.name|quote }}"}}`),
					Meta:    &fnv1.RequestMeta{Tag: "templates"},
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:      v1beta1.EnvironmentSource,
							Environment: &v1beta1.TemplateSourceEnvironment{Key: key},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Context: resource.MustStructJSON(`{"apiextensions.crossplane.io/environment": {"userkey/go-template":"apiVersion: example.org/v1\nkind: CD\nmetadata:\n  name: cool-cd\n  annotations:\n    gotemplating.fn.crossplane.io/composition-resource-name: cool-cd\n  labels:\n    belongsTo: {{ .observed.composite.resource.metadata.name|quote }}"}}`),
					Meta:    &fnv1.ResponseMeta{Tag: "templates", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CD","metadata":{"annotations":{},"name":"cool-cd","labels":{"belongsTo":"cool-xr"}}}`),
							},
						},
					},
				},
			},
		},
		"CannotReadTemplatesFromEnvironment": {
			reason: "The Function should return a fatal result if the templates cannot be read from the environment.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Context: resource.MustStructJSON(`{}`),
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:      v1beta1.EnvironmentSource,
							Environment: &v1beta1.TemplateSourceEnvironment{Key: key},
						},
					),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Context: resource.MustStructJSON(`{}`),
					Meta:    &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot read tmpl from the environment: apiextensions.crossplane.io/environment key does not exist in context",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"CannotReadTemplatesFromEnvironmentKey": {
			reason: "The Function should return a fatal result if the templates cannot be read from the environment.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Context: resource.MustStructJSON(`{"apiextensions.crossplane.io/environment": {}}`),
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:      v1beta1.EnvironmentSource,
							Environment: &v1beta1.TemplateSourceEnvironment{Key: key},
						},
					),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Context: resource.MustStructJSON(`{"apiextensions.crossplane.io/environment": {}}`),
					Meta:    &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: cannot read tmpl from the environment: key: userkey/go-template does not exist",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"ReadyStatusAnnotationNotValid": {
			reason: "The Function should return a fatal result if the ready annotation is not valid.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWithReadyWrong},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid function input: invalid \"" + annotationKeyReady + "\" annotation value \"wrongValue\": must be True, False, or Unspecified",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"ReadyStatusAnnotation": {
			reason: "The Function should return desired composed resource with True ready state.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: cdWithReadyTrue},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceInvalid},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "invalid kind \"InvalidMeta\" for apiVersion \"" + metaApiVersion + "\" - must be one of CompositeConnectionDetails, Context or ExtraResources",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"ClaimConditionsError": {
			reason: "The Function should return a fatal result if a reserved Condition is set.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: claimConditionsReservedKey},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot set ClaimCondition type: Ready is a reserved Crossplane Condition",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"ClaimConditions": {
			reason: "The Function should correctly set ClaimConditions.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: claimConditions},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Conditions: []*fnv1.Condition{
						{
							Type:    "TestCondition",
							Status:  fnv1.Status_STATUS_CONDITION_FALSE,
							Reason:  "InstallFail",
							Message: ptr.To("failed to install"),
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:    "ConditionTrue",
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Reason:  "this condition is true",
							Message: ptr.To("we are true"),
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
						{
							Type:    "DatabaseReady",
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Reason:  "Ready",
							Message: ptr.To("Database is ready"),
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"CompositeConnectionDetails": {
			reason: "The Function should return the desired composite with CompositeConnectionDetails.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceConDet},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource:          resource.MustStructJSON(xr),
							ConnectionDetails: map[string][]byte{"key": []byte("value")},
						},
					},
				},
			},
		},
		"MergeCompositeConnectionDetails": {
			reason: "The Function should merge all CompositeConnectionDetails.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceConDet + metaResourceConDet2},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource:          resource.MustStructJSON(xr),
							ConnectionDetails: map[string][]byte{"key": []byte("value"), "key2": []byte("eulav")},
						},
					},
				},
			},
		},
		"ContextInvalidData": {
			reason: "The Function should return an error if he context data is invalid.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceContextInvalid},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot get Contexts from input: cannot unmarshal value from JSON: json: cannot unmarshal number into Go value of type map[string]interface {}",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"Context": {
			reason: "The Function should return the desired composite with updated context.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: metaResourceContext},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResources},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "cool-extra-resource",
								},
							},
							"another-cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{"key": "value"},
									},
								},
							},
							"yet-another-cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "foo",
								},
							},
							"all-cool-resources": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{},
									},
								},
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
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
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResourcesDuplicatedKey},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "duplicate extra resource key \"cool-extra-resource\"",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1.Resource{
							"cool-cd": {
								Resource: resource.MustStructJSON(cd),
							},
						},
					},
				},
			},
		},
		"InvalidTemplateOption": {
			reason: "The Function should return error when an invalid option is provided.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:  v1beta1.InlineSource,
							Inline:  &v1beta1.TemplateSourceInline{Template: cdMissingKeyTmpl},
							Options: &[]string{"missingoption=nothing"},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot apply template options: panic occurred while applying template options: unrecognized option: missingoption=nothing",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"TemplateOptionsMissingKeyError": {
			reason: "The Function should panic if missingkey=error is provided as template option.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source:  v1beta1.InlineSource,
							Inline:  &v1beta1.TemplateSourceInline{Template: cdMissingKeyTmpl},
							Options: &[]string{"missingkey=error"},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot execute template: template: manifests:1:96: executing \"manifests\" at <.missing>: map has no entry for key \"missing\"",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"PrintYamlErrorLine": {
			reason: "The Function should print the line content when invalid YAML is provided.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: invalidYaml},
						}),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot decode manifest: error converting YAML to JSON: yaml: line 6 (document 1, line 4) near: 'name: %!@#$%^&*()_+': found character that cannot start any token",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"AddExtraResourcesToContext": {
			reason: "The Function should add extra resources to context.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResource},
						}),
					RequiredResources: map[string]*fnv1.Resources{
						"cool-extra-resource": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CoolExtraResource","metadata":{"name":"cool-extra-resource"},"spec":{}}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Context: resource.MustStructJSON(
						`{
							"apiextensions.crossplane.io/extra-resources": {
								"cool-extra-resource": {
								    "items": [
									    {
											"resource": {
												"apiVersion": "example.org/v1",
												"kind": "CoolExtraResource",
												"metadata": {
													"name": "cool-extra-resource"
												},
												"spec": {}
											}
										}
									]
								}
							}
						}`,
					),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON("{}"),
						},
					},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "cool-extra-resource",
								},
							},
						},
					},
				},
			},
		},
		"MergeExtraResourcesToContext": {
			reason: "The Function should merge extra resources into context.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Input: resource.MustStructObject(
						&v1beta1.GoTemplate{
							Source: v1beta1.InlineSource,
							Inline: &v1beta1.TemplateSourceInline{Template: extraResource},
						}),
					Context: resource.MustStructJSON(`{
						"apiextensions.crossplane.io/extra-resources": {
							"existing-extra-resource": {
							    "items": [
									{
										"resource": {
											"apiVersion": "example.org/v1",
											"kind": "CoolExtraResource",
											"metadata": {
												"name": "existing-extra-resource"
											},
											"spec": {}
										}
									}
								]
							}
						}
					}`),
					RequiredResources: map[string]*fnv1.Resources{
						"cool-extra-resource": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1","kind":"CoolExtraResource","metadata":{"name":"cool-extra-resource"},"spec":{}}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Context: resource.MustStructJSON(
						`{
							"apiextensions.crossplane.io/extra-resources": {
								"existing-extra-resource": {
									"items": [
										{
											"resource": {
												"apiVersion": "example.org/v1",
												"kind": "CoolExtraResource",
												"metadata": {
													"name": "existing-extra-resource"
												},
												"spec": {}
											}
										}
									]
								},
								"cool-extra-resource": {
								    "items": [
									    {
											"resource": {
												"apiVersion": "example.org/v1",
												"kind": "CoolExtraResource",
												"metadata": {
													"name": "cool-extra-resource"
												},
												"spec": {}
											}
										}
									]
								}
							}
						}`,
					),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON("{}"),
						},
					},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"cool-extra-resource": {
								ApiVersion: "example.org/v1",
								Kind:       "CoolExtraResource",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "cool-extra-resource",
								},
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
