package detector

import (
	"testing"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
)

func TestWorkloadClassifier_DetermineWorkloadType(t *testing.T) {
	// Create a fake logger for testing
	logger := &fakeLogger{}
	
	classifier := NewWorkloadClassifier(logger)
	
	tests := []struct {
		name           string
		config         *common.NodeConfig
		expectedType   common.WorkloadType
		description    string
	}{
		{
			name: "Cisco CSR1000v should be VM",
			config: &common.NodeConfig{
				Name:  "router1",
				Image: "cisco/csr1000v:latest",
				Kind:  "csr1000v",
			},
			expectedType: common.WorkloadTypeVM,
			description:  "CSR1000v is a VM-based router",
		},
		{
			name: "Arista cEOS should be container",
			config: &common.NodeConfig{
				Name:  "switch1",
				Image: "ceos:latest",
				Kind:  "ceos",
			},
			expectedType: common.WorkloadTypeContainer,
			description:  "cEOS is a container-based switch",
		},
		{
			name: "Nokia SRL should be container",
			config: &common.NodeConfig{
				Name:  "leaf1",
				Image: "nokia/srl:latest",
				Kind:  "srl",
			},
			expectedType: common.WorkloadTypeContainer,
			description:  "SR Linux is container-based",
		},
		{
			name: "VyOS should be VM",
			config: &common.NodeConfig{
				Name:  "fw1",
				Image: "vyos/vyos:1.4",
				Kind:  "vyos",
			},
			expectedType: common.WorkloadTypeVM,
			description:  "VyOS typically runs as VM",
		},
		{
			name: "pfSense should be VM",
			config: &common.NodeConfig{
				Name:  "firewall1",
				Image: "pfsense/pfsense:latest",
				Kind:  "pfsense",
			},
			expectedType: common.WorkloadTypeVM,
			description:  "pfSense is VM-based firewall",
		},
		{
			name: "FRR should be container",
			config: &common.NodeConfig{
				Name:  "bgp1",
				Image: "frr/frr:latest",
				Kind:  "frr",
			},
			expectedType: common.WorkloadTypeContainer,
			description:  "FRR is container-based routing",
		},
		{
			name: "Unknown image defaults to container",
			config: &common.NodeConfig{
				Name:  "unknown1",
				Image: "unknown/router:latest",
				Kind:  "unknown",
			},
			expectedType: common.WorkloadTypeContainer,
			description:  "Unknown types default to container",
		},
		{
			name: "Environment variable override to VM",
			config: &common.NodeConfig{
				Name:  "override1",
				Image: "ceos:latest",
				Kind:  "ceos",
				Environment: map[string]string{
					"EXECUTION_MODE": "vm",
				},
			},
			expectedType: common.WorkloadTypeVM,
			description:  "Environment variable should override classification",
		},
		{
			name: "Environment variable override to container",
			config: &common.NodeConfig{
				Name:  "override2",
				Image: "vyos/vyos:latest",
				Kind:  "vyos",
				Environment: map[string]string{
					"EXECUTION_MODE": "container",
				},
			},
			expectedType: common.WorkloadTypeContainer,
			description:  "Environment variable should override VM classification",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.DetermineWorkloadType(tt.config)
			
			if result != tt.expectedType {
				t.Errorf("DetermineWorkloadType() = %v, want %v\nDescription: %s\nReasoning: %s",
					result, tt.expectedType, tt.description,
					classifier.GetClassificationReasoning(tt.config))
			}
		})
	}
}

func TestWorkloadClassifier_ForceOverrides(t *testing.T) {
	logger := &fakeLogger{}
	classifier := NewWorkloadClassifier(logger)
	
	config := &common.NodeConfig{
		Name:  "test1",
		Image: "ceos:latest",
		Kind:  "ceos",
	}
	
	// Test normal classification first
	result := classifier.DetermineWorkloadType(config)
	if result != common.WorkloadTypeContainer {
		t.Errorf("Expected container for cEOS, got %v", result)
	}
	
	// Force to VM
	classifier.ForceVM(config.Name)
	result = classifier.DetermineWorkloadType(config)
	if result != common.WorkloadTypeVM {
		t.Errorf("Expected VM after ForceVM, got %v", result)
	}
	
	// Force back to container
	classifier.ForceContainer(config.Name)
	result = classifier.DetermineWorkloadType(config)
	if result != common.WorkloadTypeContainer {
		t.Errorf("Expected container after ForceContainer, got %v", result)
	}
}

func TestWorkloadClassifier_GetSupportedKinds(t *testing.T) {
	logger := &fakeLogger{}
	classifier := NewWorkloadClassifier(logger)
	
	vmKinds := classifier.GetSupportedVMKinds()
	containerKinds := classifier.GetSupportedContainerKinds()
	
	// Check that we have some VM kinds
	if len(vmKinds) == 0 {
		t.Error("Expected some VM kinds to be supported")
	}
	
	// Check that we have some container kinds
	if len(containerKinds) == 0 {
		t.Error("Expected some container kinds to be supported")
	}
	
	// Check for specific known VM kinds
	vmKindsMap := make(map[string]bool)
	for _, kind := range vmKinds {
		vmKindsMap[kind] = true
	}
	
	expectedVMKinds := []string{"cisco/csr1000v", "vyos/vyos", "pfsense/pfsense"}
	for _, expectedKind := range expectedVMKinds {
		if !vmKindsMap[expectedKind] {
			t.Errorf("Expected VM kind %s not found in supported VM kinds", expectedKind)
		}
	}
	
	// Check for specific known container kinds
	containerKindsMap := make(map[string]bool)
	for _, kind := range containerKinds {
		containerKindsMap[kind] = true
	}
	
	expectedContainerKinds := []string{"nokia/srl", "arista/ceos", "frr/frr"}
	for _, expectedKind := range expectedContainerKinds {
		if !containerKindsMap[expectedKind] {
			t.Errorf("Expected container kind %s not found in supported container kinds", expectedKind)
		}
	}
}

func TestWorkloadClassifier_GetClassificationReasoning(t *testing.T) {
	logger := &fakeLogger{}
	classifier := NewWorkloadClassifier(logger)
	
	config := &common.NodeConfig{
		Name:  "test1",
		Image: "cisco/csr1000v:latest",
		Kind:  "csr1000v",
	}
	
	reasoning := classifier.GetClassificationReasoning(config)
	
	if reasoning == "" {
		t.Error("Expected reasoning to be provided")
	}
	
	// Should mention the image pattern match
	if !contains(reasoning, "cisco/csr1000v") {
		t.Errorf("Expected reasoning to mention image pattern, got: %s", reasoning)
	}
}

// fakeLogger implements a simple logger for testing
type fakeLogger struct{}

func (f *fakeLogger) Debug(msg string)                        {}
func (f *fakeLogger) Debugf(format string, args ...interface{}) {}
func (f *fakeLogger) Info(msg string)                         {}
func (f *fakeLogger) Infof(format string, args ...interface{})  {}
func (f *fakeLogger) Warn(msg string)                         {}
func (f *fakeLogger) Warnf(format string, args ...interface{})  {}
func (f *fakeLogger) Error(msg string)                        {}
func (f *fakeLogger) Errorf(format string, args ...interface{}) {}
func (f *fakeLogger) Critical(msg string)                     {}
func (f *fakeLogger) Criticalf(format string, args ...interface{}) {}
func (f *fakeLogger) Fatal(msg string)                        {}
func (f *fakeLogger) Fatalf(format string, args ...interface{})    {}
func (f *fakeLogger) Write(p []byte) (n int, err error)       { return len(p), nil }
func (f *fakeLogger) GetLevel() string                        { return "debug" }
func (f *fakeLogger) GetName() string                         { return "fake" }

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}