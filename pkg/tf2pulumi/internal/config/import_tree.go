package config

import (
	"fmt"
	"io"

	"github.com/hashicorp/errwrap"
	"github.com/spf13/afero"
)

// configurable is an interface that must be implemented by any configuration
// formats of Terraform in order to return a *Config.
type configurable interface {
	Config() (*Config, error)
}

// importTree is the result of the first-pass load of the configuration
// files. It is a tree of raw configurables and then any children (their
// imports).
//
// An importTree can be turned into a configTree.
type importTree struct {
	Path     string
	Raw      configurable
	Children []*importTree
}

// This is the function type that must be implemented by the configuration
// file loader to turn a single file into a configurable and any additional
// imports.
type fileLoaderFunc func(fs afero.Fs, path string) (configurable, []string, error)

// Set this to a non-empty value at link time to enable the HCL2 experiment.
// This is not currently enabled for release builds.
//
// For example:
//
//	go install -ldflags="-X github.com/hashicorp/terraform/config.enableHCL2Experiment=true" github.com/hashicorp/terraform
var enableHCL2Experiment = ""

// loadTreeFromString takes a string and treats it as an HCL file with the
// given path and loads the entire importTree for that file.
func loadTreeFromString(path, contents string) (*importTree, error) {
	return loadTreeFromFunc(nil, path, func(_ afero.Fs, path string) (configurable, []string, error) {
		return loadStringHcl(path, contents)
	})
}

// loadTree takes a single file and loads the entire importTree for that
// file. This function detects what kind of configuration file it is an
// executes the proper fileLoaderFunc.
func loadTree(fs afero.Fs, root string) (*importTree, error) {
	switch ext(root) {
	case ".tf", ".tf.json":
		return loadTreeFromFunc(fs, root, loadFileHcl)
	default:
		return nil, fmt.Errorf(
			"%s: unknown configuration format. Use '.tf' or '.tf.json' extension",
			root)
	}
}

// loadTree takes a fileLoaderFunc and a path and loads the entire importTree
// for that path using the func.
func loadTreeFromFunc(fs afero.Fs, root string, f fileLoaderFunc) (*importTree, error) {
	c, imps, err := f(fs, root)
	if err != nil {
		return nil, err
	}

	children := make([]*importTree, len(imps))
	for i, imp := range imps {
		t, err := loadTree(fs, imp)
		if err != nil {
			return nil, err
		}

		children[i] = t
	}

	return &importTree{
		Path:     root,
		Raw:      c,
		Children: children,
	}, nil
}

// Close releases any resources we might be holding open for the importTree.
//
// This can safely be called even while ConfigTree results are alive. The
// importTree is not bound to these.
func (t *importTree) Close() error {
	if c, ok := t.Raw.(io.Closer); ok {
		c.Close()
	}
	for _, ct := range t.Children {
		ct.Close()
	}

	return nil
}

// ConfigTree traverses the importTree and turns each node into a *Config
// object, ultimately returning a *configTree.
func (t *importTree) ConfigTree() (*configTree, error) {
	config, err := t.Raw.Config()
	if err != nil {
		return nil, errwrap.Wrapf(fmt.Sprintf("Error loading %s: {{err}}", t.Path), err)
	}

	// Build our result
	result := &configTree{
		Path:   t.Path,
		Config: config,
	}

	// Build the config trees for the children
	result.Children = make([]*configTree, len(t.Children))
	for i, ct := range t.Children {
		t, err := ct.ConfigTree()
		if err != nil {
			return nil, err
		}

		result.Children[i] = t
	}

	return result, nil
}
