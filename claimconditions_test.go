package main

import (
	"reflect"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func Test_UpdateClaimConditions(t *testing.T) {
	type args struct {
		rsp *fnv1.RunFunctionResponse
		c   []TargetedCondition
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
		"EmptyResponseNoConditions": {
			reason: "When No Response or Conditions are provided, return a nil response",
			args:   args{},
			want:   want{},
		},
		"ErrorOnReadyReservedType": {
			reason: "Return an error if a Reserved Condition Type is being used",
			args: args{
				rsp: &fnv1.RunFunctionResponse{},
				c: []TargetedCondition{
					{
						Condition: xpv1.Condition{
							Message: "Ready Message",
							Status:  v1.ConditionTrue,
							Type:    "Ready",
						},
						Target: CompositionTargetComposite,
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot set ClaimCondition type: Ready is a reserved Crossplane Condition",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
				err: errors.New("error updating response"),
			},
		},
		"SuccessfullyAddConditions": {
			reason: "Add Conditions Successfully",
			args: args{
				rsp: &fnv1.RunFunctionResponse{},
				c: []TargetedCondition{
					{
						Condition: xpv1.Condition{
							Message: "Creating Resource",
							Status:  v1.ConditionFalse,
							Type:    "NetworkReady",
						},
						Target: CompositionTargetCompositeAndClaim,
					},
					{
						Condition: xpv1.Condition{
							Message: "Ready Message",
							Status:  v1.ConditionTrue,
							Type:    "DatabaseReady",
						},
						Target: CompositionTargetComposite,
					},
					{
						Condition: xpv1.Condition{
							Message: "No Target should add CompositeAndClaim",
							Status:  v1.ConditionTrue,
							Type:    "NoTarget",
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Conditions: []*fnv1.Condition{
						{
							Message: ptr.To("Creating Resource"),
							Status:  fnv1.Status_STATUS_CONDITION_FALSE,
							Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							Type:    "NetworkReady",
						},
						{
							Message: ptr.To("Ready Message"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
							Type:    "DatabaseReady",
						},
						{
							Message: ptr.To("No Target should add CompositeAndClaim"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
							Type:    "NoTarget",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := UpdateClaimConditions(tc.args.rsp, tc.args.c...)
			if diff := cmp.Diff(tc.args.rsp, tc.want.rsp, cmpopts.IgnoreUnexported(fnv1.RunFunctionResponse{}, fnv1.Result{}, fnv1.Condition{})); diff != "" {
				t.Errorf("%s\nUpdateClaimConditions(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nUpdateClaimConditions(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

func Test_transformCondition(t *testing.T) {
	type args struct {
		tc TargetedCondition
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *fnv1.Condition
	}{
		"Basic": {
			reason: "Basic Target",
			args: args{
				tc: TargetedCondition{
					Condition: xpv1.Condition{
						Message: "Basic Message",
						Status:  v1.ConditionTrue,
						Type:    "TestType",
					},
					Target: CompositionTargetComposite,
				},
			},
			want: &fnv1.Condition{
				Message: ptr.To("Basic Message"),
				Status:  fnv1.Status_STATUS_CONDITION_TRUE,
				Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
				Type:    "TestType",
			},
		},
		"Defaults": {
			reason: "Default Settings",
			args: args{
				tc: TargetedCondition{
					Condition: xpv1.Condition{},
				},
			},
			want: &fnv1.Condition{
				Status: fnv1.Status_STATUS_CONDITION_UNKNOWN,
				Target: fnv1.Target_TARGET_COMPOSITE.Enum(),
			},
		},
		"StatusFalseNoTarget": {
			reason: "When Status is false and no target set",
			args: args{
				tc: TargetedCondition{
					Condition: xpv1.Condition{
						Message: "Basic Message",
						Status:  v1.ConditionFalse,
						Type:    "TestType",
					},
				},
			},
			want: &fnv1.Condition{
				Message: ptr.To("Basic Message"),
				Status:  fnv1.Status_STATUS_CONDITION_FALSE,
				Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
				Type:    "TestType",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := transformCondition(tc.args.tc); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("transformCondition() = %v, want %v", got, tc.want)
			}
		})
	}
}

func Test_transformTarget(t *testing.T) {
	type args struct {
		ct CompositionTarget
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *fnv1.Target
	}{
		"DefaultToComposite": {
			reason: "unknown target will default to Composite",
			args: args{
				ct: "COMPOSE",
			},
			want: fnv1.Target_TARGET_COMPOSITE.Enum(),
		},
		"Composite": {
			reason: "Composite target correctly set",
			args: args{
				ct: "Composite",
			},
			want: fnv1.Target_TARGET_COMPOSITE.Enum(),
		},
		"CompositeAndClaim": {
			reason: "CompositeAndClaim target correctly set",
			args: args{
				ct: "CompositeAndClaim",
			},
			want: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := transformTarget(tc.args.ct); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("transformTarget() = %v, want %v", got, tc.want)
			}
		})
	}
}

func Test_UpdateResponseWithCondition(t *testing.T) {
	type args struct {
		rsp *fnv1.RunFunctionResponse
		c   *fnv1.Condition
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *fnv1.RunFunctionResponse
	}{
		"EmptyResponseNoConditions": {
			reason: "When No Response or Conditions are provided, return a nil response",
			args:   args{},
		},
		"ResponseWithNoConditions": {
			reason: "A response with no conditions should initialize an array before adding the condition",
			args: args{
				rsp: &fnv1.RunFunctionResponse{},
			},
			want: &fnv1.RunFunctionResponse{
				Conditions: []*fnv1.Condition{},
			},
		},
		"ResponseAddCondition": {
			reason: "A response with no conditions should initialize an array before adding the condition",
			args: args{
				rsp: &fnv1.RunFunctionResponse{},
				c: &fnv1.Condition{
					Message: ptr.To("Basic Message"),
					Status:  fnv1.Status_STATUS_CONDITION_FALSE,
					Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
					Type:    "TestType",
				},
			},
			want: &fnv1.RunFunctionResponse{
				Conditions: []*fnv1.Condition{
					{
						Message: ptr.To("Basic Message"),
						Status:  fnv1.Status_STATUS_CONDITION_FALSE,
						Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						Type:    "TestType",
					},
				},
			},
		},
		"ResponseAppCondition": {
			reason: "A response with existing conditions should append the condition",
			args: args{
				rsp: &fnv1.RunFunctionResponse{
					Conditions: []*fnv1.Condition{
						{
							Message: ptr.To("Existing Message"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
							Type:    "ExistingTestType",
						},
					},
				},
				c: &fnv1.Condition{
					Message: ptr.To("Basic Message"),
					Status:  fnv1.Status_STATUS_CONDITION_FALSE,
					Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
					Type:    "TestType",
				},
			},
			want: &fnv1.RunFunctionResponse{
				Conditions: []*fnv1.Condition{
					{
						Message: ptr.To("Existing Message"),
						Status:  fnv1.Status_STATUS_CONDITION_TRUE,
						Target:  fnv1.Target_TARGET_COMPOSITE.Enum(),
						Type:    "ExistingTestType",
					},
					{
						Message: ptr.To("Basic Message"),
						Status:  fnv1.Status_STATUS_CONDITION_FALSE,
						Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						Type:    "TestType",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			UpdateResponseWithCondition(tc.args.rsp, tc.args.c)
			if diff := cmp.Diff(tc.args.rsp, tc.want, cmpopts.IgnoreUnexported(fnv1.RunFunctionResponse{}, fnv1.Condition{})); diff != "" {
				t.Errorf("%s\nUpdateResponseWithCondition(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
		})
	}
}
