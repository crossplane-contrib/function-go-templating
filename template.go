package main

import (
	"os"
	"path/filepath"

	"github.com/crossplane/function-sdk-go/errors"

	"github.com/crossplane-contrib/function-go-templating/input/v1beta1"
)

const dotCharacter = 46

// TemplateGetter interface is used to read templates from different sources
type TemplateGetter interface {
	// GetTemplates returns the templates from the datasource
	GetTemplates() string
}

// NewTemplateSourceGetter returns a TemplateGetter based on the cd source
func NewTemplateSourceGetter(in *v1beta1.GoTemplate) (TemplateGetter, error) {
	switch in.Source {
	case v1beta1.InlineSource:
		return newInlineSource(in)
	case v1beta1.FileSystemSource:
		return newFileSource(in)
	case "":
		return nil, errors.Errorf("source is required")
	default:
		return nil, errors.Errorf("invalid source: %s", in.Source)
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

// GetTemplates returns the inline template
func (is *InlineSource) GetTemplates() string {
	return is.Template
}

func newInlineSource(in *v1beta1.GoTemplate) (*InlineSource, error) {
	return &InlineSource{
		Template: in.Inline.Template,
	}, nil
}

// GetTemplates returns the templates in the folder
func (fs *FileSource) GetTemplates() string {
	return fs.Template
}

func newFileSource(in *v1beta1.GoTemplate) (*FileSource, error) {
	d := in.FileSystem.DirPath

	tmpl, err := readTemplates(d)
	if err != nil {
		return nil, errors.Errorf("cannot read tmpl from the folder %s: %s", *in.FileSystem, err)
	}

	return &FileSource{
		FolderPath: in.FileSystem.DirPath,
		Template:   tmpl,
	}, nil
}

func readTemplates(dir string) (string, error) {
	tmpl := ""

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check for directory and hidden files/folders
		if info.IsDir() || info.Name()[0] == dotCharacter {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		tmpl += string(data)
		tmpl += "\n---\n"

		return nil
	}); err != nil {
		return "", err
	}

	return tmpl, nil
}
