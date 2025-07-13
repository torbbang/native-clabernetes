#!/bin/bash
# Helper script to set KUBECONFIG for the native clabernetes cluster
# Usage: source .develop/use-native-cluster.sh

CLUSTER_NAME="clabernetes-native"
KUBECONFIG_FILE=".develop/kubeconfig-${CLUSTER_NAME}"

# Check if we're in the right directory
if [[ ! -f ".develop/kind-cluster-native.yml" ]]; then
    echo "❌ Error: This script must be run from the clabernetes project root directory"
    return 1 2>/dev/null || exit 1
fi

# Check if kubeconfig file exists
if [[ ! -f "${KUBECONFIG_FILE}" ]]; then
    echo "❌ Error: Kubeconfig file not found: ${KUBECONFIG_FILE}"
    echo "💡 Hint: Run '.develop/setup-native-dev.sh' to create the cluster first"
    return 1 2>/dev/null || exit 1
fi

# Set KUBECONFIG environment variable
export KUBECONFIG="${PWD}/${KUBECONFIG_FILE}"

# Verify connection
if kubectl cluster-info >/dev/null 2>&1; then
    echo "✅ Successfully connected to native clabernetes cluster"
    echo "📁 Using kubeconfig: ${PWD}/${KUBECONFIG_FILE}"
    echo ""
    echo "🔧 Cluster status:"
    kubectl get nodes
else
    echo "❌ Error: Cannot connect to cluster"
    echo "💡 Hint: The cluster might not be running. Check with 'kind get clusters'"
    return 1 2>/dev/null || exit 1
fi

echo ""
echo "💡 To restore your original kubeconfig in this session:"
echo "   unset KUBECONFIG"