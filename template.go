package main

import (
	"io/fs"
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
func NewTemplateSourceGetter(fsys fs.FS, in *v1beta1.GoTemplate) (TemplateGetter, error) {
	switch in.Source {
	case v1beta1.InlineSource:
		return newInlineSource(in)
	case v1beta1.FileSystemSource:
		return newFileSource(fsys, in)
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
	if in.Inline == nil || in.Inline.Template == "" {
		return nil, errors.New("inline.template should be provided")
	}

	return &InlineSource{
		Template: in.Inline.Template,
	}, nil
}

// GetTemplates returns the templates in the folder
func (fs *FileSource) GetTemplates() string {
	return fs.Template
}

func newFileSource(fsys fs.FS, in *v1beta1.GoTemplate) (*FileSource, error) {
	if in.FileSystem == nil || in.FileSystem.DirPath == "" {
		return nil, errors.New("fileSystem.dirPath should be provided")
	}

	d := in.FileSystem.DirPath

	tmpl, err := readTemplates(fsys, d)
	if err != nil {
		return nil, errors.Errorf("cannot read tmpl from the folder %s: %s", *in.FileSystem, err)
	}

	return &FileSource{
		FolderPath: in.FileSystem.DirPath,
		Template:   tmpl,
	}, nil
}

func readTemplates(fsys fs.FS, dir string) (string, error) {
	tmpl := ""

	if err := fs.WalkDir(fsys, dir, func(path string, dirEntry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}

		// skip hidden directories
		if dirEntry.IsDir() && dirEntry.Name()[0] == dotCharacter {
			return filepath.SkipDir
		}

		info, err := dirEntry.Info()
		if err != nil {
			return err
		}

		// check for directory and hidden files/folders
		if info.IsDir() || info.Name()[0] == dotCharacter {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
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
