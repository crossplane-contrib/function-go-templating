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

// A GoTemplate is used to provide templates to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type GoTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Template delimiters
	// +optional
	Delims *Delims `json:"delims,omitempty"`
	// Source specifies the different types of input sources that can be used with this function
	Source TemplateSource `json:"source"`
	// Inline is the inline form input of the templates
	Inline *TemplateSourceInline `json:"inline,omitempty"`
	// FileSystem is the folder path where the templates are located
	FileSystem *TemplateSourceFileSystem `json:"fileSystem,omitempty"`
	// Environment is the key that defines the location of the templates in the environment
	Environment *TemplateSourceEnvironment `json:"environment,omitempty"`
}

type TemplateSource string

const (
	// InlineSource indicates that function will get its input as inline
	InlineSource TemplateSource = "Inline"

	// FileSystemSource indicates that function will get its input from a folder
	FileSystemSource TemplateSource = "FileSystem"

	// EnvironmentSource indicates that function will get its input from the environment
	EnvironmentSource TemplateSource = "Environment"
)

type TemplateSourceInline struct {
	Template string `json:"template,omitempty"`
}

type TemplateSourceFileSystem struct {
	DirPath string `json:"dirPath,omitempty"`
}

type TemplateSourceEnvironment struct {
	Key string `json:"key,omitempty"`
}

type Delims struct {
	// Template start characters
	// +kubebuilder:default:="{{"
	// +optional
	Left *string `json:"left,omitempty"`
	// Template end characters
	// +kubebuilder:default:="}}"
	// +optional
	Right *string `json:"right,omitempty"`
}
