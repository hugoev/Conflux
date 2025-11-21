/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	raftv1alpha1 "github.com/hugovillarreal/conflux/operator/api/v1alpha1"
)

// RaftClusterReconciler reconciles a RaftCluster object
type RaftClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=raft.conflux.io,resources=raftclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=raft.conflux.io,resources=raftclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=raft.conflux.io,resources=raftclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *RaftClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the RaftCluster instance
	var cluster raftv1alpha1.RaftCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle finalizer
	finalizerName := "raftcluster.finalizers.conflux.io"
	if cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(cluster.Finalizers, finalizerName) {
			cluster.Finalizers = append(cluster.Finalizers, finalizerName)
			if err := r.Update(ctx, &cluster); err != nil {
				log.Error(err, "failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Deleting: remove finalizer after owned resources are cleaned up (owner references will handle it)
		if containsString(cluster.Finalizers, finalizerName) {
			cluster.Finalizers = removeString(cluster.Finalizers, finalizerName)
			if err := r.Update(ctx, &cluster); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure headless Service exists
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name, Namespace: cluster.Namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.ClusterIP = "None"
		svc.Spec.Selector = map[string]string{"app": "raft", "raftcluster": cluster.Name}
		svc.Spec.Ports = []corev1.ServicePort{
			{Port: 8080, Name: "http"},
			{Port: 9090, Name: "raft-rpc"},
		}
		return ctrl.SetControllerReference(&cluster, svc, r.Scheme)
	})
	if err != nil {
		log.Error(err, "failed to create or update Service")
		return ctrl.Result{}, err
	}

	// Build desired StatefulSet spec
	desiredSts := r.buildStatefulSet(&cluster)

	// Check if StatefulSet exists
	existingSts := &appsv1.StatefulSet{}
	err = r.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, existingSts)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "failed to get StatefulSet")
			return ctrl.Result{}, err
		}
		// StatefulSet doesn't exist, create it
		if err := ctrl.SetControllerReference(&cluster, desiredSts, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredSts); err != nil {
			log.Error(err, "failed to create StatefulSet")
			return ctrl.Result{}, err
		}
		log.Info("Created StatefulSet", "name", desiredSts.Name)
	} else {
		// StatefulSet exists, check if recreation is needed
		if needsStatefulSetRecreation(existingSts, desiredSts) {
			log.Info("StatefulSet immutable fields changed, recreating", "name", existingSts.Name)

			// Delete existing StatefulSet (owner references will clean up pods)
			if err := r.Delete(ctx, existingSts); err != nil {
				log.Error(err, "failed to delete StatefulSet for recreation")
				return ctrl.Result{}, err
			}

			// Requeue to create new StatefulSet after deletion completes
			return ctrl.Result{Requeue: true}, nil
		}

		// Update mutable fields only
		existingSts.Spec.Replicas = desiredSts.Spec.Replicas
		if err := r.Update(ctx, existingSts); err != nil {
			log.Error(err, "failed to update StatefulSet")
			return ctrl.Result{}, err
		}
	}

	// Fetch the current StatefulSet for status update
	currentSts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, currentSts); err != nil {
		// StatefulSet might be deleted, requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status if needed
	if cluster.Status.Phase != "Running" || cluster.Status.ReadyReplicas != currentSts.Status.ReadyReplicas {
		cluster.Status.Phase = "Running"
		cluster.Status.ReadyReplicas = currentSts.Status.ReadyReplicas
		if err := r.Status().Update(ctx, &cluster); err != nil {
			log.Error(err, "failed to update RaftCluster status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildStatefulSet creates the desired StatefulSet spec for the RaftCluster
func (r *RaftClusterReconciler) buildStatefulSet(cluster *raftv1alpha1.RaftCluster) *appsv1.StatefulSet {
	replicas := cluster.Spec.Replicas

	// Generate PEERS environment variable
	peersEnv := buildPeersEnv(cluster)

	// Determine ImagePullPolicy based on image tag
	imagePullPolicy := corev1.PullIfNotPresent
	if strings.HasSuffix(cluster.Spec.Image, ":latest") {
		imagePullPolicy = corev1.PullNever
	}

	// Set ENABLE_RAFT based on replica count
	enableRaft := "false"
	if replicas > 1 {
		enableRaft = "true"
	}

	// Default storage size
	storageSize := cluster.Spec.StorageSize
	if storageSize == "" {
		storageSize = "1Gi"
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: cluster.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "raft", "raftcluster": cluster.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "raft", "raftcluster": cluster.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "raft",
						Image:           cluster.Spec.Image,
						ImagePullPolicy: imagePullPolicy,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080, Name: "http"},
							{ContainerPort: 9090, Name: "raft"},
						},
						Resources: cluster.Spec.Resources,
						Env: []corev1.EnvVar{
							{
								Name:      "NODE_ID",
								ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
							},
							{
								Name:  "PEERS",
								Value: peersEnv,
							},
							{
								Name:  "ENABLE_RAFT",
								Value: enableRaft,
							},
							{
								Name:  "DATA_DIR",
								Value: "/data",
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/data",
						}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/health",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
							TimeoutSeconds:      5,
							FailureThreshold:    3,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/health",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       5,
							TimeoutSeconds:      3,
							FailureThreshold:    2,
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storageSize),
						},
					},
					StorageClassName: cluster.Spec.StorageClassName,
				},
			}},
		},
	}

	return sts
}

// buildPeersEnv generates the PEERS environment variable using DNS-based discovery
func buildPeersEnv(cluster *raftv1alpha1.RaftCluster) string {
	if cluster.Spec.Replicas <= 1 {
		return "" // Single-node mode
	}

	peers := make([]string, 0, cluster.Spec.Replicas)
	for i := 0; i < int(cluster.Spec.Replicas); i++ {
		peer := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:9090",
			cluster.Name, i, cluster.Name, cluster.Namespace)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// needsStatefulSetRecreation checks if immutable fields have changed
func needsStatefulSetRecreation(existing, desired *appsv1.StatefulSet) bool {
	// Check ImagePullPolicy
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if existing.Spec.Template.Spec.Containers[0].ImagePullPolicy != desired.Spec.Template.Spec.Containers[0].ImagePullPolicy {
			return true
		}
	}

	// Check VolumeClaimTemplates
	if len(existing.Spec.VolumeClaimTemplates) != len(desired.Spec.VolumeClaimTemplates) {
		return true
	}

	// Check VolumeMounts
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		existingMounts := existing.Spec.Template.Spec.Containers[0].VolumeMounts
		desiredMounts := desired.Spec.Template.Spec.Containers[0].VolumeMounts
		if len(existingMounts) != len(desiredMounts) {
			return true
		}
	}

	// Check environment variables (PEERS, ENABLE_RAFT, DATA_DIR)
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		existingEnv := existing.Spec.Template.Spec.Containers[0].Env
		desiredEnv := desired.Spec.Template.Spec.Containers[0].Env

		existingEnvMap := make(map[string]string)
		for _, e := range existingEnv {
			existingEnvMap[e.Name] = e.Value
		}

		for _, e := range desiredEnv {
			if e.Name == "PEERS" || e.Name == "ENABLE_RAFT" || e.Name == "DATA_DIR" {
				if existingEnvMap[e.Name] != e.Value {
					return true
				}
			}
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *RaftClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&raftv1alpha1.RaftCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// Helper functions for finalizer handling
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
