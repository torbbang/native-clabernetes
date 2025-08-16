# native-clabernetes (nc9s) - CLAUDE.md

## Project Overview

**native-clabernetes** (also known as nc9s) is a prototype fork of clabernetes that focuses on enhanced Kubernetes-native networking functionality. This is a heavily "vibe-coded" experimental fork that removes some upstream features while enhancing others.

⚠️ **PROTOTYPE WARNING**: This codebase is experimental and not recommended for production use.

## Core Concept

The project allows users to take existing containerlab topology files and deploy them as distributed network topologies across a Kubernetes cluster. Each containerlab node becomes a Kubernetes deployment, with sophisticated networking and connectivity management handled automatically.

## Repository Structure

```
/
├── apis/v1alpha1/          # Kubernetes API definitions (CRDs)
├── controllers/            # Kubernetes controller implementations
├── launcher/               # Pod launcher and node management
├── config/                 # Configuration management
├── clabverter/            # Tool to convert containerlab files to clabernetes resources
├── charts/                # Helm charts for deployment
├── manager/               # Main controller manager
└── examples/              # Example topology files
```

## Key Components

### 1. Custom Resource Definitions (CRDs)
- **Topology**: Main CR representing a containerlab network topology
- **Config**: Global configuration for the clabernetes controller
- **Connectivity**: Inter-node connectivity configuration
- **ImageRequest**: Image pull requests for launcher pods

**Note**: KNE support has been completely removed from this fork.

### 2. Controllers
- **Topology Controller** (`controllers/topology/`): Main reconciliation logic
- **ImageRequest Controller** (`controllers/imagerequest/`): Handles image pulling

### 3. Launcher System
- **Launcher** (`launcher/`): Runs inside each node pod
- Handles containerlab execution, Docker-in-Docker setup, and connectivity
- Manages VXLAN tunnels for inter-node connectivity

### 4. Architecture Approach
- **One Deployment per Node**: Each containerlab node = one Kubernetes Deployment
- **Docker-in-Docker**: Independent Docker daemon in each launcher pod (not CRI dependent)
- **VXLAN Connectivity**: Inter-node networking via VXLAN tunnels
- **LoadBalancer Services**: Node exposure for SSH/NETCONF access

## Major Changes from Upstream Clabernetes

This fork has made several significant changes from the upstream clabernetes project:

### Removed Features
- **KNE Support**: Completely removed Kubernetes Network Emulation support to focus purely on containerlab workflows
- **Slurpeeth Connectivity**: Removed broken slurpeeth connectivity references

### Enhanced Features

### Node Selectors & Scheduling
- Recent addition of `NodeSelectorsByImage` functionality
- Allows targeting specific Kubernetes nodes based on container image patterns
- Configuration in `config/nodeselectors.go` and related test files
- Supports pattern matching (glob patterns) for image-to-node mapping

### Native Kubernetes Features Integration
- Advanced node scheduling with NodeSelectors, tolerations, and affinity
- PersistentVolumeClaim support for node data persistence
- ConfigMap-based file mounting (replacing local file dependencies)
- ServiceAccount and RBAC integration

## Development Workflow

### Essential Commands
```bash
# Formatting and linting
make fmt                    # Run code formatters
make lint                   # Run linters (Go + Helm)

# Testing
make test                   # Unit tests
make test-race             # Unit tests with race detection
make test-e2e              # End-to-end tests

# Code generation
make run-generate          # Generate all CRDs, clients, deepcopy code

# Building
make build-manager         # Build manager container
make build-launcher        # Build launcher container
make build-clabverter      # Build clabverter tool
```

### Key Files to Understand

1. **`apis/v1alpha1/topology.go`**: Core Topology CRD definition
2. **`controllers/topology/controller.go`**: Main topology reconciliation
3. **`launcher/clabernetes.go`**: Launcher pod main logic
4. **`config/manager.go`**: Configuration management interface
5. **`charts/clabernetes/values.yaml`**: Helm chart configuration

## Configuration Management

The system uses a sophisticated configuration hierarchy:
- **Global Config CR**: Cluster-wide settings
- **Topology Spec**: Per-topology overrides
- **Bootstrap ConfigMaps**: Initial configuration (Helm-managed)

Key configuration areas:
- Resource requests/limits (by containerlab kind/type)
- Node selectors (by image patterns)
- Image pull configuration (CRI integration)
- Launcher pod settings (privileged mode, debugging)

## Testing & Development

### Test Structure
- Unit tests alongside source code (`*_test.go`)
- Golden file testing pattern (`test-fixtures/golden/`)
- E2E tests in `e2e/` directory
- Helm chart testing in `charts/*/tests/`

### Common Testing Patterns
- Mock managers for configuration testing
- Golden file comparisons for rendered resources
- Integration tests with real Kubernetes clusters

## Tools & Dependencies

### Core Dependencies
- **controller-runtime**: Kubernetes controller framework
- **containerlab**: Network topology simulation

**Note**: KNE dependencies have been removed from this fork.

### Development Tools
- **gofumpt, gci, golines**: Code formatting
- **golangci-lint**: Static analysis
- **gotestsum**: Test execution
- **helm**: Chart management

## Image and Container Strategy

### Image Pull Through Mode
- Support for CRI socket mounting (containerd primary)
- Fallback to launcher pod Docker daemon
- Configurable via `imagePullThroughMode` (auto/always/never)

### Node Selector Strategy
- Pattern-based image-to-node mapping
- Longest pattern match wins
- Default fallback support
- Integration with Kubernetes scheduling

## Notable Features

### Connectivity Options
- **VXLAN tunnels**: Inter-node connectivity (slurpeeth support removed)

### Exposure Methods
- LoadBalancer services with optional management IP assignment
- ClusterIP with status tracking
- NodePort support

### File Management
- ConfigMap-based file mounting (via clabverter)
- Startup configuration support
- License file handling

## Development Status & Focus

This is a **prototype fork** with the following characteristics:

### Current State
- **Heavily "vibe-coded"**: Implementation prioritizes functionality over code quality
- **Experimental**: Not recommended for production use
- **Breaking changes**: Removes upstream features (KNE) to focus on core containerlab functionality

### Development Priorities
1. **Enhanced node scheduling capabilities** with image-based node selectors
2. **Deeper Kubernetes integration** with native resource management  
3. **Streamlined codebase** by removing unused features (KNE, slurpeeth)
4. **Better CRI integration** for image management
5. **Improved test coverage** with comprehensive golden file testing

## Debugging & Troubleshooting

### Common Debug Points
- Launcher pod logs for node-level issues
- Controller manager logs for reconciliation problems
- Service status for connectivity verification
- ConfigMap content for file mounting issues

### Key Environment Variables
- `LAUNCHER_*`: Launcher pod configuration
- Various debug and log level settings
- CRI and image pull configuration

## Recent Changes Made

### KNE Removal (Complete)
- Removed `util/kne/` package entirely
- Removed `controllers/topology/definitionkne.go`
- Updated `NewDefinitionProcessor()` to only handle containerlab
- Removed KNE test cases and golden files
- Updated CRDs to remove KNE enum values
- Cleaned up Go module dependencies

### Bug Fixes
- Fixed malformed JSON in deployment test golden files
- Removed broken slurpeeth connectivity references
- Fixed service fabric test mismatches
- Updated all test golden files to match actual output

### Test Improvements
- All deployment and service fabric tests now pass
- Comprehensive golden file testing pattern maintained
- Removed outdated KNE test references

This CLAUDE.md provides context for understanding the native-clabernetes fork, its architecture, and development direction as a prototype focused on containerlab-only functionality with enhanced Kubernetes integration.