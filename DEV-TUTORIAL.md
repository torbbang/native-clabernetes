# Native-Clabernetes Development Environment Tutorial

This tutorial walks you through setting up and using the native-clabernetes development environment for contributing to the native Kubernetes execution architecture.

## 🚀 Quick Start

### Prerequisites

Before starting, ensure you have these tools installed:

```bash
# Required tools
- Docker (running)
- kubectl
- kind (Kubernetes in Docker)
- helm
- git
- Go 1.21+

# Optional but recommended
- DevSpace (for enhanced development workflow)
- virtctl (for KubeVirt VM management)
- cilium CLI (for network debugging)
```

### 1. Clone and Setup

```bash
# Clone the repository
git clone https://github.com/torbbang/native-clabernetes.git
cd native-clabernetes

# Run the automated setup (this takes 5-10 minutes)
./.develop/setup-native-dev.sh
```

The setup script will:
- ✅ Create a kind cluster with Cilium CNI
- ✅ Install KubeVirt for VM support
- ✅ Install cert-manager and other dependencies
- ✅ Validate the environment
- ✅ Provide connection details

### 2. Verify Installation

```bash
# Validate the entire setup
./.develop/validate-native-setup.sh

# Quick cluster check
kubectl get nodes
kubectl get pods -A
```

You should see:
- 3 nodes (1 control-plane, 2 workers)
- Cilium pods running
- KubeVirt operator running
- All pods in Running state

## 🛠️ Development Workflow

### Option A: Standard Development (Recommended for beginners)

```bash
# 1. Build the project
make build-manager

# 2. Run tests
make test

# 3. Run linting
make lint

# 4. Format code
make fmt
```

### Option B: DevSpace Development (Advanced)

```bash
# 1. Install DevSpace (if not already installed)
curl -s -L "https://github.com/loft-sh/devspace/releases/latest" | sed -nE 's!.*"([^"]*devspace-linux-amd64)".*!https://github.com\1!p' | xargs -r curl -o devspace -L && chmod +x devspace && sudo mv devspace /usr/local/bin

# 2. Start development environment
DEVSPACE_CONFIG=./.develop/devspace-native.yaml devspace dev --profile native-dev

# 3. This opens an interactive development session with:
#    - Hot reload for code changes
#    - Port forwarding
#    - Log streaming
#    - Direct cluster access
```

## 📁 Key Development Files

### Essential Configuration Files

```
.develop/
├── setup-native-dev.sh           # Complete environment setup
├── validate-native-setup.sh      # Environment validation
├── start-native.sh              # Quick cluster start
├── kind-cluster-native.yml      # Kind cluster configuration
├── devspace-native.yaml         # DevSpace configuration
└── README-NATIVE-ARCHITECTURE.md # Detailed architecture docs
```

### Important Source Code Locations

```
pkg/
├── executor/           # 🔥 Core execution logic (containers & VMs)
│   ├── common/        # Shared interfaces and types
│   ├── container/     # Native container execution
│   └── vm/           # KubeVirt VM execution
├── networking/
│   └── cilium/       # 🔥 Cilium CNI integration
└── workload/          # 🔥 Workload classification and rendering
    ├── detector/      # Container vs VM detection
    ├── renderer/      # Kubernetes resource generation
    └── reconciler/    # Workload lifecycle management

apis/v1alpha1/
└── topologyspec.go   # 🔥 Extended API definitions

examples/
└── native-architecture-demo.yaml # Example topology
```

## 🧪 Testing Your Changes

### 1. Unit Tests

```bash
# Run all tests
make test

# Test specific packages
go test ./pkg/executor/...
go test ./pkg/workload/...
go test ./pkg/networking/...
```

### 2. Integration Testing

```bash
# Apply example topology
kubectl apply -f examples/native-architecture-demo.yaml

# Watch topology deployment
kubectl get topology -w
kubectl get pods -w

# Check logs
kubectl logs -l clabernetes/topology=native-demo
```

### 3. Debug Common Issues

```bash
# Check cluster status
kubectl cluster-info
kubectl get nodes

# Check Cilium status
kubectl get pods -n kube-system -l k8s-app=cilium

# Check KubeVirt status
kubectl get pods -n kubevirt

# View topology events
kubectl describe topology native-demo
kubectl get events --sort-by=.metadata.creationTimestamp
```

## 🔄 Development Lifecycle

### Working on a New Feature

```bash
# 1. Create feature branch
git checkout -b feat/my-new-feature

# 2. Set up environment (if not already done)
./.develop/setup-native-dev.sh

# 3. Make your changes
# Edit files in pkg/, apis/, etc.

# 4. Test your changes
make test
make lint

# 5. Test with example topology
kubectl apply -f examples/native-architecture-demo.yaml

# 6. Commit your changes
git add .
git commit -m "feat: implement my new feature"

# 7. Push and create PR
git push origin feat/my-new-feature
```

### Making Changes to APIs

```bash
# 1. Edit apis/v1alpha1/topologyspec.go
# 2. Regenerate code
make run-generate

# 3. Update CRDs
make run-generate-crds

# 4. Test changes
make test
kubectl apply -f examples/native-architecture-demo.yaml
```

## 📊 Performance Testing

### Comparing Native vs Legacy Performance

```bash
# 1. Deploy with legacy mode
kubectl apply -f - <<EOF
apiVersion: clabernetes.containerlab.dev/v1alpha1
kind: Topology
metadata:
  name: legacy-test
spec:
  nativeExecution:
    executionMode: legacy
  definition:
    containerlab: |
      name: legacy-test
      topology:
        nodes:
          node1:
            kind: srl
            image: ghcr.io/nokia/srlinux:23.10.1
EOF

# 2. Deploy with native mode  
kubectl apply -f examples/native-architecture-demo.yaml

# 3. Compare metrics
kubectl top pods
kubectl get pods -o wide

# 4. Measure startup times
time kubectl wait --for=condition=ready pod -l clabernetes/topology=legacy-test --timeout=300s
time kubectl wait --for=condition=ready pod -l clabernetes/topology=native-demo --timeout=300s
```

## 🐛 Troubleshooting

### Environment Issues

```bash
# Reset environment completely
kind delete cluster --name clabernetes-native
./.develop/setup-native-dev.sh

# Check prerequisites
./.develop/validate-native-setup.sh

# View setup logs
./.develop/setup-native-dev.sh 2>&1 | tee setup.log
```

### Build Issues

```bash
# Clean and rebuild
go clean -cache
go mod tidy
make fmt
make test
```

### Deployment Issues

```bash
# Check controller logs
kubectl logs -n clabernetes-system deployment/clabernetes-manager

# Check topology status
kubectl describe topology <topology-name>

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp

# Check resource creation
kubectl get pods,services,deployments,vms -l clabernetes/topology=<topology-name>
```

## 📚 Next Steps

1. **Read Architecture Docs**: `.develop/README-NATIVE-ARCHITECTURE.md`
2. **Check TODO**: `TODO.md` for current development priorities
3. **Review CLAUDE.md**: Understanding AI-assisted development workflow
4. **Join Development**: Look at open issues and current Phase 2 tasks

## 🤝 Contributing

1. **Fork the repository**
2. **Follow this tutorial** to set up your environment
3. **Pick a task** from `TODO.md` Phase 2 or create an issue
4. **Make your changes** following the development workflow
5. **Submit a PR** with clear description and tests

## ⚠️ Important Notes

- This is experimental code largely generated by LLMs
- Use caution in production environments
- Always run the validation script after setup
- Keep TODO.md updated with your progress
- Test thoroughly before submitting PRs

---

**Happy coding! 🚀**

For questions or issues, please open a GitHub issue or check the troubleshooting section above.