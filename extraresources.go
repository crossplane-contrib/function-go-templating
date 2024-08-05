package main

import (
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
)

// ExtraResourcesRequirements defines the requirements for extra resources.
type ExtraResourcesRequirements map[string]ExtraResourcesRequirement

// ExtraResourcesRequirement defines a single requirement for extra resources.
// Needed to have camelCase keys instead of the snake_case keys as defined
// through json tags by fnv1beta1.ResourceSelector.
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
}

// ToResourceSelector converts the ExtraResourcesRequirement to a fnv1beta1.ResourceSelector.
func (e *ExtraResourcesRequirement) ToResourceSelector() *fnv1beta1.ResourceSelector {
	out := &fnv1beta1.ResourceSelector{
		ApiVersion: e.APIVersion,
		Kind:       e.Kind,
	}
	if e.MatchName == "" {
		out.Match = &fnv1beta1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1beta1.MatchLabels{Labels: e.MatchLabels},
		}
		return out
	}

	out.Match = &fnv1beta1.ResourceSelector_MatchName{
		MatchName: e.MatchName,
	}
	return out
}
