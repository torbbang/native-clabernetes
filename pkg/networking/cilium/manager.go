package cilium

import (
	"context"
	"fmt"
	"strings"

	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	k8scorev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// Manager handles Cilium-specific networking operations
type Manager struct {
	kubeClient kubernetes.Interface
	namespace  string
	logger     claberneteslogging.Instance
}

// NewManager creates a new Cilium networking manager
func NewManager(
	kubeClient kubernetes.Interface,
	namespace string,
	logger claberneteslogging.Instance,
) *Manager {
	return &Manager{
		kubeClient: kubeClient,
		namespace:  namespace,
		logger:     logger,
	}
}

// CreateNetworkConnectivity creates network connectivity between topology nodes using Cilium features
func (m *Manager) CreateNetworkConnectivity(ctx context.Context, topology *clabernetesapisv1alpha1.Topology) error {
	m.logger.Debugf("Creating Cilium network connectivity for topology %s", topology.Name)
	
	// Generate network policies for the topology
	policies, err := m.generateNetworkPolicies(topology)
	if err != nil {
		return fmt.Errorf("failed to generate network policies: %w", err)
	}
	
	// Apply network policies
	for _, policy := range policies {
		_, err := m.kubeClient.NetworkingV1().NetworkPolicies(m.namespace).Create(
			ctx, policy, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create network policy %s: %w", policy.Name, err)
		}
		m.logger.Debugf("Created network policy %s", policy.Name)
	}
	
	return nil
}

// DeleteNetworkConnectivity removes network connectivity for a topology
func (m *Manager) DeleteNetworkConnectivity(ctx context.Context, topology *clabernetesapisv1alpha1.Topology) error {
	m.logger.Debugf("Deleting Cilium network connectivity for topology %s", topology.Name)
	
	// List and delete network policies for this topology
	policies, err := m.kubeClient.NetworkingV1().NetworkPolicies(m.namespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", clabernetesconstants.LabelTopology, topology.Name),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to list network policies: %w", err)
	}
	
	for _, policy := range policies.Items {
		err := m.kubeClient.NetworkingV1().NetworkPolicies(m.namespace).Delete(
			ctx, policy.Name, metav1.DeleteOptions{},
		)
		if err != nil {
			m.logger.Warnf("Failed to delete network policy %s: %v", policy.Name, err)
		} else {
			m.logger.Debugf("Deleted network policy %s", policy.Name)
		}
	}
	
	return nil
}

// UpdateNetworkConnectivity updates network connectivity based on topology changes
func (m *Manager) UpdateNetworkConnectivity(ctx context.Context, topology *clabernetesapisv1alpha1.Topology) error {
	m.logger.Debugf("Updating Cilium network connectivity for topology %s", topology.Name)
	
	// For now, we'll delete and recreate - can be optimized later
	if err := m.DeleteNetworkConnectivity(ctx, topology); err != nil {
		return fmt.Errorf("failed to delete existing connectivity: %w", err)
	}
	
	return m.CreateNetworkConnectivity(ctx, topology)
}

// generateNetworkPolicies creates NetworkPolicy resources based on topology links
func (m *Manager) generateNetworkPolicies(topology *clabernetesapisv1alpha1.Topology) ([]*networkingv1.NetworkPolicy, error) {
	var policies []*networkingv1.NetworkPolicy
	
	// Get topology definition
	definition := topology.Spec.Definition
	if definition == nil {
		return policies, nil
	}
	
	// Create base policy that denies all traffic by default
	basePolicy := m.createDenyAllPolicy(topology)
	policies = append(policies, basePolicy)
	
	// Create management network policy (allow access to management services)
	mgmtPolicy := m.createManagementPolicy(topology)
	policies = append(policies, mgmtPolicy)
	
	// Process topology links to create connectivity policies
	linkPolicies := m.createLinkPolicies(topology, definition.Links)
	policies = append(policies, linkPolicies...)
	
	// Create policies for external access if specified
	externalPolicies := m.createExternalAccessPolicies(topology)
	policies = append(policies, externalPolicies...)
	
	return policies, nil
}

// createDenyAllPolicy creates a default deny-all network policy
func (m *Manager) createDenyAllPolicy(topology *clabernetesapisv1alpha1.Topology) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-deny-all", topology.Name),
			Namespace: m.namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopology: topology.Name,
				"clabernetes/policy-type":          "deny-all",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopology: topology.Name,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// Empty ingress/egress rules = deny all
		},
	}
}

// createManagementPolicy creates a policy allowing management traffic
func (m *Manager) createManagementPolicy(topology *clabernetesapisv1alpha1.Topology) *networkingv1.NetworkPolicy {
	// Allow traffic to/from management services
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-mgmt-allow", topology.Name),
			Namespace: m.namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopology: topology.Name,
				"clabernetes/policy-type":          "management",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopology: topology.Name,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow traffic from management namespace
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"name": "kube-system",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.FromInt(22),  // SSH
							Protocol: &protocolTCP,
						},
						{
							Port:     &intstr.FromInt(830), // NETCONF
							Protocol: &protocolTCP,
						},
						{
							Port:     &intstr.FromInt(57400), // gNMI
							Protocol: &protocolTCP,
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow DNS
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.FromInt(53),
							Protocol: &protocolUDP,
						},
						{
							Port:     &intstr.FromInt(53),
							Protocol: &protocolTCP,
						},
					},
				},
				{
					// Allow external access for updates, etc.
					To: []networkingv1.NetworkPolicyPeer{},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.FromInt(80),
							Protocol: &protocolTCP,
						},
						{
							Port:     &intstr.FromInt(443),
							Protocol: &protocolTCP,
						},
					},
				},
			},
		},
	}
}

// createLinkPolicies creates network policies for topology links
func (m *Manager) createLinkPolicies(topology *clabernetesapisv1alpha1.Topology, links []interface{}) []*networkingv1.NetworkPolicy {
	var policies []*networkingv1.NetworkPolicy
	
	// Track which nodes need connectivity
	nodeConnections := make(map[string][]string)
	
	// Process links to build connectivity map
	for _, linkInterface := range links {
		link, ok := linkInterface.(map[string]interface{})
		if !ok {
			continue
		}
		
		endpoints := link["endpoints"]
		if endpoints == nil {
			continue
		}
		
		endpointList, ok := endpoints.([]interface{})
		if !ok || len(endpointList) != 2 {
			continue
		}
		
		// Extract node names from endpoints
		var nodeA, nodeB string
		if endpoint0, ok := endpointList[0].(string); ok {
			nodeA = extractNodeName(endpoint0)
		}
		if endpoint1, ok := endpointList[1].(string); ok {
			nodeB = extractNodeName(endpoint1)
		}
		
		if nodeA != "" && nodeB != "" {
			nodeConnections[nodeA] = append(nodeConnections[nodeA], nodeB)
			nodeConnections[nodeB] = append(nodeConnections[nodeB], nodeA)
		}
	}
	
	// Create policies for each node's connections
	for sourceNode, targetNodes := range nodeConnections {
		policy := m.createNodeConnectivityPolicy(topology, sourceNode, targetNodes)
		policies = append(policies, policy)
	}
	
	return policies
}

// createNodeConnectivityPolicy creates a policy allowing connectivity between specific nodes
func (m *Manager) createNodeConnectivityPolicy(topology *clabernetesapisv1alpha1.Topology, sourceNode string, targetNodes []string) *networkingv1.NetworkPolicy {
	// Create peer selectors for target nodes
	var peers []networkingv1.NetworkPolicyPeer
	for _, targetNode := range targetNodes {
		peers = append(peers, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopologyNode: targetNode,
				},
			},
		})
	}
	
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-connectivity", topology.Name, sourceNode),
			Namespace: m.namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopology:     topology.Name,
				clabernetesconstants.LabelTopologyNode: sourceNode,
				"clabernetes/policy-type":              "link-connectivity",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopologyNode: sourceNode,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: peers,
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: peers,
				},
			},
		},
	}
}

// createExternalAccessPolicies creates policies for external access
func (m *Manager) createExternalAccessPolicies(topology *clabernetesapisv1alpha1.Topology) []*networkingv1.NetworkPolicy {
	var policies []*networkingv1.NetworkPolicy
	
	// Create policy for external access to specific services
	externalPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-external-access", topology.Name),
			Namespace: m.namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopology: topology.Name,
				"clabernetes/policy-type":          "external-access",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopology: topology.Name,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow external access to management ports
					From: []networkingv1.NetworkPolicyPeer{},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.FromInt(22),
							Protocol: &protocolTCP,
						},
						{
							Port:     &intstr.FromInt(80),
							Protocol: &protocolTCP,
						},
						{
							Port:     &intstr.FromInt(443),
							Protocol: &protocolTCP,
						},
					},
				},
			},
		},
	}
	
	policies = append(policies, externalPolicy)
	
	return policies
}

// extractNodeName extracts the node name from an endpoint string
func extractNodeName(endpoint string) string {
	// Handle formats like "node1:eth1" or just "node1"
	if colonIndex := strings.Index(endpoint, ":"); colonIndex != -1 {
		return endpoint[:colonIndex]
	}
	return endpoint
}

// Protocol and port helper variables
var (
	protocolTCP = k8scorev1.ProtocolTCP
	protocolUDP = k8scorev1.ProtocolUDP
)