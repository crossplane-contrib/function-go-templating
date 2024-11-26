package main

import (
	"dario.cat/mergo"
	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

// MergeContext merges existing Context with new values provided
func (f *Function) MergeContext(req *fnv1.RunFunctionRequest, val map[string]interface{}) (map[string]interface{}, error) {
	mergedContext := req.GetContext().AsMap()
	if len(val) == 0 {
		return mergedContext, nil
	}
	if err := mergo.Merge(&mergedContext, val, mergo.WithOverride); err != nil {
		return mergedContext, errors.Wrapf(err, "cannot merge data %T", req)
	}
	return mergedContext, nil
}
