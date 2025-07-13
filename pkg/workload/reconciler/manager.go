package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
	"github.com/srl-labs/clabernetes/pkg/workload/renderer"
	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// WorkloadReconciler manages the reconciliation of native workloads
type WorkloadReconciler struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	logger        claberneteslogging.Instance
	executorMgr   *common.Manager
}

// NewWorkloadReconciler creates a new workload reconciler
func NewWorkloadReconciler(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	logger claberneteslogging.Instance,
	executorMgr *common.Manager,
) *WorkloadReconciler {
	return &WorkloadReconciler{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		logger:        logger,
		executorMgr:   executorMgr,
	}
}

// ReconcileResult contains the result of a reconciliation operation
type ReconcileResult struct {
	// Created contains newly created resources
	Created []ResourceInfo
	// Updated contains updated resources
	Updated []ResourceInfo
	// Deleted contains deleted resources
	Deleted []ResourceInfo
	// Errors contains any errors that occurred
	Errors []error
}

// ResourceInfo contains information about a reconciled resource
type ResourceInfo struct {
	// Type is the resource type
	Type string
	// Name is the resource name
	Name string
	// Namespace is the resource namespace
	Namespace string
	// WorkloadType indicates if this is a container or VM workload
	WorkloadType common.WorkloadType
}

// ReconcileTopologyWorkloads reconciles all workloads for a topology
func (r *WorkloadReconciler) ReconcileTopologyWorkloads(
	ctx context.Context,
	topology *clabernetesapisv1alpha1.Topology,
	renderResults map[string]*renderer.RenderResult,
	namespace string,
) (*ReconcileResult, error) {
	r.logger.Debugf("Reconciling workloads for topology %s", topology.Name)
	
	result := &ReconcileResult{
		Created: []ResourceInfo{},
		Updated: []ResourceInfo{},
		Deleted: []ResourceInfo{},
		Errors:  []error{},
	}
	
	// Get existing resources
	existingResources, err := r.getExistingResources(ctx, topology, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing resources: %w", err)
	}
	
	// Track desired resources
	desiredResources := make(map[string]renderer.Resource)
	
	// Process each node's render results
	for nodeName, renderResult := range renderResults {
		for _, resource := range renderResult.Resources {
			key := r.getResourceKey(resource)
			desiredResources[key] = resource
			
			if err := r.reconcileResource(ctx, resource, renderResult.WorkloadType, namespace, result); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to reconcile resource %s for node %s: %w", key, nodeName, err))
			}
		}
	}
	
	// Delete resources that are no longer desired
	if err := r.deleteUnwantedResources(ctx, existingResources, desiredResources, topology, result); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to delete unwanted resources: %w", err))
	}
	
	r.logger.Debugf("Reconciliation complete: %d created, %d updated, %d deleted, %d errors",
		len(result.Created), len(result.Updated), len(result.Deleted), len(result.Errors))
	
	return result, nil
}

// reconcileResource reconciles a single resource
func (r *WorkloadReconciler) reconcileResource(
	ctx context.Context,
	resource renderer.Resource,
	workloadType common.WorkloadType,
	namespace string,
	result *ReconcileResult,
) error {
	switch resource.Type {
	case "Deployment":
		return r.reconcileDeployment(ctx, resource.Object.(*k8sappsv1.Deployment), workloadType, result)
		
	case "Service":
		return r.reconcileService(ctx, resource.Object.(*k8scorev1.Service), workloadType, result)
		
	case "ConfigMap":
		return r.reconcileConfigMap(ctx, resource.Object.(*k8scorev1.ConfigMap), workloadType, result)
		
	case "VirtualMachine":
		return r.reconcileVirtualMachine(ctx, resource.Object.(*unstructured.Unstructured), workloadType, result)
		
	default:
		return fmt.Errorf("unsupported resource type: %s", resource.Type)
	}
}

// reconcileDeployment reconciles a Deployment resource
func (r *WorkloadReconciler) reconcileDeployment(
	ctx context.Context,
	deployment *k8sappsv1.Deployment,
	workloadType common.WorkloadType,
	result *ReconcileResult,
) error {
	existing, err := r.kubeClient.AppsV1().Deployments(deployment.Namespace).Get(
		ctx, deployment.Name, metav1.GetOptions{},
	)
	
	if errors.IsNotFound(err) {
		// Create new deployment
		_, err = r.kubeClient.AppsV1().Deployments(deployment.Namespace).Create(
			ctx, deployment, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create deployment %s: %w", deployment.Name, err)
		}
		
		result.Created = append(result.Created, ResourceInfo{
			Type:         "Deployment",
			Name:         deployment.Name,
			Namespace:    deployment.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Created deployment %s", deployment.Name)
		return nil
	}
	
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", deployment.Name, err)
	}
	
	// Check if update is needed
	if r.needsDeploymentUpdate(existing, deployment) {
		// Preserve resource version for update
		deployment.ResourceVersion = existing.ResourceVersion
		
		_, err = r.kubeClient.AppsV1().Deployments(deployment.Namespace).Update(
			ctx, deployment, metav1.UpdateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to update deployment %s: %w", deployment.Name, err)
		}
		
		result.Updated = append(result.Updated, ResourceInfo{
			Type:         "Deployment",
			Name:         deployment.Name,
			Namespace:    deployment.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Updated deployment %s", deployment.Name)
	}
	
	return nil
}

// reconcileService reconciles a Service resource
func (r *WorkloadReconciler) reconcileService(
	ctx context.Context,
	service *k8scorev1.Service,
	workloadType common.WorkloadType,
	result *ReconcileResult,
) error {
	existing, err := r.kubeClient.CoreV1().Services(service.Namespace).Get(
		ctx, service.Name, metav1.GetOptions{},
	)
	
	if errors.IsNotFound(err) {
		// Create new service
		_, err = r.kubeClient.CoreV1().Services(service.Namespace).Create(
			ctx, service, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create service %s: %w", service.Name, err)
		}
		
		result.Created = append(result.Created, ResourceInfo{
			Type:         "Service",
			Name:         service.Name,
			Namespace:    service.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Created service %s", service.Name)
		return nil
	}
	
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", service.Name, err)
	}
	
	// Check if update is needed
	if r.needsServiceUpdate(existing, service) {
		// Preserve fields that shouldn't be updated
		service.ResourceVersion = existing.ResourceVersion
		service.Spec.ClusterIP = existing.Spec.ClusterIP
		
		_, err = r.kubeClient.CoreV1().Services(service.Namespace).Update(
			ctx, service, metav1.UpdateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to update service %s: %w", service.Name, err)
		}
		
		result.Updated = append(result.Updated, ResourceInfo{
			Type:         "Service",
			Name:         service.Name,
			Namespace:    service.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Updated service %s", service.Name)
	}
	
	return nil
}

// reconcileConfigMap reconciles a ConfigMap resource
func (r *WorkloadReconciler) reconcileConfigMap(
	ctx context.Context,
	configMap *k8scorev1.ConfigMap,
	workloadType common.WorkloadType,
	result *ReconcileResult,
) error {
	existing, err := r.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Get(
		ctx, configMap.Name, metav1.GetOptions{},
	)
	
	if errors.IsNotFound(err) {
		// Create new configmap
		_, err = r.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Create(
			ctx, configMap, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create configmap %s: %w", configMap.Name, err)
		}
		
		result.Created = append(result.Created, ResourceInfo{
			Type:         "ConfigMap",
			Name:         configMap.Name,
			Namespace:    configMap.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Created configmap %s", configMap.Name)
		return nil
	}
	
	if err != nil {
		return fmt.Errorf("failed to get configmap %s: %w", configMap.Name, err)
	}
	
	// Check if update is needed
	if !reflect.DeepEqual(existing.Data, configMap.Data) {
		configMap.ResourceVersion = existing.ResourceVersion
		
		_, err = r.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Update(
			ctx, configMap, metav1.UpdateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to update configmap %s: %w", configMap.Name, err)
		}
		
		result.Updated = append(result.Updated, ResourceInfo{
			Type:         "ConfigMap",
			Name:         configMap.Name,
			Namespace:    configMap.Namespace,
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Updated configmap %s", configMap.Name)
	}
	
	return nil
}

// reconcileVirtualMachine reconciles a VirtualMachine resource
func (r *WorkloadReconciler) reconcileVirtualMachine(
	ctx context.Context,
	vm *unstructured.Unstructured,
	workloadType common.WorkloadType,
	result *ReconcileResult,
) error {
	vmResource := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	
	existing, err := r.dynamicClient.Resource(vmResource).Namespace(vm.GetNamespace()).Get(
		ctx, vm.GetName(), metav1.GetOptions{},
	)
	
	if errors.IsNotFound(err) {
		// Create new VM
		_, err = r.dynamicClient.Resource(vmResource).Namespace(vm.GetNamespace()).Create(
			ctx, vm, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create VM %s: %w", vm.GetName(), err)
		}
		
		result.Created = append(result.Created, ResourceInfo{
			Type:         "VirtualMachine",
			Name:         vm.GetName(),
			Namespace:    vm.GetNamespace(),
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Created VM %s", vm.GetName())
		return nil
	}
	
	if err != nil {
		return fmt.Errorf("failed to get VM %s: %w", vm.GetName(), err)
	}
	
	// For VMs, we'll do a simple spec comparison
	existingSpec, _, _ := unstructured.NestedMap(existing.Object, "spec")
	desiredSpec, _, _ := unstructured.NestedMap(vm.Object, "spec")
	
	if !reflect.DeepEqual(existingSpec, desiredSpec) {
		vm.SetResourceVersion(existing.GetResourceVersion())
		
		_, err = r.dynamicClient.Resource(vmResource).Namespace(vm.GetNamespace()).Update(
			ctx, vm, metav1.UpdateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to update VM %s: %w", vm.GetName(), err)
		}
		
		result.Updated = append(result.Updated, ResourceInfo{
			Type:         "VirtualMachine",
			Name:         vm.GetName(),
			Namespace:    vm.GetNamespace(),
			WorkloadType: workloadType,
		})
		
		r.logger.Debugf("Updated VM %s", vm.GetName())
	}
	
	return nil
}

// needsDeploymentUpdate checks if a deployment needs to be updated
func (r *WorkloadReconciler) needsDeploymentUpdate(existing, desired *k8sappsv1.Deployment) bool {
	// Compare important fields
	if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers, desired.Spec.Template.Spec.Containers) {
		return true
	}
	
	if !reflect.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	
	if !reflect.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	
	return false
}

// needsServiceUpdate checks if a service needs to be updated
func (r *WorkloadReconciler) needsServiceUpdate(existing, desired *k8scorev1.Service) bool {
	// Compare ports
	if !reflect.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}
	
	// Compare selector
	if !reflect.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		return true
	}
	
	// Compare type
	if existing.Spec.Type != desired.Spec.Type {
		return true
	}
	
	return false
}

// getExistingResources gets all existing resources for a topology
func (r *WorkloadReconciler) getExistingResources(
	ctx context.Context,
	topology *clabernetesapisv1alpha1.Topology,
	namespace string,
) (map[string]interface{}, error) {
	resources := make(map[string]interface{})
	
	// Get deployments
	deployments, err := r.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", clabernetesconstants.LabelTopology, topology.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	
	for i := range deployments.Items {
		key := fmt.Sprintf("Deployment/%s", deployments.Items[i].Name)
		resources[key] = &deployments.Items[i]
	}
	
	// Get services
	services, err := r.kubeClient.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", clabernetesconstants.LabelTopology, topology.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	
	for i := range services.Items {
		key := fmt.Sprintf("Service/%s", services.Items[i].Name)
		resources[key] = &services.Items[i]
	}
	
	// Get configmaps
	configMaps, err := r.kubeClient.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", clabernetesconstants.LabelTopology, topology.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps: %w", err)
	}
	
	for i := range configMaps.Items {
		key := fmt.Sprintf("ConfigMap/%s", configMaps.Items[i].Name)
		resources[key] = &configMaps.Items[i]
	}
	
	return resources, nil
}

// deleteUnwantedResources deletes resources that are no longer desired
func (r *WorkloadReconciler) deleteUnwantedResources(
	ctx context.Context,
	existing map[string]interface{},
	desired map[string]renderer.Resource,
	topology *clabernetesapisv1alpha1.Topology,
	result *ReconcileResult,
) error {
	for existingKey := range existing {
		if _, found := desired[existingKey]; !found {
			if err := r.deleteResource(ctx, existingKey, existing[existingKey], result); err != nil {
				return fmt.Errorf("failed to delete resource %s: %w", existingKey, err)
			}
		}
	}
	
	return nil
}

// deleteResource deletes a single resource
func (r *WorkloadReconciler) deleteResource(
	ctx context.Context,
	key string,
	resource interface{},
	result *ReconcileResult,
) error {
	// Parse the key to get type and name
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid resource key: %s", key)
	}
	
	resourceType := parts[0]
	resourceName := parts[1]
	
	switch resourceType {
	case "Deployment":
		deployment := resource.(*k8sappsv1.Deployment)
		err := r.kubeClient.AppsV1().Deployments(deployment.Namespace).Delete(
			ctx, resourceName, metav1.DeleteOptions{},
		)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		
	case "Service":
		service := resource.(*k8scorev1.Service)
		err := r.kubeClient.CoreV1().Services(service.Namespace).Delete(
			ctx, resourceName, metav1.DeleteOptions{},
		)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		
	case "ConfigMap":
		configMap := resource.(*k8scorev1.ConfigMap)
		err := r.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(
			ctx, resourceName, metav1.DeleteOptions{},
		)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	
	result.Deleted = append(result.Deleted, ResourceInfo{
		Type: resourceType,
		Name: resourceName,
	})
	
	r.logger.Debugf("Deleted %s %s", resourceType, resourceName)
	return nil
}

// getResourceKey generates a unique key for a resource
func (r *WorkloadReconciler) getResourceKey(resource renderer.Resource) string {
	switch obj := resource.Object.(type) {
	case *k8sappsv1.Deployment:
		return fmt.Sprintf("Deployment/%s", obj.Name)
	case *k8scorev1.Service:
		return fmt.Sprintf("Service/%s", obj.Name)
	case *k8scorev1.ConfigMap:
		return fmt.Sprintf("ConfigMap/%s", obj.Name)
	case *unstructured.Unstructured:
		return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
	default:
		return fmt.Sprintf("Unknown/%s", resource.Type)
	}
}