#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="clabernetes-native"
CILIUM_VERSION="1.16.5"
KUBEVIRT_VERSION="v1.4.0"

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        error "kind is not installed. Please install kind first: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed. Please install kubectl first"
    fi
    
    # Check if helm is installed
    if ! command -v helm &> /dev/null; then
        error "helm is not installed. Please install helm first"
    fi
    
    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        error "jq is not installed. Please install jq first"
    fi
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        error "Docker is not running. Please start Docker first"
    fi
    
    # Check inotify limits for KubeVirt
    local max_watches=$(sysctl -n fs.inotify.max_user_watches 2>/dev/null || echo "0")
    local max_instances=$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo "0")
    
    if [[ $max_watches -lt 524288 ]] || [[ $max_instances -lt 512 ]]; then
        warn "inotify limits are too low for KubeVirt (current: watches=$max_watches, instances=$max_instances)"
        warn "KubeVirt may fail with 'too many open files' errors"
        warn "Please increase limits on the host system:"
        warn "  echo 'fs.inotify.max_user_watches=524288' | sudo tee -a /etc/sysctl.conf"
        warn "  echo 'fs.inotify.max_user_instances=512' | sudo tee -a /etc/sysctl.conf"
        warn "  sudo sysctl -p"
        warn ""
        warn "NOTE: Future iterations of the dev environment may use minikube instead of kind"
        warn "      as it provides better support for virtualization workloads like KubeVirt"
    fi
    
    log "All prerequisites satisfied"
}

create_kind_cluster() {
    log "Creating kind cluster with native architecture support..."
    
    # Delete existing cluster if it exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        warn "Cluster ${CLUSTER_NAME} already exists. Deleting..."
        kind delete cluster --name "${CLUSTER_NAME}"
    fi
    
    # Create new cluster with custom configuration
    kind create cluster --config ".develop/kind-cluster-native.yml" --name "${CLUSTER_NAME}"
    
    # Export kubeconfig to a dedicated file
    local kubeconfig_file=".develop/kubeconfig-${CLUSTER_NAME}"
    kind get kubeconfig --name "${CLUSTER_NAME}" > "${kubeconfig_file}"
    
    # Set KUBECONFIG environment variable
    export KUBECONFIG="${PWD}/${kubeconfig_file}"
    
    # Verify cluster access
    kubectl cluster-info
    
    log "Kind cluster created successfully"
    log "Kubeconfig exported to: ${PWD}/${kubeconfig_file}"
    log "KUBECONFIG environment variable set"
}

install_cilium() {
    log "Installing Cilium CNI with Helm..."
    
    # Add Cilium Helm repository
    log "Adding Cilium Helm repository..."
    helm repo add cilium https://helm.cilium.io/
    helm repo update
    
    # Check if Cilium is already installed
    if helm list -n kube-system | grep -q cilium; then
        log "Cilium is already installed, skipping installation"
        return 0
    fi
    
    # Install Cilium with configuration optimized for kind
    log "Installing Cilium ${CILIUM_VERSION}..."
    helm upgrade --install cilium cilium/cilium \
        --version="${CILIUM_VERSION}" \
        --namespace=kube-system \
        --set kubeProxyReplacement=true \
        --set k8sServiceHost=${CLUSTER_NAME}-control-plane \
        --set k8sServicePort=6443 \
        --set hostServices.enabled=false \
        --set externalIPs.enabled=true \
        --set nodePort.enabled=true \
        --set hostPort.enabled=true \
        --set image.pullPolicy=IfNotPresent \
        --set ipam.mode=kubernetes \
        --set routingMode=native \
        --set ipv4NativeRoutingCIDR=10.244.0.0/16 \
        --set autoDirectNodeRoutes=true \
        --set hubble.enabled=true \
        --set hubble.relay.enabled=true \
        --set hubble.ui.enabled=true \
        --set operator.replicas=1 \
        --set nodeinit.enabled=true \
        --wait \
        --timeout=15m
    
    # Wait for Cilium to be ready
    log "Waiting for Cilium pods to be ready..."
    kubectl wait --for=condition=ready pod -l k8s-app=cilium -n kube-system --timeout=600s
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cilium-operator -n kube-system --timeout=600s
    
    # Verify Cilium installation
    if kubectl get pods -n kube-system -l k8s-app=cilium --no-headers | grep -v Running; then
        error "Cilium pods are not running properly"
    fi
    
    # Check node readiness
    log "Checking node readiness..."
    kubectl get nodes
    
    log "Cilium installed and running successfully with Helm"
}

install_kubevirt() {
    log "Installing KubeVirt operator..."
    
    # Install KubeVirt operator
    kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"
    
    # Wait for operator to be ready
    kubectl wait --for=condition=ready pod -l kubevirt.io=virt-operator -n kubevirt --timeout=300s
    
    # Install KubeVirt CR with development configuration
    kubectl apply -f - <<EOF
apiVersion: kubevirt.io/v1
kind: KubeVirt
metadata:
  name: kubevirt
  namespace: kubevirt
spec:
  certificateRotateStrategy: {}
  configuration:
    developerConfiguration:
      useEmulation: true
      featureGates:
        - DataVolumes
        - LiveMigration
        - CPUManager
        - CPUNodeDiscovery
        - Snapshot
        - VMExport
        - HotplugVolumes
        - HostDevices
        - GPU
        - NetworkPolicy
  customizeComponents: {}
  imagePullPolicy: IfNotPresent
  workloadUpdateStrategy: {}
EOF
    
    # Wait for KubeVirt to be ready
    log "Waiting for KubeVirt to be ready..."
    kubectl wait --for=condition=Available kubevirt kubevirt -n kubevirt --timeout=600s
    
    log "KubeVirt installed successfully"
}

install_cdi() {
    log "Installing Containerized Data Importer (CDI) for KubeVirt..."
    
    # Install CDI
    CDI_VERSION=$(curl -s https://api.github.com/repos/kubevirt/containerized-data-importer/releases/latest | jq -r .tag_name)
    kubectl apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml"
    kubectl apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml"
    
    # Wait for CDI to be ready
    log "Waiting for CDI operator to be ready..."
    kubectl wait --for=condition=Available cdi cdi -n cdi --timeout=300s || {
        log "CDI CR not available yet, waiting for operator pods..."
        kubectl wait --for=condition=ready pod -l name=cdi-operator -n cdi --timeout=300s
    }
    
    log "CDI installed successfully"
}

setup_development_tools() {
    log "Setting up development tools..."
    
    # Create namespace for clabernetes development
    kubectl create namespace clabernetes-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Install cert-manager for webhook certificates
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
    kubectl wait --for=condition=ready pod -l app=cert-manager -n cert-manager --timeout=300s
    kubectl wait --for=condition=ready pod -l app=cainjector -n cert-manager --timeout=300s
    kubectl wait --for=condition=ready pod -l app=webhook -n cert-manager --timeout=300s
    
    # Create development storage class
    kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: rancher.io/local-path
parameters:
  volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
EOF
    
    log "Development tools installed successfully"
}

create_test_resources() {
    log "Creating test resources for native architecture..."
    
    # Create a test namespace
    kubectl create namespace clabernetes-test --dry-run=client -o yaml | kubectl apply -f -
    
    # Create a test pod to verify networking
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: network-test
  namespace: clabernetes-test
  labels:
    app: network-test
spec:
  containers:
  - name: test
    image: nicolaka/netshoot
    command: ["/bin/bash"]
    args: ["-c", "sleep 3600"]
  restartPolicy: Always
EOF
    
    # Create a test NetworkPolicy
    kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: clabernetes-test
spec:
  podSelector:
    matchLabels:
      app: network-test
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - {}
  egress:
  - {}
EOF
    
    log "Test resources created successfully"
}

verify_installation() {
    log "Verifying installation..."
    
    # Check cluster nodes
    kubectl get nodes -o wide
    
    # Check Cilium status
    kubectl get pods -n kube-system -l k8s-app=cilium
    
    # Check KubeVirt status
    kubectl get pods -n kubevirt
    
    # Check CDI status
    kubectl get pods -n cdi
    
    # Test basic networking
    if kubectl get pod network-test -n clabernetes-test &> /dev/null; then
        kubectl wait --for=condition=ready pod network-test -n clabernetes-test --timeout=60s
        log "Network test pod is ready"
    fi
    
    log "Installation verification completed"
}

print_next_steps() {
    log "🎉 Native architecture development environment setup complete!"
    echo ""
    echo -e "${BLUE}Important - Kubeconfig Setup:${NC}"
    echo "The cluster kubeconfig is available at: ${PWD}/.develop/kubeconfig-${CLUSTER_NAME}"
    echo "To use this cluster in new terminal sessions, run:"
    echo -e "${YELLOW}export KUBECONFIG=${PWD}/.develop/kubeconfig-${CLUSTER_NAME}${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Run 'kubectl get nodes' to verify cluster status"
    echo "2. Run 'kubectl get pods -A' to see all running pods"
    echo "3. Use 'devspace dev' to start development"
    echo "4. Access Hubble UI with 'kubectl port-forward -n kube-system svc/hubble-ui 12000:80'"
    echo ""
    echo -e "${BLUE}Useful commands:${NC}"
    echo "- View Cilium status: kubectl get pods -n kube-system -l k8s-app=cilium"
    echo "- View KubeVirt status: kubectl get pods -n kubevirt"
    echo "- Test networking: kubectl exec -it network-test -n clabernetes-test -- ping 8.8.8.8"
    echo "- Delete cluster: kind delete cluster --name ${CLUSTER_NAME}"
    echo ""
    echo -e "${BLUE}Environment restoration:${NC}"
    echo "- To restore your original kubeconfig: unset KUBECONFIG"
}

main() {
    log "Setting up clabernetes native architecture development environment..."
    
    check_prerequisites
    create_kind_cluster
    install_cilium
    install_kubevirt
    install_cdi
    setup_development_tools
    create_test_resources
    verify_installation
    print_next_steps
}

# Run main function
main "$@"