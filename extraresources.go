package main

import (
	"encoding/json"
	"maps"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// ExtraResourcesRequirements defines the requirements for extra resources.
type ExtraResourcesRequirements map[string]ExtraResourcesRequirement

// ExtraResourcesRequirement defines a single requirement for extra resources.
// Needed to have camelCase keys instead of the snake_case keys as defined
// through json tags by fnv1.ResourceSelector.
type ExtraResourcesRequirement struct {
	// APIVersion of the resource.
	APIVersion string `json:"apiVersion"`
	// Kind of the resource.
	Kind string `json:"kind"`
	// MatchLabels defines the labels to match the resource, if defined,
	// matchName is ignored.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// MatchName defines the name to match the resource, if MatchLabels is
	// empty.
	MatchName string `json:"matchName,omitempty"`
	// Namespace defines the namespace of the resource to match, leave empty for cluster-scoped.
	Namespace string `json:"namespace,omitempty"`
}

const (
	extraResourcesContextKey = "apiextensions.crossplane.io/extra-resources"
)

// ToResourceSelector converts the ExtraResourcesRequirement to a fnv1.ResourceSelector.
func (e *ExtraResourcesRequirement) ToResourceSelector() *fnv1.ResourceSelector {
	out := &fnv1.ResourceSelector{
		ApiVersion: e.APIVersion,
		Kind:       e.Kind,
	}
	if e.MatchName == "" {
		out.Match = &fnv1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1.MatchLabels{Labels: e.MatchLabels},
		}
		return out
	}

	out.Match = &fnv1.ResourceSelector_MatchName{
		MatchName: e.MatchName,
	}

	if e.Namespace != "" {
		*out.Namespace = e.Namespace
	}
	return out
}

func mergeExtraResourcesToContext(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error {
	b, err := json.Marshal(req.ExtraResources) //nolint:staticcheck
	if err != nil {
		return errors.Errorf("cannot marshal %T: %w", req.ExtraResources, err) //nolint:staticcheck
	}

	s := &structpb.Struct{}
	if err := protojson.Unmarshal(b, s); err != nil {
		return errors.Errorf("cannot unmarshal %T into %T: %w", req.ExtraResources, s, err) //nolint:staticcheck
	}

	extraResourcesFromContext, exists := request.GetContextKey(req, extraResourcesContextKey)
	if exists {
		merged := mergeStructs(extraResourcesFromContext.GetStructValue(), s)
		s = merged
	}

	response.SetContextKey(rsp, extraResourcesContextKey, structpb.NewStructValue(s))
	return nil
}

func mergeRequiredResourcesToContext(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error {
	b, err := json.Marshal(req.RequiredResources)
	if err != nil {
		return errors.Errorf("cannot marshal %T: %w", req.RequiredResources, err)
	}

	s := &structpb.Struct{}
	if err := protojson.Unmarshal(b, s); err != nil {
		return errors.Errorf("cannot unmarshal %T into %T: %w", req.RequiredResources, s, err)
	}

	extraResourcesFromContext, exists := request.GetContextKey(req, extraResourcesContextKey)
	if exists {
		merged := mergeStructs(extraResourcesFromContext.GetStructValue(), s)
		s = merged
	}

	response.SetContextKey(rsp, extraResourcesContextKey, structpb.NewStructValue(s))
	return nil
}

// MergeStructs merges fields from s2 into s1, overwriting s1's fields if keys overlap.
func mergeStructs(s1, s2 *structpb.Struct) *structpb.Struct {
	if s1 == nil {
		return s2
	}
	if s2 == nil {
		return s1
	}
	merged := s1.AsMap()
	maps.Copy(merged, s2.AsMap())
	result, _ := structpb.NewStruct(merged)
	return result
}
