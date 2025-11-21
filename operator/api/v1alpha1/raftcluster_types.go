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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RaftClusterSpec defines the desired state of RaftCluster
type RaftClusterSpec struct {
	// Replicas is the number of Raft nodes
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the Docker image to use
	Image string `json:"image,omitempty"`

	// Resources defines the resource requirements for each node
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// StorageSize is the size of the persistent volume for each node
	// +kubebuilder:default="1Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// StorageClassName is the storage class name for persistent volumes
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// RaftClusterStatus defines the observed state of RaftCluster
type RaftClusterStatus struct {
	// Phase represents the current state of the cluster
	Phase string `json:"phase,omitempty"`

	// ReadyReplicas is the number of ready nodes
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Leader is the name of the current leader node
	// +optional
	Leader string `json:"leader,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RaftCluster is the Schema for the raftclusters API
type RaftCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RaftClusterSpec   `json:"spec,omitempty"`
	Status RaftClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RaftClusterList contains a list of RaftCluster
type RaftClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RaftCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RaftCluster{}, &RaftClusterList{})
}
