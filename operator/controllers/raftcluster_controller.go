package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/hugovillarreal/conflux/operator/api/v1alpha1"
)

// RaftClusterReconciler reconciles a RaftCluster object
type RaftClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infra.hugo.dev,resources=raftclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infra.hugo.dev,resources=raftclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infra.hugo.dev,resources=raftclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RaftClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the RaftCluster instance
	raftCluster := &infrav1alpha1.RaftCluster{}
	if err := r.Get(ctx, req.NamespacedName, raftCluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling RaftCluster", "name", raftCluster.Name)

	// Reconcile StatefulSet
	if err := r.reconcileStatefulSet(ctx, raftCluster); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, raftCluster); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, raftCluster); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileStatefulSet ensures the StatefulSet exists and matches the desired state
func (r *RaftClusterReconciler) reconcileStatefulSet(ctx context.Context, raftCluster *infrav1alpha1.RaftCluster) error {
	logger := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      raftCluster.Name,
		Namespace: raftCluster.Namespace,
	}, statefulSet)

	if err != nil && errors.IsNotFound(err) {
		// Create StatefulSet
		statefulSet = r.buildStatefulSet(raftCluster)
		if err := r.Create(ctx, statefulSet); err != nil {
			return err
		}
		logger.Info("Created StatefulSet", "name", statefulSet.Name)
		return nil
	} else if err != nil {
		return err
	}

	// Update StatefulSet if needed
	desiredReplicas := *raftCluster.Spec.Replicas
	if *statefulSet.Spec.Replicas != desiredReplicas {
		statefulSet.Spec.Replicas = &desiredReplicas
		if err := r.Update(ctx, statefulSet); err != nil {
			return err
		}
		logger.Info("Updated StatefulSet replicas", "name", statefulSet.Name, "replicas", desiredReplicas)
	}

	return nil
}

// reconcileService ensures the Service exists
func (r *RaftClusterReconciler) reconcileService(ctx context.Context, raftCluster *infrav1alpha1.RaftCluster) error {
	logger := log.FromContext(ctx)

	service := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      raftCluster.Name,
		Namespace: raftCluster.Namespace,
	}, service)

	if err != nil && errors.IsNotFound(err) {
		// Create Service
		service = r.buildService(raftCluster)
		if err := r.Create(ctx, service); err != nil {
			return err
		}
		logger.Info("Created Service", "name", service.Name)
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

// buildStatefulSet creates a StatefulSet for the RaftCluster
func (r *RaftClusterReconciler) buildStatefulSet(raftCluster *infrav1alpha1.RaftCluster) *appsv1.StatefulSet {
	replicas := raftCluster.Spec.Replicas
	labels := map[string]string{
		"app":     "raft-node",
		"cluster": raftCluster.Name,
	}

	// Build peer list for environment variable
	// Format: peer1,peer2,peer3
	var peerList []string
	for i := int32(0); i < replicas; i++ {
		peerList = append(peerList, fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:9090",
			raftCluster.Name, i, raftCluster.Name, raftCluster.Namespace))
	}
	peersEnv := ""
	if len(peerList) > 0 {
		peersEnv = peerList[0]
		for i := 1; i < len(peerList); i++ {
			peersEnv += "," + peerList[i]
		}
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      raftCluster.Name,
			Namespace: raftCluster.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "raftnode",
							Image: raftCluster.Spec.Image,
							Env: []corev1.EnvVar{
								{
									Name:  "NODE_ID",
									Value: fmt.Sprintf("$(POD_NAME)"),
								},
								{
									Name:  "ENABLE_RAFT",
									Value: "true",
								},
								{
									Name:  "PEERS",
									Value: peersEnv,
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
								{
									Name:          "raft",
									ContainerPort: 9090,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{},
								Limits:   corev1.ResourceList{},
							},
						},
					},
				},
			},
		},
	}

	// Set resource requirements if specified
	if raftCluster.Spec.Resources.CPU != "" {
		// Parse and set CPU (simplified)
	}
	if raftCluster.Spec.Resources.Memory != "" {
		// Parse and set Memory (simplified)
	}

	// Set owner reference
	ctrl.SetControllerReference(raftCluster, statefulSet, r.Scheme)

	return statefulSet
}

// buildService creates a Service for the RaftCluster
func (r *RaftClusterReconciler) buildService(raftCluster *infrav1alpha1.RaftCluster) *corev1.Service {
	labels := map[string]string{
		"app":     "raft-node",
		"cluster": raftCluster.Name,
	}

	port := int32(8080)
	if raftCluster.Spec.Service.Port > 0 {
		port = raftCluster.Spec.Service.Port
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      raftCluster.Name,
			Namespace: raftCluster.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceType(raftCluster.Spec.Service.Type),
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port: port,
					Name: "http",
				},
			},
		},
	}

	// Set owner reference
	ctrl.SetControllerReference(raftCluster, service, r.Scheme)

	return service
}

// updateStatus updates the status of the RaftCluster
func (r *RaftClusterReconciler) updateStatus(ctx context.Context, raftCluster *infrav1alpha1.RaftCluster) error {
	// Get StatefulSet to check ready replicas
	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      raftCluster.Name,
		Namespace: raftCluster.Namespace,
	}, statefulSet)

	if err != nil {
		return err
	}

	// Update status
	raftCluster.Status.ReadyReplicas = statefulSet.Status.ReadyReplicas
	raftCluster.Status.Phase = "Healthy"
	if statefulSet.Status.ReadyReplicas < raftCluster.Spec.Replicas {
		raftCluster.Status.Phase = "Scaling"
	}

	// Update conditions
	raftCluster.Status.Conditions = []infrav1alpha1.Condition{
		{
			Type:   "Ready",
			Status: "True",
			Reason: "AllReplicasReady",
		},
	}

	return r.Status().Update(ctx, raftCluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RaftClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.RaftCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

