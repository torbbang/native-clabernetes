package detector

import (
	"fmt"
	"strings"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
)

// WorkloadClassifier determines the appropriate workload type for topology nodes
type WorkloadClassifier struct {
	logger        claberneteslogging.Instance
	vmImageMap    map[string]bool
	forceVM       map[string]bool
	forceContainer map[string]bool
}

// NewWorkloadClassifier creates a new workload classifier
func NewWorkloadClassifier(logger claberneteslogging.Instance) *WorkloadClassifier {
	// Define known VM images/kinds that should run as VMs
	vmImageMap := map[string]bool{
		// Cisco
		"cisco/csr1000v":     true,
		"cisco/iosv":         true,
		"cisco/iosxr":        true,
		"cisco/nxos":         true,
		
		// Arista
		"arista/veos":        true,
		"arista/ceos":        false, // cEOS runs as container
		
		// Juniper
		"juniper/vmx":        true,
		"juniper/vsrx":       true,
		"juniper/vqfx":       true,
		
		// Open source routers/firewalls
		"vyos/vyos":          true,
		"pfsense/pfsense":    true,
		"opnsense/opnsense":  true,
		
		// MikroTik
		"mikrotik/routeros":  true,
		"mikrotik/chr":       true,
		
		// Fortinet
		"fortinet/fortigate": true,
		
		// Nokia (SR Linux runs as container)
		"nokia/srl":          false,
		"nokia/srlinux":      false,
		
		// SONiC (typically container)
		"sonic/sonic":        false,
		"azure/sonic":        false,
		
		// FRR (typically container)
		"frr/frr":            false,
		"quagga/quagga":      false,
		
		// Linux (container)
		"alpine":             false,
		"ubuntu":             false,
		"centos":             false,
		"debian":             false,
	}
	
	return &WorkloadClassifier{
		logger:         logger,
		vmImageMap:     vmImageMap,
		forceVM:        make(map[string]bool),
		forceContainer: make(map[string]bool),
	}
}

// DetermineWorkloadType analyzes a node configuration and determines whether it should
// run as a container or virtual machine
func (c *WorkloadClassifier) DetermineWorkloadType(config *common.NodeConfig) common.WorkloadType {
	c.logger.Debugf("Determining workload type for node %s with image %s and kind %s", 
		config.Name, config.Image, config.Kind)
	
	// Check for explicit execution mode override in config
	if execMode, exists := config.Environment["EXECUTION_MODE"]; exists {
		switch strings.ToLower(execMode) {
		case "vm", "virtual-machine":
			c.logger.Debugf("Node %s forced to VM by EXECUTION_MODE environment variable", config.Name)
			return common.WorkloadTypeVM
		case "container", "pod":
			c.logger.Debugf("Node %s forced to container by EXECUTION_MODE environment variable", config.Name)
			return common.WorkloadTypeContainer
		}
	}
	
	// Check forced classifications
	if c.forceVM[config.Name] {
		c.logger.Debugf("Node %s forced to VM by classifier configuration", config.Name)
		return common.WorkloadTypeVM
	}
	
	if c.forceContainer[config.Name] {
		c.logger.Debugf("Node %s forced to container by classifier configuration", config.Name)
		return common.WorkloadTypeContainer
	}
	
	// Analyze by image name
	workloadType := c.classifyByImage(config.Image)
	if workloadType != "" {
		c.logger.Debugf("Node %s classified as %s based on image %s", config.Name, workloadType, config.Image)
		return workloadType
	}
	
	// Analyze by node kind
	workloadType = c.classifyByKind(config.Kind)
	if workloadType != "" {
		c.logger.Debugf("Node %s classified as %s based on kind %s", config.Name, workloadType, config.Kind)
		return workloadType
	}
	
	// Analyze by image characteristics
	workloadType = c.classifyByImageCharacteristics(config.Image)
	if workloadType != "" {
		c.logger.Debugf("Node %s classified as %s based on image characteristics", config.Name, workloadType)
		return workloadType
	}
	
	// Default to container for unknown types
	c.logger.Debugf("Node %s defaulting to container workload type", config.Name)
	return common.WorkloadTypeContainer
}

// classifyByImage determines workload type based on the container image
func (c *WorkloadClassifier) classifyByImage(image string) common.WorkloadType {
	imageLower := strings.ToLower(image)
	
	// Check exact matches first
	for imagePattern, isVM := range c.vmImageMap {
		if strings.Contains(imageLower, strings.ToLower(imagePattern)) {
			if isVM {
				return common.WorkloadTypeVM
			}
			return common.WorkloadTypeContainer
		}
	}
	
	// Check for VM indicators in image name
	vmIndicators := []string{
		"vmx", "vsrx", "vqfx", "veos", "csr1000v", "iosv", "iosxr",
		"vyos", "pfsense", "opnsense", "routeros", "chr", "fortigate",
		"vm-", "-vm", "virtual", "qemu", "kvm",
	}
	
	for _, indicator := range vmIndicators {
		if strings.Contains(imageLower, indicator) {
			return common.WorkloadTypeVM
		}
	}
	
	// Check for container indicators
	containerIndicators := []string{
		"ceos", "srl", "srlinux", "sonic", "frr", "quagga",
		"alpine", "ubuntu", "centos", "debian", "busybox",
		"container", "docker", "k8s",
	}
	
	for _, indicator := range containerIndicators {
		if strings.Contains(imageLower, indicator) {
			return common.WorkloadTypeContainer
		}
	}
	
	return ""
}

// classifyByKind determines workload type based on the node kind
func (c *WorkloadClassifier) classifyByKind(kind string) common.WorkloadType {
	kindLower := strings.ToLower(kind)
	
	// VM-based kinds
	vmKinds := map[string]bool{
		"csr1000v":    true,
		"iosv":        true,
		"iosxr":       true,
		"nxos":        true,
		"veos":        true,
		"vmx":         true,
		"vsrx":        true,
		"vqfx":        true,
		"vyos":        true,
		"pfsense":     true,
		"opnsense":    true,
		"routeros":    true,
		"chr":         true,
		"fortigate":   true,
		"fortios":     true,
	}
	
	// Container-based kinds
	containerKinds := map[string]bool{
		"ceos":        true,
		"srl":         true,
		"srlinux":     true,
		"sonic":       true,
		"frr":         true,
		"quagga":      true,
		"linux":       true,
		"host":        true,
		"bridge":      true,
		"ovs":         true,
	}
	
	if vmKinds[kindLower] {
		return common.WorkloadTypeVM
	}
	
	if containerKinds[kindLower] {
		return common.WorkloadTypeContainer
	}
	
	return ""
}

// classifyByImageCharacteristics analyzes image properties to determine workload type
func (c *WorkloadClassifier) classifyByImageCharacteristics(image string) common.WorkloadType {
	imageLower := strings.ToLower(image)
	
	// Images that typically indicate VM workloads
	if strings.Contains(imageLower, "qcow2") ||
		strings.Contains(imageLower, "vmdk") ||
		strings.Contains(imageLower, "iso") ||
		strings.Contains(imageLower, "ova") ||
		strings.Contains(imageLower, "vhd") {
		return common.WorkloadTypeVM
	}
	
	// Images from registries known for VM images
	vmRegistries := []string{
		"registry.hub.docker.com/virtualization/",
		"quay.io/kubevirt/",
		"registry.redhat.io/ubi8/",
	}
	
	for _, registry := range vmRegistries {
		if strings.HasPrefix(imageLower, registry) {
			return common.WorkloadTypeVM
		}
	}
	
	// Standard container registries with container images
	containerRegistries := []string{
		"docker.io/",
		"ghcr.io/",
		"quay.io/",
		"gcr.io/",
		"registry.k8s.io/",
	}
	
	for _, registry := range containerRegistries {
		if strings.HasPrefix(imageLower, registry) {
			return common.WorkloadTypeContainer
		}
	}
	
	return ""
}

// ForceVM forces a specific node to run as a virtual machine
func (c *WorkloadClassifier) ForceVM(nodeName string) {
	c.forceVM[nodeName] = true
	delete(c.forceContainer, nodeName) // Remove any conflicting setting
	c.logger.Debugf("Node %s forced to VM workload type", nodeName)
}

// ForceContainer forces a specific node to run as a container
func (c *WorkloadClassifier) ForceContainer(nodeName string) {
	c.forceContainer[nodeName] = true
	delete(c.forceVM, nodeName) // Remove any conflicting setting
	c.logger.Debugf("Node %s forced to container workload type", nodeName)
}

// GetClassificationReasoning provides detailed reasoning for why a node was classified
func (c *WorkloadClassifier) GetClassificationReasoning(config *common.NodeConfig) string {
	reasoning := []string{}
	
	// Check explicit overrides
	if execMode, exists := config.Environment["EXECUTION_MODE"]; exists {
		reasoning = append(reasoning, fmt.Sprintf("EXECUTION_MODE environment variable set to %s", execMode))
	}
	
	if c.forceVM[config.Name] {
		reasoning = append(reasoning, "forced to VM by classifier configuration")
	}
	
	if c.forceContainer[config.Name] {
		reasoning = append(reasoning, "forced to container by classifier configuration")
	}
	
	// Check image-based classification
	imageLower := strings.ToLower(config.Image)
	for imagePattern, isVM := range c.vmImageMap {
		if strings.Contains(imageLower, strings.ToLower(imagePattern)) {
			if isVM {
				reasoning = append(reasoning, fmt.Sprintf("image contains VM pattern: %s", imagePattern))
			} else {
				reasoning = append(reasoning, fmt.Sprintf("image contains container pattern: %s", imagePattern))
			}
			break
		}
	}
	
	// Check kind-based classification
	workloadType := c.classifyByKind(config.Kind)
	if workloadType != "" {
		reasoning = append(reasoning, fmt.Sprintf("node kind %s indicates %s workload", config.Kind, workloadType))
	}
	
	if len(reasoning) == 0 {
		reasoning = append(reasoning, "no specific indicators found, defaulting to container")
	}
	
	return strings.Join(reasoning, "; ")
}

// GetSupportedVMKinds returns a list of node kinds that should run as VMs
func (c *WorkloadClassifier) GetSupportedVMKinds() []string {
	kinds := []string{}
	for imagePattern, isVM := range c.vmImageMap {
		if isVM {
			kinds = append(kinds, imagePattern)
		}
	}
	return kinds
}

// GetSupportedContainerKinds returns a list of node kinds that should run as containers
func (c *WorkloadClassifier) GetSupportedContainerKinds() []string {
	kinds := []string{}
	for imagePattern, isVM := range c.vmImageMap {
		if !isVM {
			kinds = append(kinds, imagePattern)
		}
	}
	return kinds
}