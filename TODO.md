# Clabernetes Implementation TODO

This document outlines the detailed implementation plan for migrating clabernetes from Docker-in-Docker architecture to native Kubernetes execution with Cilium CNI and KubeVirt support.

## 🎯 Project Overview

**Goal**: Eliminate Docker-in-Docker complexity by implementing native Kubernetes container execution with advanced networking via Cilium CNI and VM support through KubeVirt.

**Expected Benefits**:
- 50%+ reduction in pod startup time
- 30%+ reduction in CPU/memory overhead  
- Elimination of privileged container requirements
- Support for VM-based network appliances
- Enhanced security and compliance

---

## Phase 1: Foundation and Architecture (Weeks 1-2)

### ✅ Completed
- [x] Create feature branch `feat/native-architecture`
- [x] Set up kind cluster configuration with Cilium CNI
- [x] Configure KubeVirt operator installation
- [x] Update devspace configuration for new architecture
- [x] Create GitHub workflow for CI/CD testing
- [x] Write development documentation

### 🔄 In Progress
- [ ] Design new package structure
- [ ] Extend CRD APIs for native execution
- [ ] Update constants and configuration

### 📋 Pending Tasks

#### 1.1 New Package Structure
- [ ] Create `pkg/executor/` package
  - [ ] `pkg/executor/container/` - Native container execution
  - [ ] `pkg/executor/vm/` - KubeVirt VM execution
  - [ ] `pkg/executor/common/` - Shared execution logic
- [ ] Create `pkg/networking/` package
  - [ ] `pkg/networking/cilium/` - Cilium-specific networking
  - [ ] `pkg/networking/policies/` - NetworkPolicy generation
  - [ ] `pkg/networking/connectivity/` - Inter-node connectivity
- [ ] Create `pkg/workload/` package
  - [ ] `pkg/workload/detector/` - Container vs VM detection
  - [ ] `pkg/workload/renderer/` - K8s resource generation
  - [ ] `pkg/workload/reconciler/` - Workload state reconciliation

#### 1.2 API Extensions
- [ ] Extend `TopologySpec` with execution mode
  ```go
  type TopologySpec struct {
      ExecutionMode string `json:"executionMode,omitempty"` // "container" | "vm" | "auto"
      Networking NetworkingSpec `json:"networking,omitempty"`
  }
  ```
- [ ] Add `NetworkingSpec` for Cilium configuration
- [ ] Update CRD definitions and generated code

#### 1.3 Configuration Updates
- [ ] Remove Docker-related constants from `constants/env.go`
- [ ] Add Cilium and KubeVirt configuration options
- [ ] Update Helm charts for new architecture
- [ ] Create configuration validation

---

## Phase 2: Native Container Execution (Weeks 3-5)

### 📋 Tasks

#### 2.1 Replace Launcher Architecture
- [ ] Delete `launcher/docker.go` entirely
- [ ] Replace Docker startup logic in `launcher/clabernetes.go:169-185`
- [ ] Remove containerlab execution from `launcher/containerlab.go:107-145`
- [ ] Create new `ContainerManager` in `pkg/executor/container/`

#### 2.2 Implement Native Executor
- [ ] Design `ContainerManager` interface
  ```go
  type ContainerManager struct {
      kubeClient    kubernetes.Interface
      namespace     string
      topologyName  string
      logger        logging.Instance
  }
  ```
- [ ] Implement `ExecuteNode()` method for direct pod creation
- [ ] Handle container configuration without Docker
- [ ] Implement container lifecycle management

#### 2.3 Update Controllers
- [ ] Modify `controllers/topology/deployment.go`
- [ ] Replace launcher pod rendering with direct node containers
- [ ] Update deployment reconciliation logic
- [ ] Remove Docker-in-Docker container specifications

#### 2.4 Configuration Management
- [ ] Replace `topo.clab.yaml` with native Kubernetes configs
- [ ] Convert containerlab configs to container environment variables
- [ ] Remove Docker daemon configuration templates
- [ ] Update ConfigMap generation logic

#### 2.5 Testing
- [ ] Unit tests for `ContainerManager`
- [ ] Integration tests for native container execution
- [ ] Performance benchmarks vs Docker-in-Docker
- [ ] Compatibility tests with existing topologies

---

## Phase 3: Cilium CNI Integration (Weeks 4-7)

### 📋 Tasks

#### 3.1 Replace VXLAN Connectivity
- [ ] Delete `launcher/connectivity/vxlan.go`
- [ ] Remove `launcher/connectivity/slurpeeth.go`
- [ ] Replace `launcher/connectivity.go` entirely
- [ ] Analyze existing tunnel allocation logic for migration

#### 3.2 Implement Cilium Networking
- [ ] Create `CiliumManager` in `pkg/networking/cilium/`
  ```go
  type CiliumManager struct {
      ciliumClient ciliumv2.Interface
      k8sClient    kubernetes.Interface
  }
  ```
- [ ] Implement `CreateNetworkConnectivity()` method
- [ ] Replace VXLAN tunnels with CiliumNetworkPolicies
- [ ] Handle Cilium-specific configuration

#### 3.3 Network Policy Generation
- [ ] Create `PolicyGenerator` in `pkg/networking/policies/`
- [ ] Generate NetworkPolicies from topology links
- [ ] Implement policy reconciliation logic
- [ ] Handle policy updates and deletions

#### 3.4 Service Discovery Updates
- [ ] Update `controllers/topology/service.go` for Cilium
- [ ] Implement ClusterIP/headless services for topology links
- [ ] Replace IP-based connectivity with service-based
- [ ] Handle service mesh integration

#### 3.5 Testing
- [ ] Unit tests for Cilium integration
- [ ] Network policy functionality tests
- [ ] Connectivity tests between nodes
- [ ] Performance comparison with VXLAN
- [ ] Hubble integration for debugging

---

## Phase 4: KubeVirt Integration (Weeks 6-9)

### 📋 Tasks

#### 4.1 Workload Type Detection
- [ ] Create `WorkloadClassifier` in `pkg/workload/detector/`
- [ ] Implement node type detection logic
  ```go
  func (c *WorkloadClassifier) DetermineWorkloadType(nodeConfig *Config) WorkloadType
  ```
- [ ] Define VM image patterns for routers/firewalls
- [ ] Handle user-specified execution preferences

#### 4.2 KubeVirt Resource Generation
- [ ] Create `VirtualMachineReconciler` in `controllers/topology/`
- [ ] Implement VM rendering logic
- [ ] Handle VM lifecycle management
- [ ] Integrate with existing topology controller

#### 4.3 VM Networking Configuration
- [ ] Generate VirtIO network interfaces
- [ ] Handle management and data interfaces
- [ ] Implement network bridge configuration
- [ ] Integrate with Cilium networking

#### 4.4 VM Image Management
- [ ] Handle ContainerDisk images
- [ ] Implement cloud-init configuration
- [ ] Support custom VM images
- [ ] Handle image pull policies

#### 4.5 Testing
- [ ] Unit tests for VM functionality
- [ ] Integration tests with KubeVirt
- [ ] VM networking tests
- [ ] Performance testing for VMs
- [ ] Mixed container/VM topology tests

---

## Phase 5: Testing and Migration (Weeks 8-12)

### 📋 Tasks

#### 5.1 Backward Compatibility
- [ ] Implement feature flag system
  ```go
  type TopologySpec struct {
      ExecutionMode string `json:"executionMode,omitempty"` // "legacy" | "native" | "hybrid"
  }
  ```
- [ ] Support legacy execution mode
- [ ] Implement hybrid mode for gradual migration
- [ ] Ensure existing topologies continue working

#### 5.2 Migration Tools
- [ ] Create `clabernetes-migrate` utility
- [ ] Implement topology backup functionality
- [ ] Create migration validation
- [ ] Handle rollback scenarios

#### 5.3 Comprehensive Testing
- [ ] E2E tests for native architecture
  - [ ] `e2e/native/container_execution_test.go`
  - [ ] `e2e/native/cilium_networking_test.go`
  - [ ] `e2e/native/kubevirt_vm_test.go`
  - [ ] `e2e/native/hybrid_topology_test.go`
  - [ ] `e2e/native/migration_test.go`

#### 5.4 Performance Benchmarking
- [ ] Startup time comparisons
- [ ] Resource utilization analysis
- [ ] Network latency measurements
- [ ] Throughput benchmarks
- [ ] Scalability testing

#### 5.5 Documentation and Training
- [ ] Update README.md
- [ ] Create migration guide
- [ ] Update API documentation
- [ ] Create troubleshooting guide
- [ ] Record demo videos

---

## 🎯 Success Criteria

### Performance Metrics
- [ ] **Startup Time**: 50%+ reduction in pod startup time
- [ ] **Resource Usage**: 30%+ reduction in CPU/memory overhead
- [ ] **Network Performance**: Latency equivalent or better than VXLAN
- [ ] **Scalability**: Support for larger topologies

### Functionality Metrics
- [ ] **Security**: Elimination of privileged container requirements
- [ ] **Compatibility**: 100% backward compatibility with existing topologies
- [ ] **VM Support**: Functional VM-based network appliances
- [ ] **Networking**: Full NetworkPolicy support with Cilium

### Quality Metrics
- [ ] **Test Coverage**: 90%+ code coverage for new components
- [ ] **Documentation**: Complete API and user documentation
- [ ] **CI/CD**: All tests passing in automated pipeline
- [ ] **Migration**: Smooth migration path from legacy architecture

---

## 🚨 Risk Mitigation

### High-Risk Items
- [ ] **Docker Dependency Removal**: Plan for gradual removal with feature flags
- [ ] **VXLAN to Cilium Migration**: Ensure network connectivity during transition
- [ ] **KubeVirt Stability**: Test thoroughly in CI environment
- [ ] **Performance Regression**: Continuous benchmarking throughout development

### Mitigation Strategies
- [ ] **Parallel Development**: Work on phases 2-4 simultaneously where possible
- [ ] **Feature Flags**: Enable gradual rollout and easy rollback
- [ ] **Comprehensive Testing**: Test at each phase boundary
- [ ] **User Communication**: Regular updates on breaking changes

---

## 📅 Timeline Summary

| Phase | Duration | Key Deliverables | Dependencies |
|-------|----------|------------------|--------------|
| **Phase 1** | Weeks 1-2 | ✅ Foundation, API design, dev environment | None |
| **Phase 2** | Weeks 3-5 | Native container execution | Phase 1 |
| **Phase 3** | Weeks 4-7 | Cilium CNI integration | Phase 1, partial Phase 2 |
| **Phase 4** | Weeks 6-9 | KubeVirt VM support | Phase 1, partial Phase 2 |
| **Phase 5** | Weeks 8-12 | Testing, migration, documentation | All phases |

**Total Duration**: 12 weeks with overlapping development phases

---

## 🔗 Related Issues and PRs

- [ ] Create GitHub issues for each major component
- [ ] Link to relevant containerlab issues
- [ ] Track dependencies on upstream projects (Cilium, KubeVirt)
- [ ] Document breaking changes and migration requirements

---

## 📝 Notes

- This plan prioritizes backward compatibility to ensure smooth adoption
- Feature flags enable gradual migration and reduce deployment risk
- Comprehensive testing ensures quality and performance improvements
- Documentation and migration tools support user adoption

**Next Steps**: Begin Phase 1 implementation starting with package structure and API design.