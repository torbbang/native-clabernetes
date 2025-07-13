#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CLUSTER_NAME="clabernetes-native"
PASSED=0
FAILED=0

log() {
    echo -e "${GREEN}[VALIDATE] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
    ((FAILED++))
}

pass() {
    echo -e "${GREEN}[PASS] $1${NC}"
    ((PASSED++))
}

info() {
    echo -e "${BLUE}[INFO] $1${NC}"
}

validate_prerequisites() {
    log "Validating prerequisites..."
    
    local tools=("kind" "kubectl" "helm" "docker" "devspace" "go")
    
    for tool in "${tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            pass "$tool is installed"
        else
            error "$tool is not installed"
        fi
    done
    
    # Check Docker is running
    if docker info &> /dev/null; then
        pass "Docker is running"
    else
        error "Docker is not running"
    fi
    
    # Check Go version
    local go_version
    go_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | cut -c 3-)
    if [[ $(echo "$go_version >= 1.24" | bc -l) -eq 1 ]]; then
        pass "Go version $go_version is compatible"
    else
        error "Go version $go_version is too old (need 1.24+)"
    fi
}

validate_cluster() {
    log "Validating kind cluster..."
    
    # Check if cluster exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        pass "Kind cluster '$CLUSTER_NAME' exists"
        
        # Check if cluster is accessible
        if kubectl cluster-info --context "kind-${CLUSTER_NAME}" &> /dev/null; then
            pass "Cluster is accessible via kubectl"
        else
            error "Cluster exists but is not accessible"
        fi
    else
        error "Kind cluster '$CLUSTER_NAME' does not exist"
        info "Run '.develop/setup-native-dev.sh' to create the cluster"
        return
    fi
    
    # Check cluster nodes
    local node_count
    node_count=$(kubectl get nodes --no-headers | wc -l)
    if [[ $node_count -eq 3 ]]; then
        pass "Cluster has expected 3 nodes (1 control-plane, 2 workers)"
    else
        warn "Cluster has $node_count nodes (expected 3)"
    fi
}

validate_cilium() {
    log "Validating Cilium CNI..."
    
    # Check Cilium pods
    local cilium_pods
    cilium_pods=$(kubectl get pods -n kube-system -l k8s-app=cilium --no-headers 2>/dev/null | wc -l)
    
    if [[ $cilium_pods -gt 0 ]]; then
        pass "Cilium pods are deployed ($cilium_pods pods)"
        
        # Check if all Cilium pods are running
        local running_pods
        running_pods=$(kubectl get pods -n kube-system -l k8s-app=cilium --no-headers | grep -c Running || echo "0")
        
        if [[ $running_pods -eq $cilium_pods ]]; then
            pass "All Cilium pods are running"
        else
            error "$running_pods/$cilium_pods Cilium pods are running"
        fi
        
        # Check Cilium operator
        if kubectl get pods -n kube-system -l name=cilium-operator --no-headers | grep -q Running; then
            pass "Cilium operator is running"
        else
            error "Cilium operator is not running"
        fi
        
        # Check Hubble relay
        if kubectl get pods -n kube-system -l k8s-app=hubble-relay --no-headers | grep -q Running; then
            pass "Hubble relay is running"
        else
            warn "Hubble relay is not running"
        fi
        
    else
        error "No Cilium pods found"
    fi
}

validate_kubevirt() {
    log "Validating KubeVirt..."
    
    # Check if KubeVirt namespace exists
    if kubectl get namespace kubevirt &> /dev/null; then
        pass "KubeVirt namespace exists"
        
        # Check KubeVirt operator
        if kubectl get pods -n kubevirt -l kubevirt.io=virt-operator --no-headers | grep -q Running; then
            pass "KubeVirt operator is running"
        else
            error "KubeVirt operator is not running"
        fi
        
        # Check KubeVirt CR
        if kubectl get kubevirt kubevirt -n kubevirt &> /dev/null; then
            local kubevirt_status
            kubevirt_status=$(kubectl get kubevirt kubevirt -n kubevirt -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
            
            if [[ "$kubevirt_status" == "Deployed" ]]; then
                pass "KubeVirt is deployed and ready"
            else
                warn "KubeVirt status: $kubevirt_status"
            fi
        else
            error "KubeVirt CR not found"
        fi
        
        # Check CDI (Containerized Data Importer)
        if kubectl get namespace cdi &> /dev/null; then
            if kubectl get pods -n cdi -l app=cdi-operator --no-headers | grep -q Running; then
                pass "CDI operator is running"
            else
                warn "CDI operator is not running"
            fi
        else
            warn "CDI namespace not found"
        fi
        
    else
        error "KubeVirt namespace does not exist"
    fi
}

validate_networking() {
    log "Validating networking functionality..."
    
    # Create test namespace
    kubectl create namespace validate-native --dry-run=client -o yaml | kubectl apply -f - &> /dev/null
    
    # Deploy test pod
    kubectl apply -f - <<EOF &> /dev/null
apiVersion: v1
kind: Pod
metadata:
  name: network-validate-test
  namespace: validate-native
spec:
  containers:
  - name: test
    image: nicolaka/netshoot:latest
    command: ["/bin/bash", "-c", "sleep 60"]
  restartPolicy: Never
EOF
    
    # Wait for pod to be ready
    if kubectl wait --for=condition=ready pod network-validate-test -n validate-native --timeout=30s &> /dev/null; then
        pass "Test pod started successfully"
        
        # Test external connectivity
        if kubectl exec network-validate-test -n validate-native -- ping -c 2 8.8.8.8 &> /dev/null; then
            pass "External connectivity works"
        else
            error "External connectivity failed"
        fi
        
        # Test DNS resolution
        if kubectl exec network-validate-test -n validate-native -- nslookup kubernetes.default &> /dev/null; then
            pass "DNS resolution works"
        else
            error "DNS resolution failed"
        fi
        
    else
        error "Test pod failed to start"
    fi
    
    # Cleanup
    kubectl delete namespace validate-native --ignore-not-found=true &> /dev/null
}

validate_devspace_config() {
    log "Validating DevSpace configuration..."
    
    # Check if native devspace config exists
    if [[ -f ".develop/devspace-native.yaml" ]]; then
        pass "Native DevSpace configuration exists"
    else
        error "Native DevSpace configuration missing"
    fi
    
    # Check if setup script exists
    if [[ -f ".develop/setup-native-dev.sh" && -x ".develop/setup-native-dev.sh" ]]; then
        pass "Setup script exists and is executable"
    else
        error "Setup script missing or not executable"
    fi
    
    # Check if start script exists
    if [[ -f ".develop/start-native.sh" && -x ".develop/start-native.sh" ]]; then
        pass "Start script exists and is executable"
    else
        error "Start script missing or not executable"
    fi
}

validate_github_workflows() {
    log "Validating GitHub workflows..."
    
    if [[ -f ".github/workflows/test-native-architecture.yaml" ]]; then
        pass "Native architecture GitHub workflow exists"
    else
        error "Native architecture GitHub workflow missing"
    fi
}

test_basic_functionality() {
    log "Testing basic functionality..."
    
    # Test that we can create a simple deployment
    kubectl apply -f - <<EOF &> /dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx:alpine
        ports:
        - containerPort: 80
EOF
    
    if kubectl wait --for=condition=available deployment test-deployment --timeout=60s &> /dev/null; then
        pass "Basic deployment functionality works"
    else
        error "Basic deployment functionality failed"
    fi
    
    # Cleanup
    kubectl delete deployment test-deployment --ignore-not-found=true &> /dev/null
}

print_summary() {
    echo ""
    echo "================================================"
    echo -e "${BLUE}VALIDATION SUMMARY${NC}"
    echo "================================================"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    
    if [[ $FAILED -eq 0 ]]; then
        echo -e "${GREEN}🎉 All validations passed! Your native architecture development environment is ready.${NC}"
        echo ""
        echo -e "${BLUE}Next steps:${NC}"
        echo "1. Start development: DEVSPACE_CONFIG=./.develop/devspace-native.yaml devspace dev --profile native-dev"
        echo "2. Read the documentation: .develop/README-NATIVE-ARCHITECTURE.md"
        echo "3. Run tests: make test-native"
        echo ""
    else
        echo -e "${RED}❌ Some validations failed. Please fix the issues above before proceeding.${NC}"
        echo ""
        echo -e "${BLUE}Common fixes:${NC}"
        echo "- Run '.develop/setup-native-dev.sh' to set up the complete environment"
        echo "- Check that Docker is running and kind cluster is accessible"
        echo "- Verify all required tools are installed"
        echo ""
    fi
}

main() {
    echo -e "${BLUE}"
    echo "================================================"
    echo "  Clabernetes Native Architecture Validation"
    echo "================================================"
    echo -e "${NC}"
    
    validate_prerequisites
    validate_cluster
    validate_cilium
    validate_kubevirt
    validate_networking
    validate_devspace_config
    validate_github_workflows
    test_basic_functionality
    
    print_summary
    
    return $FAILED
}

main "$@"