package vm

import (
	"context"
	"fmt"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VMExecutor implements the Executor interface for KubeVirt virtual machine workloads
type VMExecutor struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	logger        claberneteslogging.Instance
}

// NewVMExecutor creates a new VM executor
func NewVMExecutor(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	logger claberneteslogging.Instance,
) *VMExecutor {
	return &VMExecutor{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		logger:        logger,
	}
}

// Execute creates and starts a VM workload
func (e *VMExecutor) Execute(ctx context.Context, config *common.NodeConfig) (*common.ExecutionResult, error) {
	e.logger.Debugf("Creating VM workload for node %s", config.Name)
	
	// Check if KubeVirt is available
	if !e.isKubeVirtAvailable(ctx) {
		return nil, fmt.Errorf("KubeVirt is not available in the cluster")
	}
	
	// Create VirtualMachine resource
	vm := e.renderVirtualMachine(config)
	
	// Convert to unstructured for dynamic client
	vmUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to convert VM to unstructured: %w", err)
	}
	
	vmResource := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	
	createdVM, err := e.dynamicClient.Resource(vmResource).Namespace(e.namespace).Create(
		ctx, &unstructured.Unstructured{Object: vmUnstructured}, metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM for node %s: %w", config.Name, err)
	}
	
	// Create service for the VM
	service := e.renderService(config)
	_, err = e.kubeClient.CoreV1().Services(e.namespace).Create(
		ctx, service, metav1.CreateOptions{},
	)
	if err != nil {
		e.logger.Warnf("Failed to create service for VM %s: %v", config.Name, err)
	}
	
	return &common.ExecutionResult{
		WorkloadType: common.WorkloadTypeVM,
		Name:         createdVM.GetName(),
		Namespace:    createdVM.GetNamespace(),
		Status:       "Creating",
		Ready:        false,
		Message:      "Virtual machine created successfully",
	}, nil
}

// Delete removes a VM workload
func (e *VMExecutor) Delete(ctx context.Context, name, namespace string) error {
	e.logger.Debugf("Deleting VM workload %s in namespace %s", name, namespace)
	
	vmResource := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	
	// Delete VM
	err := e.dynamicClient.Resource(vmResource).Namespace(namespace).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete VM %s: %w", name, err)
	}
	
	// Delete service
	err = e.kubeClient.CoreV1().Services(namespace).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
	if err != nil {
		e.logger.Warnf("Failed to delete service %s: %v", name, err)
	}
	
	return nil
}

// GetStatus returns the current status of a VM workload
func (e *VMExecutor) GetStatus(ctx context.Context, name, namespace string) (*common.ExecutionResult, error) {
	vmResource := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	
	vm, err := e.dynamicClient.Resource(vmResource).Namespace(namespace).Get(
		ctx, name, metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM %s: %w", name, err)
	}
	
	// Extract status from the VM
	status := "Creating"
	ready := false
	message := "Virtual machine is starting"
	
	// Check VM status
	if vmStatus, found, _ := unstructured.NestedMap(vm.Object, "status"); found {
		if ready, found, _ := unstructured.NestedBool(vmStatus, "ready"); found && ready {
			status = "Running"
			message = "Virtual machine is running"
		} else if printableStatus, found, _ := unstructured.NestedString(vmStatus, "printableStatus"); found {
			status = printableStatus
			message = fmt.Sprintf("Virtual machine status: %s", printableStatus)
		}
	}
	
	return &common.ExecutionResult{
		WorkloadType: common.WorkloadTypeVM,
		Name:         vm.GetName(),
		Namespace:    vm.GetNamespace(),
		Status:       status,
		Ready:        ready,
		Message:      message,
	}, nil
}

// GetLogs returns logs from the VM workload
func (e *VMExecutor) GetLogs(ctx context.Context, name, namespace string) (string, error) {
	// For VMs, we need to get logs from the virt-launcher pod
	pods, err := e.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubevirt.io/created-by=%s", name),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for VM %s: %w", name, err)
	}
	
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for VM %s", name)
	}
	
	// Get logs from the virt-launcher container
	pod := pods.Items[0]
	logOptions := &k8scorev1.PodLogOptions{
		Container: "compute",
	}
	
	logStream, err := e.kubeClient.CoreV1().Pods(namespace).GetLogs(pod.Name, logOptions).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for VM pod %s: %w", pod.Name, err)
	}
	defer logStream.Close()
	
	// Read logs (simplified for now)
	return "VM logs would be streamed here", nil
}

// GetWorkloadType returns the workload type this executor handles
func (e *VMExecutor) GetWorkloadType() common.WorkloadType {
	return common.WorkloadTypeVM
}

// isKubeVirtAvailable checks if KubeVirt is available in the cluster
func (e *VMExecutor) isKubeVirtAvailable(ctx context.Context) bool {
	// Check if the kubevirt namespace exists
	_, err := e.kubeClient.CoreV1().Namespaces().Get(ctx, "kubevirt", metav1.GetOptions{})
	return err == nil
}

// renderVirtualMachine creates a KubeVirt VirtualMachine resource
func (e *VMExecutor) renderVirtualMachine(config *common.NodeConfig) map[string]interface{} {
	labels := map[string]string{
		"app":                               config.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
		"clabernetes/execution-mode":        "native",
		"clabernetes/workload-type":         "vm",
		"kubevirt.io/vm":                    config.Name,
	}
	
	// Merge additional labels
	for k, v := range config.Labels {
		labels[k] = v
	}
	
	annotations := map[string]string{
		"clabernetes/node-kind": config.Kind,
		"clabernetes/image":     config.Image,
	}
	
	// Merge additional annotations
	for k, v := range config.Annotations {
		annotations[k] = v
	}
	
	// Generate network interfaces
	interfaces := []map[string]interface{}{
		{
			"name": "default",
			"masquerade": map[string]interface{}{},
		},
	}
	
	// Add data interfaces for topology links
	for i, intf := range config.Interfaces {
		interfaces = append(interfaces, map[string]interface{}{
			"name": fmt.Sprintf("net%d", i+1),
			"bridge": map[string]interface{}{},
		})
	}
	
	// Generate networks
	networks := []map[string]interface{}{
		{
			"name": "default",
			"pod":  map[string]interface{}{},
		},
	}
	
	// Add networks for data interfaces
	for i := range config.Interfaces {
		networks = append(networks, map[string]interface{}{
			"name": fmt.Sprintf("net%d", i+1),
			"multus": map[string]interface{}{
				"networkName": fmt.Sprintf("%s-net%d", config.Name, i+1),
			},
		})
	}
	
	// Determine memory and CPU based on node kind
	memory := "1Gi"
	cpu := "1"
	
	// Adjust resources for specific node types
	switch config.Kind {
	case "vyos", "opnsense", "pfsense":
		memory = "2Gi"
		cpu = "2"
	case "csr1000v", "vmx":
		memory = "4Gi"
		cpu = "2"
	}
	
	// Override with custom resources if specified
	if config.Resources != nil {
		if mem := config.Resources.Requests[k8scorev1.ResourceMemory]; !mem.IsZero() {
			memory = mem.String()
		}
		if cpuRes := config.Resources.Requests[k8scorev1.ResourceCPU]; !cpuRes.IsZero() {
			cpu = cpuRes.String()
		}
	}
	
	vm := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":        config.Name,
			"namespace":   e.namespace,
			"labels":      labels,
			"annotations": annotations,
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
							"disks": []map[string]interface{}{
								{
									"name": "containerdisk",
									"disk": map[string]interface{}{
										"bus": "virtio",
									},
								},
								{
									"name": "cloudinitdisk",
									"disk": map[string]interface{}{
										"bus": "virtio",
									},
								},
							},
							"interfaces": interfaces,
						},
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"memory": memory,
								"cpu":    cpu,
							},
						},
					},
					"networks": networks,
					"volumes": []map[string]interface{}{
						{
							"name": "containerdisk",
							"containerDisk": map[string]interface{}{
								"image": config.Image,
							},
						},
						{
							"name": "cloudinitdisk",
							"cloudInitNoCloud": map[string]interface{}{
								"userData": e.generateCloudInitUserData(config),
							},
						},
					},
				},
			},
		},
	}
	
	return vm
}

// generateCloudInitUserData generates cloud-init user data for the VM
func (e *VMExecutor) generateCloudInitUserData(config *common.NodeConfig) string {
	// Basic cloud-init configuration
	userData := `#cloud-config
hostname: ` + config.Name + `
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... # Add your SSH key here
runcmd:
  - echo "Node ` + config.Name + ` started" > /var/log/clabernetes-init.log
`
	
	// Add startup config if provided
	if config.StartupConfig != "" {
		userData += `  - echo "` + config.StartupConfig + `" > /tmp/startup-config.txt
`
	}
	
	return userData
}

// renderService creates a Kubernetes service for the VM
func (e *VMExecutor) renderService(config *common.NodeConfig) *k8scorev1.Service {
	labels := map[string]string{
		"app": config.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
	}
	
	ports := []k8scorev1.ServicePort{
		{
			Name:     "ssh",
			Port:     22,
			Protocol: k8scorev1.ProtocolTCP,
		},
	}
	
	// Add ports specific to VM types
	switch config.Kind {
	case "vyos":
		ports = append(ports, k8scorev1.ServicePort{
			Name:     "api",
			Port:     443,
			Protocol: k8scorev1.ProtocolTCP,
		})
	case "pfsense", "opnsense":
		ports = append(ports, k8scorev1.ServicePort{
			Name:     "web",
			Port:     80,
			Protocol: k8scorev1.ProtocolTCP,
		})
		ports = append(ports, k8scorev1.ServicePort{
			Name:     "https",
			Port:     443,
			Protocol: k8scorev1.ProtocolTCP,
		})
	}
	
	return &k8scorev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: e.namespace,
			Labels:    labels,
		},
		Spec: k8scorev1.ServiceSpec{
			Selector: map[string]string{
				"kubevirt.io/vm": config.Name,
			},
			Ports: ports,
			Type:  k8scorev1.ServiceTypeClusterIP,
		},
	}
}