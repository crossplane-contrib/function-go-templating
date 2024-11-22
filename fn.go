package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"io/fs"
	"os"

	"dario.cat/mergo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	xpruntimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
)

// osFS is a dead-simple implementation of [io/fs.FS] that just wraps around
// [os.Open].
type osFS struct{}

func (*osFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

// Function uses Go templates to compose resources.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log  logging.Logger
	fsys fs.FS
}

const (
	annotationKeyCompositionResourceName = "gotemplating.fn.crossplane.io/composition-resource-name"
	annotationKeyReady                   = "gotemplating.fn.crossplane.io/ready"

	metaApiVersion = "meta.gotemplating.fn.crossplane.io/v1alpha1"
)

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running Function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.GoTemplate{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	tg, err := NewTemplateSourceGetter(f.fsys, in)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "invalid function input"))
		return rsp, nil
	}

	f.log.Debug("template", "template", tg.GetTemplates())

	tmpl, err := GetNewTemplateWithFunctionMaps(in.Delims).Parse(tg.GetTemplates())
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "invalid function input: cannot parse the provided templates"))
		return rsp, nil
	}

	reqMap, err := convertToMap(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot convert request to map"))
		return rsp, nil
	}

	f.log.Debug("constructed request map", "request", reqMap)

	buf := &bytes.Buffer{}

	if err := tmpl.Execute(buf, reqMap); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot execute template"))
		return rsp, nil
	}

	f.log.Debug("rendered manifests", "manifests", buf.String())

	// Parse the rendered manifests.
	var objs []*unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(buf.String()), 1024)
	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if err == io.EOF {
				break
			}
			response.Fatal(rsp, errors.Wrap(err, "cannot decode manifest"))
			return rsp, nil
		}

		if u == nil {
			continue
		}

		// When decoding YAML into an Unstructured object, unquoted values like booleans or integers
		// can inadvertently be set as annotations, leading to unexpected behavior in later processing
		// steps that assume string-only values, such as GetAnnotations.
		if _, _, err := unstructured.NestedStringMap(u.Object, "metadata", "annotations"); err != nil {
			m, _, _ := unstructured.NestedMap(u.Object, "metadata", "annotations")
			response.Fatal(rsp, errors.Wrapf(err, "invalid annotations in resource '%s resource-name=%v'", u.GroupVersionKind(), m[annotationKeyCompositionResourceName]))
			return rsp, nil
		}

		objs = append(objs, u)
	}

	// Get the desired composite resource from the request.
	desiredComposite, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composite resource"))
		return rsp, nil
	}

	// Get the observed composite resource from the request.
	observedComposite, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}

	//  Get the desired composed resources from the request.
	desiredComposed, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composed resources"))
		return rsp, nil
	}

	// Initialize the requirements.
	requirements := &fnv1.Requirements{ExtraResources: make(map[string]*fnv1.ResourceSelector)}

	// Convert the rendered manifests to a list of desired composed resources.
	for _, obj := range objs {
		cd := resource.NewDesiredComposed()
		cd.Resource.Unstructured = *obj.DeepCopy()

		// TODO(ezgidemirel): Refactor to reduce cyclomatic complexity.
		// Handle if the composite resource appears in the rendered template.
		// Unless resource name annotation is present, update only the status of the desired composite resource.
		name, nameFound := obj.GetAnnotations()[annotationKeyCompositionResourceName]
		if cd.Resource.GetAPIVersion() == observedComposite.Resource.GetAPIVersion() && cd.Resource.GetKind() == observedComposite.Resource.GetKind() && !nameFound {
			dst := make(map[string]any)
			if err := desiredComposite.Resource.GetValueInto("status", &dst); err != nil && !fieldpath.IsNotFound(err) {
				response.Fatal(rsp, errors.Wrap(err, "cannot get desired composite status"))
				return rsp, nil
			}

			src := make(map[string]any)
			if err := cd.Resource.GetValueInto("status", &src); err != nil && !fieldpath.IsNotFound(err) {
				response.Fatal(rsp, errors.Wrap(err, "cannot get templated composite status"))
				return rsp, nil
			}

			if err := mergo.Merge(&dst, src, mergo.WithOverride); err != nil {
				response.Fatal(rsp, errors.Wrap(err, "cannot merge desired composite status"))
				return rsp, nil
			}

			if err := fieldpath.Pave(desiredComposite.Resource.Object).SetValue("status", dst); err != nil {
				response.Fatal(rsp, errors.Wrap(err, "cannot set desired composite status"))
				return rsp, nil
			}

			continue
		}

		// TODO(ezgidemirel): Refactor to reduce cyclomatic complexity.
		if cd.Resource.GetAPIVersion() == metaApiVersion {
			switch obj.GetKind() {
			case "CompositeConnectionDetails":
				// Set composite resource's connection details.
				con, _ := cd.Resource.GetStringObject("data")
				for k, v := range con {
					d, _ := base64.StdEncoding.DecodeString(v) //nolint:errcheck // k8s returns secret values encoded
					desiredComposite.ConnectionDetails[k] = d
				}
			case "ClaimConditions":
				var conditions []xpruntimev1.Condition
				if err = cd.Resource.GetValueInto("conditions", &conditions); err != nil {
					response.Fatal(rsp, errors.Wrap(err, "cannot get Conditions from input"))
					return rsp, nil
				}
				rsp, err := UpdateClaimConditions(rsp, conditions...)
				if err != nil {
					response.Fatal(rsp, errors.Wrap(err, "cannot set ClaimCondition"))
					return rsp, nil
				}
			case "Context":
				contextData := make(map[string]interface{})
				if err = cd.Resource.GetValueInto("data", &contextData); err != nil {
					response.Fatal(rsp, errors.Wrap(err, "cannot get Contexts from input"))
					return rsp, nil
				}
				mergedCtx, err := f.MergeContext(req, contextData)
				if err != nil {
					response.Fatal(rsp, errors.Wrapf(err, "cannot merge Context"))
					return rsp, nil
				}

				for key, v := range mergedCtx {
					vv, err := structpb.NewValue(v)
					if err != nil {
						response.Fatal(rsp, errors.Wrap(err, "cannot convert value to structpb.Value"))
						return rsp, nil
					}
					f.log.Debug("Updating Composition environment", "key", key, "data", v)
					response.SetContextKey(rsp, key, vv)
				}
			case "ExtraResources":
				// Set extra resources requirements.
				ers := make(ExtraResourcesRequirements)
				if err = cd.Resource.GetValueInto("requirements", &ers); err != nil {
					response.Fatal(rsp, errors.Wrap(err, "cannot get extra resources requirements"))
					return rsp, nil
				}
				for k, v := range ers {
					if _, found := requirements.ExtraResources[k]; found {
						response.Fatal(rsp, errors.Errorf("duplicate extra resource key %q", k))
						return rsp, nil
					}
					requirements.ExtraResources[k] = v.ToResourceSelector()
				}
			default:
				response.Fatal(rsp, errors.Errorf("invalid kind %q for apiVersion %q - must be one of CompositeConnectionDetails, Context or ExtraResources", obj.GetKind(), metaApiVersion))
				return rsp, nil
			}

			continue
		}

		// TODO(ezgidemirel): Refactor to reduce cyclomatic complexity.
		// Set ready state.
		if v, found := cd.Resource.GetAnnotations()[annotationKeyReady]; found {
			if v != string(resource.ReadyTrue) && v != string(resource.ReadyUnspecified) && v != string(resource.ReadyFalse) {
				response.Fatal(rsp, errors.Errorf("invalid function input: invalid %q annotation value %q: must be True, False, or Unspecified", annotationKeyReady, v))
				return rsp, nil
			}

			cd.Ready = resource.Ready(v)

			// Remove meta annotation.
			meta.RemoveAnnotations(cd.Resource, annotationKeyReady)
		}

		// Remove resource name annotation.
		meta.RemoveAnnotations(cd.Resource, annotationKeyCompositionResourceName)

		// Add resource to the desired composed resources map.
		if !nameFound {
			response.Fatal(rsp, errors.Errorf("%q template is missing required %q annotation", obj.GetKind(), annotationKeyCompositionResourceName))
			return rsp, nil
		}

		desiredComposed[resource.Name(name)] = cd
	}

	f.log.Debug("desired composite resource", "desiredComposite:", desiredComposite)
	f.log.Debug("constructed desired composed resources", "desiredComposed:", desiredComposed)

	if err := response.SetDesiredComposedResources(rsp, desiredComposed); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot desired composed resources"))
		return rsp, nil
	}

	if err := response.SetDesiredCompositeResource(rsp, desiredComposite); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot set desired composite resource"))
		return rsp, nil
	}

	if len(requirements.ExtraResources) > 0 {
		rsp.Requirements = requirements
	}

	f.log.Info("Successfully composed desired resources", "source", in.Source, "count", len(objs))

	return rsp, nil
}

func convertToMap(req *fnv1.RunFunctionRequest) (map[string]any, error) {
	jReq, err := protojson.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal request from proto to json")
	}

	var mReq map[string]any
	if err := json.Unmarshal(jReq, &mReq); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal json to map[string]any")
	}

	return mReq, nil
}
