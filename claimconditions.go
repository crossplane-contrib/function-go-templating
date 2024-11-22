package main

import (
	xpruntimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/davecgh/go-spew/spew"
	corev1 "k8s.io/api/core/v1"
)

// A CompositionTarget is the target of a composition event or condition.
type CompositionTarget string

// Composition event and condition targets.
const (
	CompositionTargetComposite         CompositionTarget = "Composite"
	CompositionTargetCompositeAndClaim CompositionTarget = "CompositeAndClaim"
)

func UpdateClaimConditions(rsp *fnv1.RunFunctionResponse, conditions ...xpruntimev1.Condition) (*fnv1.RunFunctionResponse, error) {
	for _, c := range conditions {
		if xpruntimev1.IsSystemConditionType(c.Type) {
			response.Fatal(rsp, errors.Errorf("cannot set ClaimCondition type: %s is a reserved Crossplane Condition", c.Type))
			return rsp, nil
		}
		var co *response.ConditionOption
		switch c.Status {
		case corev1.ConditionTrue:
			co = response.ConditionTrue(rsp, string(c.Type), string(c.Reason)).WithMessage(c.Message)
		case corev1.ConditionFalse:
			co = response.ConditionFalse(rsp, string(c.Type), string(c.Reason))
		case corev1.ConditionUnknown:
			co = response.ConditionFalse(rsp, string(c.Type), string(c.Reason))
		}
		if c.Message != "" {
			co.WithMessage(c.Message)
		}

		spew.Dump(co)
	}
	return rsp, nil
}
