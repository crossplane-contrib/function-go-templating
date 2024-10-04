package main

import (
	"dario.cat/mergo"
	"github.com/crossplane/function-sdk-go/errors"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MergeContextKey merges existing Context at a key with context data val
func (f *Function) MergeContextKey(key string, val map[string]interface{}, req *fnv1beta1.RunFunctionRequest) (*unstructured.Unstructured, error) {
	// Check if key is already defined in the context and merge fields
	var mergedContext *unstructured.Unstructured
	if v, ok := request.GetContextKey(req, key); ok {
		mergedContext = &unstructured.Unstructured{}
		if err := resource.AsObject(v.GetStructValue(), mergedContext); err != nil {
			return mergedContext, errors.Wrapf(err, "cannot get Composition environment from %T context key %q", req, key)
		}
		f.log.Debug("Loaded Existing Function Context", "context-key", key)
		if err := mergo.Merge(&mergedContext.Object, val, mergo.WithOverride); err != nil {
			return mergedContext, errors.Wrapf(err, "cannot merge data %T at context key %q", req, key)
		}
		return mergedContext, nil
	}
	return &unstructured.Unstructured{Object: val}, nil
}
