package renderer

import (
	"fmt"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
	"github.com/srl-labs/clabernetes/pkg/workload/detector"
	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	clabernetesutilcontainerlab "github.com/srl-labs/clabernetes/util/containerlab"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// WorkloadRenderer generates Kubernetes resources for topology nodes
type WorkloadRenderer struct {
	classifier *detector.WorkloadClassifier
	logger     claberneteslogging.Instance
}

// NewWorkloadRenderer creates a new workload renderer
func NewWorkloadRenderer(
	classifier *detector.WorkloadClassifier,
	logger claberneteslogging.Instance,
) *WorkloadRenderer {
	return &WorkloadRenderer{
		classifier: classifier,
		logger:     logger,
	}
}

// RenderResult contains the rendered resources for a node
type RenderResult struct {
	// WorkloadType indicates what type of workload was rendered
	WorkloadType common.WorkloadType
	// Resources contains the rendered Kubernetes resources
	Resources []Resource
	// NodeConfig contains the node configuration used for rendering
	NodeConfig *common.NodeConfig
}

// Resource represents a rendered Kubernetes resource
type Resource struct {
	// Type is the resource type (Deployment, VirtualMachine, Service, etc.)
	Type string
	// Object is the actual Kubernetes object
	Object interface{}
	// Dependencies are other resources this one depends on
	Dependencies []string
}

// RenderTopologyWorkloads renders all workloads for a topology
func (r *WorkloadRenderer) RenderTopologyWorkloads(
	topology *clabernetesapisv1alpha1.Topology,
	configs map[string]*clabernetesutilcontainerlab.NodeDefinition,
	namespace string,
) (map[string]*RenderResult, error) {
	r.logger.Debugf("Rendering workloads for topology %s", topology.Name)
	
	results := make(map[string]*RenderResult)
	
	// Process each node in the topology
	for nodeName, config := range configs {
		nodeConfig := r.buildNodeConfig(nodeName, config, topology)
		
		// Determine workload type
		workloadType := r.classifier.DetermineWorkloadType(nodeConfig)
		
		// Render resources based on workload type
		resources, err := r.renderNodeResources(nodeConfig, workloadType, topology, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to render resources for node %s: %w", nodeName, err)
		}
		
		results[nodeName] = &RenderResult{
			WorkloadType: workloadType,
			Resources:    resources,
			NodeConfig:   nodeConfig,
		}
		
		r.logger.Debugf("Rendered %s workload for node %s with %d resources", 
			workloadType, nodeName, len(resources))
	}
	
	return results, nil
}

// buildNodeConfig converts containerlab config to common node config
func (r *WorkloadRenderer) buildNodeConfig(
	nodeName string,
	config *clabernetesutilcontainerlab.NodeDefinition,
	topology *clabernetesapisv1alpha1.Topology,
) *common.NodeConfig {
	nodeConfig := &common.NodeConfig{
		Name:        nodeName,
		Image:       config.Image,
		Kind:        config.Kind,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Environment: make(map[string]string),
		Interfaces:  []common.NetworkInterface{},
		Files:       make(map[string]string),
	}
	
	// Add topology labels
	nodeConfig.Labels[clabernetesconstants.LabelTopology] = topology.Name
	nodeConfig.Labels[clabernetesconstants.LabelTopologyNode] = nodeName
	nodeConfig.Labels[clabernetesconstants.LabelNodeKind] = config.Kind
	
	// Add execution mode from topology spec
	if topology.Spec.NativeExecution.ExecutionMode != "" {
		nodeConfig.Environment[clabernetesconstants.ExecutionModeEnv] = string(topology.Spec.NativeExecution.ExecutionMode)
		nodeConfig.Labels[clabernetesconstants.LabelExecutionMode] = string(topology.Spec.NativeExecution.ExecutionMode)
	}
	
	// Add networking mode
	if topology.Spec.NativeExecution.Networking.CNI != "" {
		nodeConfig.Environment[clabernetesconstants.NetworkingModeEnv] = topology.Spec.NativeExecution.Networking.CNI
		nodeConfig.Labels[clabernetesconstants.LabelNetworkingMode] = topology.Spec.NativeExecution.Networking.CNI
	}
	
	// Apply node overrides if specified
	if override, exists := topology.Spec.NativeExecution.NodeOverrides[nodeName]; exists {
		if override.ExecutionMode != "" {
			nodeConfig.Environment[clabernetesconstants.ExecutionModeEnv] = string(override.ExecutionMode)
			nodeConfig.Labels[clabernetesconstants.LabelExecutionMode] = string(override.ExecutionMode)
		}
		if override.Resources != nil {
			nodeConfig.Resources = override.Resources
		}
		for k, v := range override.Config {
			nodeConfig.Environment[k] = v
		}
	}
	
	// Convert containerlab-specific config
	if config.Env != nil {
		for k, v := range config.Env {
			nodeConfig.Environment[k] = v
		}
	}
	
	if config.Labels != nil {
		for k, v := range config.Labels {
			nodeConfig.Labels[k] = v
		}
	}
	
	// Note: Network interfaces will be handled separately from link definitions
	// For now, we'll create a placeholder interface
	if len(nodeConfig.Interfaces) == 0 {
		nodeConfig.Interfaces = append(nodeConfig.Interfaces, common.NetworkInterface{
			Name: "eth0",
			Type: "ethernet",
		})
	}
	
	// Add startup config if available
	if config.StartupConfig != "" {
		nodeConfig.StartupConfig = config.StartupConfig
	}
	
	return nodeConfig
}

// renderNodeResources renders Kubernetes resources for a single node
func (r *WorkloadRenderer) renderNodeResources(
	config *common.NodeConfig,
	workloadType common.WorkloadType,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) ([]Resource, error) {
	var resources []Resource
	
	switch workloadType {
	case common.WorkloadTypeContainer:
		containerResources, err := r.renderContainerResources(config, topology, namespace)
		if err != nil {
			return nil, err
		}
		resources = append(resources, containerResources...)
		
	case common.WorkloadTypeVM:
		vmResources, err := r.renderVMResources(config, topology, namespace)
		if err != nil {
			return nil, err
		}
		resources = append(resources, vmResources...)
		
	default:
		return nil, fmt.Errorf("unsupported workload type: %s", workloadType)
	}
	
	// Add common resources (ConfigMaps, Services)
	commonResources, err := r.renderCommonResources(config, topology, namespace)
	if err != nil {
		return nil, err
	}
	resources = append(resources, commonResources...)
	
	return resources, nil
}

// renderContainerResources renders resources for container workloads
func (r *WorkloadRenderer) renderContainerResources(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) ([]Resource, error) {
	var resources []Resource
	
	// Create Deployment
	deployment := r.buildDeployment(config, topology, namespace)
	resources = append(resources, Resource{
		Type:   "Deployment",
		Object: deployment,
	})
	
	// Create Service
	service := r.buildService(config, topology, namespace)
	resources = append(resources, Resource{
		Type:         "Service",
		Object:       service,
		Dependencies: []string{deployment.Name},
	})
	
	return resources, nil
}

// renderVMResources renders resources for VM workloads
func (r *WorkloadRenderer) renderVMResources(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) ([]Resource, error) {
	var resources []Resource
	
	// Create VirtualMachine (as unstructured for now)
	vm := r.buildVirtualMachine(config, topology, namespace)
	resources = append(resources, Resource{
		Type:   "VirtualMachine",
		Object: vm,
	})
	
	// Create Service
	service := r.buildService(config, topology, namespace)
	resources = append(resources, Resource{
		Type:         "Service",
		Object:       service,
		Dependencies: []string{config.Name + "-vm"},
	})
	
	return resources, nil
}

// renderCommonResources renders resources common to all workload types
func (r *WorkloadRenderer) renderCommonResources(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) ([]Resource, error) {
	var resources []Resource
	
	// Create ConfigMap for node configuration if needed
	if len(config.Files) > 0 || config.StartupConfig != "" {
		configMap := r.buildConfigMap(config, topology, namespace)
		resources = append(resources, Resource{
			Type:   "ConfigMap",
			Object: configMap,
		})
	}
	
	return resources, nil
}

// buildDeployment creates a Kubernetes Deployment for container workloads
func (r *WorkloadRenderer) buildDeployment(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) *k8sappsv1.Deployment {
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}
	labels[clabernetesconstants.LabelWorkloadType] = clabernetesconstants.WorkloadTypeContainer
	
	annotations := make(map[string]string)
	for k, v := range config.Annotations {
		annotations[k] = v
	}
	
	// Environment variables
	env := []k8scorev1.EnvVar{}
	for k, v := range config.Environment {
		env = append(env, k8scorev1.EnvVar{Name: k, Value: v})
	}
	
	// Container ports
	ports := []k8scorev1.ContainerPort{
		{Name: "ssh", ContainerPort: 22, Protocol: k8scorev1.ProtocolTCP},
		{Name: "netconf", ContainerPort: 830, Protocol: k8scorev1.ProtocolTCP},
		{Name: "gnmi", ContainerPort: 57400, Protocol: k8scorev1.ProtocolTCP},
	}
	
	container := k8scorev1.Container{
		Name:  config.Name,
		Image: config.Image,
		Env:   env,
		Ports: ports,
		SecurityContext: &k8scorev1.SecurityContext{
			Capabilities: &k8scorev1.Capabilities{
				Add: []k8scorev1.Capability{"NET_ADMIN"},
			},
		},
		ImagePullPolicy: k8scorev1.PullIfNotPresent,
	}
	
	if config.Resources != nil {
		container.Resources = *config.Resources
	}
	
	replicas := int32(1)
	
	return &k8sappsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.Name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: k8sappsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					clabernetesconstants.LabelTopologyNode: config.Name,
				},
			},
			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: k8scorev1.PodSpec{
					Containers:    []k8scorev1.Container{container},
					RestartPolicy: k8scorev1.RestartPolicyAlways,
				},
			},
		},
	}
}

// buildVirtualMachine creates a KubeVirt VirtualMachine resource
func (r *WorkloadRenderer) buildVirtualMachine(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) *unstructured.Unstructured {
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}
	labels[clabernetesconstants.LabelWorkloadType] = clabernetesconstants.WorkloadTypeVM
	labels["kubevirt.io/vm"] = config.Name
	
	// Basic VM specification
	vm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      config.Name,
				"namespace": namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"running": true,
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": labels,
					},
					"spec": map[string]interface{}{
						"domain": map[string]interface{}{
							"devices": map[string]interface{}{
								"disks": []interface{}{
									map[string]interface{}{
										"name": "containerdisk",
										"disk": map[string]interface{}{
											"bus": "virtio",
										},
									},
								},
								"interfaces": []interface{}{
									map[string]interface{}{
										"name":       "default",
										"masquerade": map[string]interface{}{},
									},
								},
							},
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"memory": "1Gi",
									"cpu":    "1",
								},
							},
						},
						"networks": []interface{}{
							map[string]interface{}{
								"name": "default",
								"pod":  map[string]interface{}{},
							},
						},
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "containerdisk",
								"containerDisk": map[string]interface{}{
									"image": config.Image,
								},
							},
						},
					},
				},
			},
		},
	}
	
	return vm
}

// buildService creates a Kubernetes Service for a node
func (r *WorkloadRenderer) buildService(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) *k8scorev1.Service {
	labels := map[string]string{
		clabernetesconstants.LabelTopology:     topology.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
	}
	
	ports := []k8scorev1.ServicePort{
		{Name: "ssh", Port: 22, Protocol: k8scorev1.ProtocolTCP},
		{Name: "netconf", Port: 830, Protocol: k8scorev1.ProtocolTCP},
		{Name: "gnmi", Port: 57400, Protocol: k8scorev1.ProtocolTCP},
	}
	
	return &k8scorev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: k8scorev1.ServiceSpec{
			Selector: map[string]string{
				clabernetesconstants.LabelTopologyNode: config.Name,
			},
			Ports: ports,
			Type:  k8scorev1.ServiceTypeClusterIP,
		},
	}
}

// buildConfigMap creates a ConfigMap for node configuration
func (r *WorkloadRenderer) buildConfigMap(
	config *common.NodeConfig,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) *k8scorev1.ConfigMap {
	labels := map[string]string{
		clabernetesconstants.LabelTopology:     topology.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
	}
	
	data := make(map[string]string)
	
	// Add files
	for filename, content := range config.Files {
		data[filename] = content
	}
	
	// Add startup config
	if config.StartupConfig != "" {
		data["startup-config"] = config.StartupConfig
	}
	
	return &k8scorev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name + "-config",
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}