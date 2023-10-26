package main

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane/function-go-templating/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

const (
	invalidFunctionFmt = "invalid function input: %s"
	noTempError        = "templates are required either inline or from a path"
	cannotGet          = "cannot get the function input"
	cannotParse        = "cannot parse the provided templates"
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

	if in.Inline == nil && in.Path == nil {
		response.Fatal(rsp, errors.New(fmt.Sprintf(invalidFunctionFmt, noTempError)))
		return rsp, nil
	}

	tg := NewTemplateGetter(in)
	input, err := tg.GetTemplate()
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, fmt.Sprintf(invalidFunctionFmt, cannotGet)))
		return rsp, nil
	}

	tmpl, err := GetNewTemplateWithFunctionMaps().Parse(input)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, fmt.Sprintf(invalidFunctionFmt, cannotParse)))
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
	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composite resource"))
		return rsp, nil
	}

	// Convert the rendered manifests to a list of desired composed resources.
	dcd, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composed resources"))
		return rsp, nil
	}

	for _, obj := range objs {
		cd := resource.NewDesiredComposed()
		cd.Resource.Unstructured = *obj.DeepCopy()

		// do not add the composite resource to composed resources
		if cd.Resource.GetAPIVersion() == dxr.Resource.GetAPIVersion() && cd.Resource.GetKind() == dxr.Resource.GetKind() {
			continue
		}

		dcd[resource.Name(obj.GetName())] = cd
	}

	f.log.Debug("desired composite resource", "desiredComposite:", dxr)
	f.log.Debug("constructed desired composed resources", "desiredComposed:", dcd)

	if err := response.SetDesiredComposedResources(rsp, dcd); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot desired dsd composed resources"))
		return rsp, nil
	}

	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot set desired composite resource"))
		return rsp, nil
	}

	response.Normalf(rsp, "I was run with input source %q", in.Source)

	return rsp, nil
}

func convertToMap(req *fnv1beta1.RunFunctionRequest) (map[string]interface{}, error) {
	jReq, err := protojson.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal request from proto to json")
	}

	var mReq map[string]interface{}
	if err := json.Unmarshal(jReq, &mReq); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal json to map[string]interface{}")
	}

	return mReq, nil
}
