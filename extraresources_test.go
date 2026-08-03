package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

func TestExtraResourcesRequirementToResourceSelector(t *testing.T) {
	ns := "test"

	cases := map[string]struct {
		reason string
		e      ExtraResourcesRequirement
		want   *fnv1.ResourceSelector
	}{
		"MatchLabelsWithNamespace": {
			reason: "Namespace must be set on the selector when MatchLabels is used, not just MatchName.",
			e: ExtraResourcesRequirement{
				APIVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				MatchLabels: map[string]string{
					"cool": "true",
				},
				Namespace: ns,
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				Match: &fnv1.ResourceSelector_MatchLabels{
					MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{
						"cool": "true",
					}},
				},
				Namespace: &ns,
			},
		},
		"MatchLabelsWithoutNamespace": {
			reason: "Namespace must be left unset when it is empty.",
			e: ExtraResourcesRequirement{
				APIVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				MatchLabels: map[string]string{
					"cool": "true",
				},
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				Match: &fnv1.ResourceSelector_MatchLabels{
					MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{
						"cool": "true",
					}},
				},
			},
		},
		"MatchNameWithNamespace": {
			reason: "Namespace must still be set when MatchName is used.",
			e: ExtraResourcesRequirement{
				APIVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				MatchName:  "cool-extra-resource",
				Namespace:  ns,
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "CoolExtraResource",
				Match: &fnv1.ResourceSelector_MatchName{
					MatchName: "cool-extra-resource",
				},
				Namespace: &ns,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.e.ToResourceSelector()
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nToResourceSelector(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
