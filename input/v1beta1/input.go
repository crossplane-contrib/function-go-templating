// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=gotemplating.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// Input is used to provide templates to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Source specifies the different types of input sources that can be used with this function
	Source InputSource `json:"source"`
	// Inline is the inline form input of the templates
	Inline *string `json:"inline,omitempty"`
	// Path is the folder path where the templates are located
	Path *string `json:"path,omitempty"`
}

type InputSource string

const (
	// InputSourceInline indicates that function will get its input as inline
	InputSourceInline InputSource = "Inline"

	// InputSourceFile indicates that function will get its input from a folder
	InputSourceFile InputSource = "File"
)
