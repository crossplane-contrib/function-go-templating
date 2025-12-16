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
	metav1.TypeMeta `json:",inline"`

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
	// Options to set for the template engine. Valid options are documented at https://pkg.go.dev/text/template#Template.Option
	Options *[]string `json:"options,omitempty"`
}

// TemplateSource defines the location of the source template.
type TemplateSource string

const (
	// InlineSource indicates that function will get its input as inline.
	InlineSource TemplateSource = "Inline"

	// FileSystemSource indicates that function will get its input from a folder.
	FileSystemSource TemplateSource = "FileSystem"

	// EnvironmentSource indicates that function will get its input from the environment.
	EnvironmentSource TemplateSource = "Environment"
)

// TemplateSourceInline defines the structure of the inline source. Allows specifying either a single inline template or multiple templates, but not both.
// +kubebuilder:validation:XValidation:rule="(has(self.template) ? 1 : 0) + (has(self.templates) ? 1 : 0) == 1",message="Exactly one of 'template' or 'templates' must be set"
type TemplateSourceInline struct {
	Template  string   `json:"template,omitempty"`
	Templates []string `json:"templates,omitempty"`
}

// TemplateSourceFileSystem defines the structure of the filesystem source.
type TemplateSourceFileSystem struct {
	DirPath string `json:"dirPath,omitempty"`
}

// TemplateSourceEnvironment defines the structure of the environment source.
type TemplateSourceEnvironment struct {
	Key string `json:"key,omitempty"`
}

// Delims defines the structure for customizing template delimiters.
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
