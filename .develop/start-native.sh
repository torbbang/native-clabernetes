#!/bin/bash
set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

info() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

log "🚀 Starting clabernetes native architecture development environment..."

# Check if we're in the correct directory
if [[ ! -f "go.mod" ]] || [[ ! -d ".develop" ]]; then
    warn "This script should be run from the clabernetes root directory"
    exit 1
fi

# Set up Go environment
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# Check if native cluster is available
if ! kubectl cluster-info --context kind-clabernetes-native &> /dev/null; then
    warn "Native development cluster not found. Setting it up..."
    .develop/setup-native-dev.sh
fi

# Verify Cilium is running
log "Checking Cilium status..."
if ! kubectl get pods -n kube-system -l k8s-app=cilium --no-headers 2>/dev/null | grep -q Running; then
    warn "Cilium is not running properly. Please check the cluster setup."
fi

# Verify KubeVirt is running
log "Checking KubeVirt status..."
if ! kubectl get pods -n kubevirt --no-headers 2>/dev/null | grep -q Running; then
    warn "KubeVirt is not running properly. Some VM features may not work."
fi

# Create necessary directories for development
mkdir -p /tmp/clabernetes-native/{logs,configs,cache}

# Set up development environment variables
export CLABERNETES_DEV_MODE=true
export EXECUTION_MODE=native
export CILIUM_ENABLED=true
export KUBEVIRT_ENABLED=true
export MANAGER_NAMESPACE=clabernetes-system
export CONTROLLER_LOG_LEVEL=debug
export MANAGER_LOG_LEVEL=debug

# Create development configuration
cat > /tmp/clabernetes-native/dev-config.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: clabernetes-native-dev-config
  namespace: clabernetes-system
data:
  execution-mode: "native"
  networking-mode: "cilium"
  kubevirt-enabled: "true"
  development-mode: "true"
EOF

# Apply development configuration
kubectl apply -f /tmp/clabernetes-native/dev-config.yaml 2>/dev/null || true

log "Environment variables set:"
info "  EXECUTION_MODE=${EXECUTION_MODE}"
info "  CILIUM_ENABLED=${CILIUM_ENABLED}"
info "  KUBEVIRT_ENABLED=${KUBEVIRT_ENABLED}"
info "  MANAGER_NAMESPACE=${MANAGER_NAMESPACE}"

log "Development tools available:"
info "  kubectl - Kubernetes CLI"
info "  hubble - Cilium networking debugging (kubectl port-forward -n kube-system svc/hubble-ui 12000:80)"
info "  virtctl - KubeVirt VM management"

log "Starting development session..."
info "You can now:"
info "  - Build and test native container execution"
info "  - Develop Cilium CNI integration"
info "  - Test KubeVirt VM functionality"
info "  - Debug networking with Hubble"

# Create useful aliases for development
alias k='kubectl'
alias kns='kubectl config set-context --current --namespace'
alias logs='kubectl logs'
alias pods='kubectl get pods'
alias describe='kubectl describe'

# Export aliases for the session
export -f log warn info

# Make development tools available
export PATH="/usr/local/bin:$PATH"

# Start an interactive bash session with development environment
exec /bin/bash --init-file <(echo "
    source ~/.bashrc 2>/dev/null || true
    alias k='kubectl'
    alias kns='kubectl config set-context --current --namespace'
    alias logs='kubectl logs'
    alias pods='kubectl get pods'
    alias describe='kubectl describe'
    
    echo -e '${GREEN}🎉 Clabernetes Native Development Environment Ready!${NC}'
    echo -e '${BLUE}Current context: $(kubectl config current-context)${NC}'
    echo -e '${BLUE}Current namespace: $(kubectl config view --minify --output \"jsonpath={..namespace}\")${NC}'
    echo ''
    echo -e '${YELLOW}Useful commands:${NC}'
    echo -e '  k get pods -A                    # View all pods'
    echo -e '  k get nodes -o wide              # View cluster nodes'
    echo -e '  k logs -f <pod> -n <namespace>   # Follow pod logs'
    echo -e '  make build                       # Build clabernetes'
    echo -e '  make test                        # Run tests'
    echo -e '  devspace dev                     # Start development mode'
    echo ''
    echo -e '${YELLOW}Native architecture features:${NC}'
    echo -e '  - Cilium CNI for networking'
    echo -e '  - KubeVirt for VM workloads'  
    echo -e '  - Native container execution'
    echo -e '  - No Docker-in-Docker overhead'
    echo ''
")