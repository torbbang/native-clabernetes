package common

import (
	"context"
	"fmt"
	"strings"

	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	clabernetesgeneratedclientset "github.com/srl-labs/clabernetes/generated/clientset"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ExecutionMode represents the execution strategy for workloads
type ExecutionMode string

const (
	// ExecutionModeContainer runs workloads as native Kubernetes containers
	ExecutionModeContainer ExecutionMode = "container"
	// ExecutionModeVM runs workloads as KubeVirt virtual machines
	ExecutionModeVM ExecutionMode = "vm"
	// ExecutionModeAuto automatically detects the best execution mode
	ExecutionModeAuto ExecutionMode = "auto"
	// ExecutionModeLegacy uses the legacy Docker-in-Docker approach
	ExecutionModeLegacy ExecutionMode = "legacy"
	// ExecutionModeHybrid supports mixed execution modes
	ExecutionModeHybrid ExecutionMode = "hybrid"
)

// WorkloadType represents the type of workload to be executed
type WorkloadType string

const (
	// WorkloadTypeContainer represents a container-based workload
	WorkloadTypeContainer WorkloadType = "container"
	// WorkloadTypeVM represents a virtual machine-based workload
	WorkloadTypeVM WorkloadType = "vm"
)

// NodeConfig represents the configuration for a topology node
type NodeConfig struct {
	// Name is the name of the node
	Name string
	// Image is the container or VM image to use
	Image string
	// Kind is the type of node (e.g., "srl", "ceos", "vyos")
	Kind string
	// Labels are additional labels to apply to the workload
	Labels map[string]string
	// Annotations are additional annotations to apply to the workload
	Annotations map[string]string
	// Environment variables for the workload
	Environment map[string]string
	// Interfaces are the network interfaces for this node
	Interfaces []NetworkInterface
	// Resources specify resource requirements
	Resources *k8scorev1.ResourceRequirements
	// StartupConfig contains any startup configuration
	StartupConfig string
	// Files contains additional files to mount
	Files map[string]string
}

// NetworkInterface represents a network interface configuration
type NetworkInterface struct {
	// Name is the interface name
	Name string
	// Type is the interface type (e.g., "ethernet", "bridge")
	Type string
	// Endpoint is the remote endpoint for this interface
	Endpoint *NetworkEndpoint
}

// NetworkEndpoint represents a network connection endpoint
type NetworkEndpoint struct {
	// Node is the remote node name
	Node string
	// Interface is the remote interface name
	Interface string
	// Address is the IP address (if applicable)
	Address string
}

// ExecutionResult represents the result of workload execution
type ExecutionResult struct {
	// WorkloadType indicates what type of workload was created
	WorkloadType WorkloadType
	// Name is the name of the created workload
	Name string
	// Namespace is the namespace of the created workload
	Namespace string
	// Status is the current status of the workload
	Status string
	// Message contains additional status information
	Message string
	// Ready indicates if the workload is ready
	Ready bool
}

// Executor defines the interface for workload execution
type Executor interface {
	// Execute creates and starts a workload based on the node configuration
	Execute(ctx context.Context, config *NodeConfig) (*ExecutionResult, error)
	
	// Delete removes a workload
	Delete(ctx context.Context, name, namespace string) error
	
	// GetStatus returns the current status of a workload
	GetStatus(ctx context.Context, name, namespace string) (*ExecutionResult, error)
	
	// GetLogs returns logs from the workload
	GetLogs(ctx context.Context, name, namespace string) (string, error)
	
	// GetWorkloadType returns the type of workload this executor handles
	GetWorkloadType() WorkloadType
}

// Manager coordinates multiple executors for different workload types
type Manager struct {
	ctx                   context.Context
	logger                claberneteslogging.Instance
	kubeClient            kubernetes.Interface
	clabernetesClient     *clabernetesgeneratedclientset.Clientset
	namespace             string
	executors             map[WorkloadType]Executor
	defaultExecutionMode  ExecutionMode
}

// NewManager creates a new execution manager
func NewManager(
	ctx context.Context,
	logger claberneteslogging.Instance,
	kubeClient kubernetes.Interface,
	clabernetesClient *clabernetesgeneratedclientset.Clientset,
	namespace string,
	executionMode ExecutionMode,
) *Manager {
	return &Manager{
		ctx:                  ctx,
		logger:               logger,
		kubeClient:           kubeClient,
		clabernetesClient:    clabernetesClient,
		namespace:            namespace,
		executors:            make(map[WorkloadType]Executor),
		defaultExecutionMode: executionMode,
	}
}

// RegisterExecutor registers an executor for a specific workload type
func (m *Manager) RegisterExecutor(workloadType WorkloadType, executor Executor) {
	m.executors[workloadType] = executor
}

// Execute creates a workload using the appropriate executor
func (m *Manager) Execute(ctx context.Context, config *NodeConfig) (*ExecutionResult, error) {
	workloadType := m.determineWorkloadType(config)
	
	executor, exists := m.executors[workloadType]
	if !exists {
		m.logger.Warnf("No executor registered for workload type %s, falling back to container", workloadType)
		executor = m.executors[WorkloadTypeContainer]
	}
	
	if executor == nil {
		return nil, fmt.Errorf("no executor available for workload type %s", workloadType)
	}
	
	return executor.Execute(ctx, config)
}

// determineWorkloadType decides which workload type to use for a node
func (m *Manager) determineWorkloadType(config *NodeConfig) WorkloadType {
	// For now, implement basic logic - this will be enhanced in workload/detector
	vmImages := []string{
		"cisco/csr1000v", "arista/veos", "juniper/vmx",
		"vyos/vyos", "pfsense/pfsense", "opnsense/opnsense",
		"mikrotik/routeros", "fortinet/fortigate",
	}
	
	for _, vmImage := range vmImages {
		if strings.Contains(strings.ToLower(config.Image), vmImage) {
			return WorkloadTypeVM
		}
	}
	
	return WorkloadTypeContainer
}

// NodeSpec represents the specification for a topology node in the native architecture
type NodeSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	
	// Spec defines the desired state of the node
	Spec NodeSpecDefinition `json:"spec,omitempty"`
	
	// Status defines the observed state of the node
	Status NodeStatus `json:"status,omitempty"`
}

// NodeSpecDefinition defines the specification for a node
type NodeSpecDefinition struct {
	// ExecutionMode specifies how this node should be executed
	ExecutionMode ExecutionMode `json:"executionMode,omitempty"`
	
	// Image is the container or VM image to use
	Image string `json:"image"`
	
	// Kind is the type of node
	Kind string `json:"kind"`
	
	// Config contains node-specific configuration
	Config map[string]string `json:"config,omitempty"`
	
	// Networking defines network configuration
	Networking NodeNetworking `json:"networking,omitempty"`
	
	// Resources specify resource requirements
	Resources *k8scorev1.ResourceRequirements `json:"resources,omitempty"`
}

// NodeNetworking defines network configuration for a node
type NodeNetworking struct {
	// Interfaces defines the network interfaces
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`
	
	// ManagementIP is the management IP address
	ManagementIP string `json:"managementIP,omitempty"`
	
	// NetworkPolicies are custom network policies for this node
	NetworkPolicies []string `json:"networkPolicies,omitempty"`
}

// NodeStatus represents the status of a node
type NodeStatus struct {
	// Phase is the current phase of the node
	Phase string `json:"phase,omitempty"`
	
	// Ready indicates if the node is ready
	Ready bool `json:"ready"`
	
	// WorkloadType indicates what type of workload was created
	WorkloadType WorkloadType `json:"workloadType,omitempty"`
	
	// IPAddress is the assigned IP address
	IPAddress string `json:"ipAddress,omitempty"`
	
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// Message contains human-readable message indicating details about the node
	Message string `json:"message,omitempty"`
}