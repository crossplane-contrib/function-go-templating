package main

import (
	"bytes"
	"context"
	"dario.cat/mergo"
	"encoding/base64"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	fn "github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

const (
	errFmtInvalidFunction   = "invalid function input: %s"
	errFmtInvalidReadyValue = "%s is invalid, ready annotation must be True, Unspecified, or False"
	errFmtInvalidMetaType   = "invalid meta kind %s"

	errCannotGet   = "cannot get the function input"
	errCannotParse = "cannot parse the provided templates"
)

const (
	annotationKeyCompositionResourceName = "gotemplating.fn.crossplane.io/composition-resource-name"
	annotationKeyReady                   = "gotemplating.fn.crossplane.io/ready"

	metaApiVersion = "meta.gotemplating.fn.crossplane.io/v1alpha1"
)

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running Function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	tg, err := NewTemplateSourceGetter(in)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, fmt.Sprintf(errFmtInvalidFunction, errCannotGet)))
		return rsp, nil
	}

	tmpl, err := GetNewTemplateWithFunctionMaps().Parse(tg.GetTemplates())
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, fmt.Sprintf(errFmtInvalidFunction, errCannotParse)))
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
		if u != nil {
			objs = append(objs, u)
		}
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

	// Convert the rendered manifests to a list of desired composed resources.
	for _, obj := range objs {
		cd := resource.NewDesiredComposed()
		cd.Resource.Unstructured = *obj.DeepCopy()

		// TODO(ezgidemirel): Refactor to reduce cyclomatic complexity.
		// Update only the status of the desired composite resource.
		if cd.Resource.GetAPIVersion() == observedComposite.Resource.GetAPIVersion() && cd.Resource.GetKind() == observedComposite.Resource.GetKind() {
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
		// Set composite resource's connection details.
		if cd.Resource.GetAPIVersion() == metaApiVersion {
			switch obj.GetKind() {
			case "CompositeConnectionDetails":
				con, _ := cd.Resource.GetStringObject("data")
				for k, v := range con {
					d, _ := base64.StdEncoding.DecodeString(v) //nolint:errcheck // k8s returns secret values encoded
					desiredComposite.ConnectionDetails[k] = d
				}
			default:
				response.Fatal(rsp, fmt.Errorf(errFmtInvalidMetaType, obj.GetKind()))
				return rsp, nil
			}

			continue
		}

		// TODO(ezgidemirel): Refactor to reduce cyclomatic complexity.
		// Set ready state.
		if v, found := cd.Resource.GetAnnotations()[annotationKeyReady]; found {
			if v != string(resource.ReadyTrue) && v != string(resource.ReadyUnspecified) && v != string(resource.ReadyFalse) {
				response.Fatal(rsp, fmt.Errorf(fmt.Sprintf(errFmtInvalidFunction, errFmtInvalidReadyValue), v))
				return rsp, nil
			}

			cd.Ready = fn.Ready(v)

			// Remove meta annotation.
			meta.RemoveAnnotations(cd.Resource, annotationKeyReady)
		}

		// Remove resource name annotation.
		meta.RemoveAnnotations(cd.Resource, annotationKeyCompositionResourceName)

		// Add resource to the desired composed resources map.
		name, found := obj.GetAnnotations()[annotationKeyCompositionResourceName]
		if !found {
			response.Fatal(rsp, errors.Errorf("cannot get composition resource name of %s", obj.GetName()))
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

	response.Normalf(rsp, "Successful run with %q source", in.Source)

	return rsp, nil
}

func convertToMap(req *fnv1beta1.RunFunctionRequest) (map[string]any, error) {
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
