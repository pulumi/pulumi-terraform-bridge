package terraform

import (
	"log"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/addrs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/configs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/dag"
)

// ConfigTransformer is a GraphTransformer that adds all the resources
// from the configuration to the graph.
//
// The module used to configure this transformer must be the root module.
//
// Only resources are added to the graph. Variables, outputs, and
// providers must be added via other transforms.
//
// Unlike ConfigTransformerOld, this transformer creates a graph with
// all resources including module resources, rather than creating module
// nodes that are then "flattened".
type ConfigTransformer struct {
	Concrete ConcreteResourceNodeFunc

	// Module is the module to add resources from.
	Config *configs.Config

	// Mode will only add resources that match the given mode
	ModeFilter bool
	Mode       addrs.ResourceMode

	// Do not apply this transformer.
	skip bool

	// configuration resources that are to be imported
	importTargets []*ImportTarget
}

func (t *ConfigTransformer) Transform(g *Graph) error {
	if t.skip {
		return nil
	}

	// If no configuration is available, we don't do anything
	if t.Config == nil {
		return nil
	}

	// Start the transformation process
	return t.transform(g, t.Config)
}

func (t *ConfigTransformer) transform(g *Graph, config *configs.Config) error {
	// If no config, do nothing
	if config == nil {
		return nil
	}

	// Add our resources
	if err := t.transformSingle(g, config); err != nil {
		return err
	}

	// Transform all the children.
	for _, c := range config.Children {
		if err := t.transform(g, c); err != nil {
			return err
		}
	}

	return nil
}

func (t *ConfigTransformer) transformSingle(g *Graph, config *configs.Config) error {
	path := config.Path
	module := config.Module
	log.Printf("[TRACE] ConfigTransformer: Starting for path: %v", path)

	allResources := make([]*configs.Resource, 0, len(module.ManagedResources)+len(module.DataResources))
	for _, r := range module.ManagedResources {
		allResources = append(allResources, r)
	}
	for _, r := range module.DataResources {
		allResources = append(allResources, r)
	}

	for _, r := range allResources {
		relAddr := r.Addr()

		if t.ModeFilter && relAddr.Mode != t.Mode {
			// Skip non-matching modes
			continue
		}

		// If any of the import targets can apply to this node's instances,
		// filter them down to the applicable addresses.
		var imports []*ImportTarget
		configAddr := relAddr.InModule(path)
		for _, i := range t.importTargets {
			if target := i.Addr.ContainingResource().Config(); target.Equal(configAddr) {
				imports = append(imports, i)
			}
		}

		abstract := &NodeAbstractResource{
			Addr: addrs.ConfigResource{
				Resource: relAddr,
				Module:   path,
			},
			importTargets: imports,
		}

		var node dag.Vertex = abstract
		if f := t.Concrete; f != nil {
			node = f(abstract)
		}

		g.Add(node)
	}

	return nil
}
