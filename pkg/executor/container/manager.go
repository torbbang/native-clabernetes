package container

import (
	"context"
	"fmt"

	"github.com/srl-labs/clabernetes/pkg/executor/common"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ContainerExecutor implements the Executor interface for container workloads
type ContainerExecutor struct {
	kubeClient kubernetes.Interface
	namespace  string
	logger     claberneteslogging.Instance
}

// NewContainerExecutor creates a new container executor
func NewContainerExecutor(
	kubeClient kubernetes.Interface,
	namespace string,
	logger claberneteslogging.Instance,
) *ContainerExecutor {
	return &ContainerExecutor{
		kubeClient: kubeClient,
		namespace:  namespace,
		logger:     logger,
	}
}

// Execute creates and starts a container workload
func (e *ContainerExecutor) Execute(ctx context.Context, config *common.NodeConfig) (*common.ExecutionResult, error) {
	e.logger.Debugf("Creating container workload for node %s", config.Name)
	
	// Create deployment for the node
	deployment := e.renderDeployment(config)
	
	createdDeployment, err := e.kubeClient.AppsV1().Deployments(e.namespace).Create(
		ctx, deployment, metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment for node %s: %w", config.Name, err)
	}
	
	// Create service for the node
	service := e.renderService(config)
	_, err = e.kubeClient.CoreV1().Services(e.namespace).Create(
		ctx, service, metav1.CreateOptions{},
	)
	if err != nil {
		e.logger.Warnf("Failed to create service for node %s: %v", config.Name, err)
	}
	
	return &common.ExecutionResult{
		WorkloadType: common.WorkloadTypeContainer,
		Name:         createdDeployment.Name,
		Namespace:    createdDeployment.Namespace,
		Status:       "Creating",
		Ready:        false,
		Message:      "Container deployment created successfully",
	}, nil
}

// Delete removes a container workload
func (e *ContainerExecutor) Delete(ctx context.Context, name, namespace string) error {
	e.logger.Debugf("Deleting container workload %s in namespace %s", name, namespace)
	
	// Delete deployment
	err := e.kubeClient.AppsV1().Deployments(namespace).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete deployment %s: %w", name, err)
	}
	
	// Delete service
	err = e.kubeClient.CoreV1().Services(namespace).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
	if err != nil {
		e.logger.Warnf("Failed to delete service %s: %v", name, err)
	}
	
	return nil
}

// GetStatus returns the current status of a container workload
func (e *ContainerExecutor) GetStatus(ctx context.Context, name, namespace string) (*common.ExecutionResult, error) {
	deployment, err := e.kubeClient.AppsV1().Deployments(namespace).Get(
		ctx, name, metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s: %w", name, err)
	}
	
	ready := deployment.Status.ReadyReplicas > 0
	status := "Creating"
	message := "Deployment is starting"
	
	if ready {
		status = "Running"
		message = "Deployment is running"
	} else if deployment.Status.Replicas > 0 {
		status = "Pending"
		message = "Deployment is pending"
	}
	
	return &common.ExecutionResult{
		WorkloadType: common.WorkloadTypeContainer,
		Name:         deployment.Name,
		Namespace:    deployment.Namespace,
		Status:       status,
		Ready:        ready,
		Message:      message,
	}, nil
}

// GetLogs returns logs from the container workload
func (e *ContainerExecutor) GetLogs(ctx context.Context, name, namespace string) (string, error) {
	// Get pods for the deployment
	pods, err := e.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", name),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for deployment %s: %w", name, err)
	}
	
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for deployment %s", name)
	}
	
	// Get logs from the first pod
	pod := pods.Items[0]
	logOptions := &k8scorev1.PodLogOptions{
		Container: pod.Spec.Containers[0].Name,
	}
	
	logStream, err := e.kubeClient.CoreV1().Pods(namespace).GetLogs(pod.Name, logOptions).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for pod %s: %w", pod.Name, err)
	}
	defer logStream.Close()
	
	// Read logs (simplified for now)
	return "Container logs would be streamed here", nil
}

// GetWorkloadType returns the workload type this executor handles
func (e *ContainerExecutor) GetWorkloadType() common.WorkloadType {
	return common.WorkloadTypeContainer
}

// renderDeployment creates a Kubernetes deployment for the node
func (e *ContainerExecutor) renderDeployment(config *common.NodeConfig) *k8sappsv1.Deployment {
	labels := map[string]string{
		"app":                               config.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
		"clabernetes/execution-mode":        "native",
		"clabernetes/workload-type":         "container",
	}
	
	// Merge additional labels
	for k, v := range config.Labels {
		labels[k] = v
	}
	
	annotations := map[string]string{
		"clabernetes/node-kind": config.Kind,
		"clabernetes/image":     config.Image,
	}
	
	// Merge additional annotations
	for k, v := range config.Annotations {
		annotations[k] = v
	}
	
	// Environment variables
	env := []k8scorev1.EnvVar{
		{
			Name:  "NODE_NAME",
			Value: config.Name,
		},
		{
			Name:  "NODE_KIND",
			Value: config.Kind,
		},
		{
			Name:  "EXECUTION_MODE",
			Value: "native",
		},
	}
	
	// Add custom environment variables
	for k, v := range config.Environment {
		env = append(env, k8scorev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	
	// Container specification
	container := k8scorev1.Container{
		Name:  config.Name,
		Image: config.Image,
		Env:   env,
		Ports: []k8scorev1.ContainerPort{
			{
				Name:          "ssh",
				ContainerPort: 22,
				Protocol:      k8scorev1.ProtocolTCP,
			},
			{
				Name:          "netconf",
				ContainerPort: 830,
				Protocol:      k8scorev1.ProtocolTCP,
			},
			{
				Name:          "gnmi",
				ContainerPort: 57400,
				Protocol:      k8scorev1.ProtocolTCP,
			},
		},
		SecurityContext: &k8scorev1.SecurityContext{
			Capabilities: &k8scorev1.Capabilities{
				Add: []k8scorev1.Capability{
					"NET_ADMIN",
					"SYS_ADMIN",
				},
			},
		},
		ImagePullPolicy: k8scorev1.PullIfNotPresent,
	}
	
	// Apply resource requirements if specified
	if config.Resources != nil {
		container.Resources = *config.Resources
	}
	
	replicas := int32(1)
	
	return &k8sappsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.Name,
			Namespace:   e.namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: k8sappsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": config.Name,
				},
			},
			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: k8scorev1.PodSpec{
					Containers: []k8scorev1.Container{container},
					RestartPolicy: k8scorev1.RestartPolicyAlways,
					DNSPolicy:     k8scorev1.DNSClusterFirst,
				},
			},
		},
	}
}

// renderService creates a Kubernetes service for the node
func (e *ContainerExecutor) renderService(config *common.NodeConfig) *k8scorev1.Service {
	labels := map[string]string{
		"app": config.Name,
		clabernetesconstants.LabelTopologyNode: config.Name,
	}
	
	ports := []k8scorev1.ServicePort{
		{
			Name:     "ssh",
			Port:     22,
			Protocol: k8scorev1.ProtocolTCP,
		},
		{
			Name:     "netconf",
			Port:     830,
			Protocol: k8scorev1.ProtocolTCP,
		},
		{
			Name:     "gnmi",
			Port:     57400,
			Protocol: k8scorev1.ProtocolTCP,
		},
	}
	
	return &k8scorev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: e.namespace,
			Labels:    labels,
		},
		Spec: k8scorev1.ServiceSpec{
			Selector: map[string]string{
				"app": config.Name,
			},
			Ports: ports,
			Type:  k8scorev1.ServiceTypeClusterIP,
		},
	}
}