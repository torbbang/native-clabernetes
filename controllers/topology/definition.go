package topology

import (
	"fmt"

	clabernetesapis "github.com/srl-labs/clabernetes/apis"
	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconfig "github.com/srl-labs/clabernetes/config"
	claberneteserrors "github.com/srl-labs/clabernetes/errors"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
)

// DefinitionProcessor is an interface defining a definition processor -- that is, an object that
// accepts a clabernetes topology to update based on the included (probably containerlab, but maybe
// kne or others in the future) configuration.
type DefinitionProcessor interface {
	// Process processes the topology, updating the given reconcile data object as necessary.
	Process() error
}

// NewDefinitionProcessor returns a definition processor for the given Topology.
func NewDefinitionProcessor(
	logger claberneteslogging.Instance,
	topology *clabernetesapisv1alpha1.Topology,
	reconcileData *ReconcileData,
	configManagerGetter clabernetesconfig.ManagerGetterFunc,
) (DefinitionProcessor, error) {
	if topology.Spec.Definition.Containerlab != "" {
		reconcileData.Kind = clabernetesapis.TopologyKindContainerlab

		return &containerlabDefinitionProcessor{
			&definitionProcessor{
				logger:              logger,
				topology:            topology,
				reconcileData:       reconcileData,
				configManagerGetter: configManagerGetter,
			},
		}, nil
	}

	return nil, fmt.Errorf(
		"%w: no containerlab definition provided",
		claberneteserrors.ErrReconcile,
	)
}

type definitionProcessor struct {
	logger              claberneteslogging.Instance
	topology            *clabernetesapisv1alpha1.Topology
	reconcileData       *ReconcileData
	configManagerGetter clabernetesconfig.ManagerGetterFunc
}

func (p *definitionProcessor) getRemoveTopologyPrefix() bool {
	var removeTopologyPrefix bool
	if ResolveTopologyRemovePrefix(p.topology) {
		removeTopologyPrefix = true
	}

	return removeTopologyPrefix
}

func (c *Controller) processDefinition(
	topology *clabernetesapisv1alpha1.Topology,
	reconcileData *ReconcileData,
) error {
	processor, err := NewDefinitionProcessor(
		c.Log,
		topology,
		reconcileData,
		clabernetesconfig.GetManager,
	)
	if err != nil {
		c.Log.Criticalf("failed creating definition processor, err: %s", err)

		return err
	}

	return processor.Process()
}
