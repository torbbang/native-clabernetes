package topology

import (
	"fmt"

	clabernetesapis "github.com/srl-labs/clabernetes/apis"
	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconfig "github.com/srl-labs/clabernetes/config"
)

// GetTopologyKind returns the "kind" of topology this CR represents -- this will always be
// "containerlab" since KNE support has been removed.
func GetTopologyKind(t *clabernetesapisv1alpha1.Topology) string {
	return clabernetesapis.TopologyKindContainerlab
}

// ResolveGlobalVsTopologyBool accepts a pointer to bool value from the global config as well as
// from a topology spec, and returns a normal bool of the proper value. Meaning, if the topology
// value is unset, use the global value, but if the topology value is set always return that value.
func ResolveGlobalVsTopologyBool(globalValue bool, topologyValue *bool) bool {
	if topologyValue != nil {
		return *topologyValue
	}

	return globalValue
}

// ResolveTopologyRemovePrefix returns true if the topology resource should strip the containerlab
// topology prefix from a resource (deployment/service) name. This helper exists primarily for
// testing reasons as in the "normal" course of operation this value would always be taken from the
// status of a Topology object as this field as this will hold the resolved value at time of the
// creation of the Topology object. In the testing case the status field will be nil though, so in
// that case we'll go with the default "false" here.
func ResolveTopologyRemovePrefix(t *clabernetesapisv1alpha1.Topology) bool {
	if t.Status.RemoveTopologyPrefix == nil {
		return false
	}

	return *t.Status.RemoveTopologyPrefix
}

func resolveConnectivityDestination(
	topologyName,
	uninterestingEndpointNodeName,
	namespace string,
	removeTopologyPrefix bool,
	// inject config manager getter so we can easily test this (and things upstream)
	configManagerGetter clabernetesconfig.ManagerGetterFunc,
) string {
	destination := fmt.Sprintf(
		"%s-%s-vx.%s.%s",
		topologyName,
		uninterestingEndpointNodeName,
		namespace,
		configManagerGetter().GetInClusterDNSSuffix(),
	)

	if removeTopologyPrefix {
		destination = fmt.Sprintf(
			"%s-vx.%s.%s",
			uninterestingEndpointNodeName,
			namespace,
			configManagerGetter().GetInClusterDNSSuffix(),
		)
	}

	return destination
}
