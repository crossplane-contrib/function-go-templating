package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

func TestToResourceSelector(t *testing.T) {
	ns := "test-namespace"
	name := "test-name"

	cases := map[string]struct {
		reason string
		req    ExtraResourcesRequirement
		want   *fnv1.ResourceSelector
	}{
		"MatchLabelsWithNamespace": {
			reason: "Namespace must be set on the selector when matching by labels, not just when matching by name",
			req: ExtraResourcesRequirement{
				APIVersion:  "gateway.networking.k8s.io/v1",
				Kind:        "HTTPRoute",
				Namespace:   ns,
				MatchLabels: map[string]string{"routetype": "prj-hostname"},
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "gateway.networking.k8s.io/v1",
				Kind:       "HTTPRoute",
				Namespace:  &ns,
				Match: &fnv1.ResourceSelector_MatchLabels{
					MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{"routetype": "prj-hostname"}},
				},
			},
		},
		"MatchLabelsWithoutNamespace": {
			reason: "Namespace must remain unset when matching by labels for a cluster-scoped resource",
			req: ExtraResourcesRequirement{
				APIVersion:  "gateway.networking.k8s.io/v1",
				Kind:        "HTTPRoute",
				MatchLabels: map[string]string{"routetype": "prj-hostname"},
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "gateway.networking.k8s.io/v1",
				Kind:       "HTTPRoute",
				Match: &fnv1.ResourceSelector_MatchLabels{
					MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{"routetype": "prj-hostname"}},
				},
			},
		},
		"MatchNameWithNamespace": {
			reason: "Namespace must still be set on the selector when matching by name",
			req: ExtraResourcesRequirement{
				APIVersion: "gateway.networking.k8s.io/v1",
				Kind:       "HTTPRoute",
				Namespace:  ns,
				MatchName:  name,
			},
			want: &fnv1.ResourceSelector{
				ApiVersion: "gateway.networking.k8s.io/v1",
				Kind:       "HTTPRoute",
				Namespace:  &ns,
				Match: &fnv1.ResourceSelector_MatchName{
					MatchName: name,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.req.ToResourceSelector()
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nToResourceSelector(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
