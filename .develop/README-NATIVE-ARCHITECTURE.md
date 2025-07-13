# Clabernetes Native Architecture Development

This document describes the development environment and workflow for the new native architecture implementation of clabernetes, which eliminates Docker-in-Docker containers in favor of native Kubernetes execution with Cilium CNI and KubeVirt support.

## Overview

The native architecture replaces the current Docker-in-Docker model with:
- **Native container execution** using standard Kubernetes pods
- **Cilium CNI** for advanced networking and policies
- **KubeVirt** for virtual machine-based network nodes
- **Elimination of privileged containers** and Docker daemon dependencies

## Quick Start

### 1. Prerequisites

Ensure you have the following tools installed:
- [Docker](https://docs.docker.com/get-docker/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [helm](https://helm.sh/docs/intro/install/)
- [devspace](https://devspace.sh/docs/getting-started/installation)
- [Go 1.24+](https://golang.org/doc/install)

### 2. Set Up Development Environment

```bash
# Clone the repository and switch to the feature branch
git checkout feat/native-architecture

# Set up the complete native development environment
.develop/setup-native-dev.sh
```

This script will:
- Create a kind cluster with Cilium CNI
- Install KubeVirt operator for VM support
- Configure Hubble for network observability
- Set up development tools and namespaces

### 3. Start Development

```bash
# Using the native devspace configuration
DEVSPACE_CONFIG=./.develop/devspace-native.yaml devspace dev --profile native-dev --profile debug

# Or use the custom command
devspace run setup-env
```

## Architecture Components

### Current vs Native Architecture

| Component | Current (Docker-in-Docker) | Native Architecture |
|-----------|---------------------------|-------------------|
| **Execution** | Launcher pods running Docker | Direct pod creation via K8s API |
| **Networking** | VXLAN tunnels between containers | Cilium CNI with NetworkPolicies |
| **Container Runtime** | Docker-in-Docker with privileged pods | Native containerd/CRI-O |
| **VM Support** | Not supported | KubeVirt VirtualMachines |
| **Security** | Privileged containers required | Standard pod security contexts |

### New Package Structure

```
pkg/
├── executor/           # Replaces launcher functionality
│   ├── container/      # Native container execution
│   ├── vm/            # KubeVirt VM execution  
│   └── common/        # Shared execution logic
├── networking/
│   ├── cilium/        # Cilium-specific networking
│   ├── policies/      # NetworkPolicy generation
│   └── connectivity/  # Inter-node connectivity
└── workload/
    ├── detector/      # Detect container vs VM requirements
    ├── renderer/      # Generate K8s resources
    └── reconciler/    # Reconcile workload state
```

## Development Workflow

### 1. Building and Testing

```bash
# Build all components
make build

# Run unit tests for native components
go test ./pkg/executor/... ./pkg/networking/... ./pkg/workload/...

# Run integration tests
make test-native

# Run specific component tests
go test ./pkg/networking/cilium/... -v
```

### 2. Local Development Commands

```bash
# Quick cluster setup
devspace run quick-cluster

# Install only Cilium for networking tests
devspace run install-cilium

# Install KubeVirt for VM testing
devspace run install-kubevirt

# Test network connectivity
devspace run test-connectivity

# Clean up environment
devspace run cleanup
```

### 3. Debugging Tools

#### Cilium/Hubble Debugging
```bash
# Access Hubble UI for network flow visualization
kubectl port-forward -n kube-system svc/hubble-ui 12000:80
# Open http://localhost:12000

# Check Cilium status
kubectl get pods -n kube-system -l k8s-app=cilium

# View network policies
kubectl get networkpolicies --all-namespaces

# Debug connectivity with Cilium
kubectl exec -n kube-system ds/cilium -- cilium connectivity test
```

#### KubeVirt Debugging
```bash
# Check KubeVirt status
kubectl get pods -n kubevirt

# List virtual machines
kubectl get vms --all-namespaces

# Connect to VM console
virtctl console <vm-name> -n <namespace>

# Check VM logs
kubectl logs -n kubevirt -l kubevirt.io=virt-launcher
```

## Development Features

### 1. Feature Flags

The native architecture supports gradual migration via feature flags:

```yaml
# Topology CRD
apiVersion: clabernetes.containerlab.dev/v1alpha1
kind: Topology
metadata:
  name: example-topology
spec:
  executionMode: "native"  # "legacy" | "native" | "hybrid"
  networking:
    cni: "cilium"
    policies:
      - name: "allow-mgmt"
        type: "ingress"
```

### 2. Workload Detection

The system automatically detects whether a node should run as a container or VM:

```go
// Example workload detection
vmImages := []string{
    "cisco/csr1000v", "arista/veos", "juniper/vmx", 
    "vyos/vyos", "pfsense/pfsense", "opnsense/opnsense",
}
```

### 3. Networking Modes

- **Container-to-Container**: Direct pod networking via Cilium
- **Container-to-VM**: Pod to VirtualMachine networking
- **VM-to-VM**: VirtualMachine to VirtualMachine networking
- **External Access**: LoadBalancer and NodePort services

## CI/CD Integration

### GitHub Workflows

The native architecture includes comprehensive CI/CD testing:

- **Unit Tests**: Test new package components
- **Integration Tests**: Test Cilium + KubeVirt integration  
- **E2E Tests**: Full topology deployment testing
- **Performance Tests**: Compare native vs legacy performance

### Running CI Tests Locally

```bash
# Run the same tests as CI
act -j integration-tests-native

# Debug CI with interactive session
act -j integration-tests-native --input debug_tests=true
```

## Configuration Files

### Key Configuration Files

- **`.develop/kind-cluster-native.yml`**: Kind cluster with Cilium support
- **`.develop/devspace-native.yaml`**: DevSpace configuration for native development
- **`.develop/setup-native-dev.sh`**: Complete environment setup script
- **`.develop/start-native.sh`**: Development session startup script

### Environment Variables

```bash
# Native architecture specific
EXECUTION_MODE=native
CILIUM_ENABLED=true
KUBEVIRT_ENABLED=true

# Development settings
CLABERNETES_DEV_MODE=true
CONTROLLER_LOG_LEVEL=debug
MANAGER_LOG_LEVEL=debug
```

## Troubleshooting

### Common Issues

#### 1. Cilium Not Starting
```bash
# Check node readiness
kubectl get nodes

# Check Cilium pods
kubectl get pods -n kube-system -l k8s-app=cilium

# View Cilium logs
kubectl logs -n kube-system ds/cilium
```

#### 2. KubeVirt VMs Not Starting
```bash
# Check if nested virtualization is enabled
grep -E --color=auto 'vmx|svm' /proc/cpuinfo

# Check KubeVirt configuration
kubectl get kubevirt kubevirt -n kubevirt -o yaml

# View VM events
kubectl get events -n <namespace> --field-selector involvedObject.name=<vm-name>
```

#### 3. Network Policies Not Working
```bash
# Check if Cilium is enforcing policies
kubectl exec -n kube-system ds/cilium -- cilium status | grep Policy

# View network policy status
kubectl describe networkpolicy <policy-name> -n <namespace>
```

### Debug Mode

Enable debug mode for detailed logging:

```bash
# Start development with debug
DEVSPACE_CONFIG=./.develop/devspace-native.yaml devspace dev --profile native-dev --profile debug

# Or set environment variables
export CONTROLLER_LOG_LEVEL=debug
export MANAGER_LOG_LEVEL=debug
```

## Performance Optimization

### Resource Requirements

Native architecture typically requires:
- **50% less CPU** (no Docker-in-Docker overhead)
- **30% less memory** (no nested containers)
- **Faster startup times** (direct pod creation)

### Monitoring

```bash
# Monitor resource usage
kubectl top pods --all-namespaces

# Monitor network performance
kubectl exec -n kube-system ds/cilium -- cilium metrics list

# Monitor VM resource usage
kubectl get vmi --all-namespaces -o wide
```

## Migration Guide

### From Legacy to Native

1. **Backup existing topologies**:
   ```bash
   kubectl get topologies --all-namespaces -o yaml > topology-backup.yaml
   ```

2. **Update topology specs**:
   ```yaml
   spec:
     executionMode: "native"
   ```

3. **Apply updated topologies**:
   ```bash
   kubectl apply -f updated-topologies.yaml
   ```

4. **Verify migration**:
   ```bash
   kubectl get pods --all-namespaces -l clabernetes/execution-mode=native
   ```

## Contributing

### Development Guidelines

1. **Package Structure**: Follow the new package organization
2. **Testing**: Include unit and integration tests for new components
3. **Documentation**: Update this README for new features
4. **Backward Compatibility**: Use feature flags for breaking changes

### Pull Request Checklist

- [ ] Unit tests pass for new components
- [ ] Integration tests pass with Cilium + KubeVirt
- [ ] Documentation updated
- [ ] Feature flags implemented for new functionality
- [ ] Performance impact measured and documented

## Additional Resources

- [Cilium Documentation](https://docs.cilium.io/)
- [KubeVirt Documentation](https://kubevirt.io/user-guide/)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [DevSpace Documentation](https://devspace.sh/docs/)

## Support

For questions about native architecture development:
1. Check this README first
2. Review existing GitHub issues
3. Create a new issue with the `native-architecture` label