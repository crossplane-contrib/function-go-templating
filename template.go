package main

import "github.com/crossplane/function-go-templating/input/v1beta1"

// TemplateGetter interface is used to read templates from different sources
type TemplateGetter interface {
	// GetTemplate returns the templates from the datasource
	GetTemplate() (string, error)
}

func NewTemplateGetter(in *v1beta1.Input) TemplateGetter {
	switch in.Source {
	case v1beta1.InputSourceInline:
		return newInlineSource(in)
	case v1beta1.InputSourceFile:
		return newFileSource(in)
	default:
		return nil
	}
}

// InlineSource is a datasource that reads a template from the composition
type InlineSource struct {
	Template string
}

// FileSource is a datasource that reads a template from a folder
type FileSource struct {
	FolderPath string
	Template   string
}

// GetTemplate returns the inline template
func (is *InlineSource) GetTemplate() (string, error) {
	return is.Template, nil
}

func newInlineSource(in *v1beta1.Input) *InlineSource {
	return &InlineSource{
		Template: *in.Inline,
	}
}

// GetTemplate returns the templates in the folder
func (fs *FileSource) GetTemplate() (string, error) {
	return fs.Template, nil
}

func newFileSource(in *v1beta1.Input) *FileSource {
	// TODO(ezgidemirel): read templates from the folder
	tmpl := ""
	return &FileSource{
		FolderPath: *in.Path,
		Template:   tmpl,
	}
}
