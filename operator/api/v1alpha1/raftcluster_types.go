package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RaftClusterSpec defines the desired state of RaftCluster
type RaftClusterSpec struct {
	// Replicas is the desired number of nodes in the cluster
	Replicas int32 `json:"replicas"`

	// Image is the container image to use
	Image string `json:"image"`

	// Resources defines resource limits and requests
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Storage defines storage configuration
	Storage StorageSpec `json:"storage,omitempty"`

	// Service defines the service configuration
	Service ServiceSpec `json:"service,omitempty"`
}

// ResourceRequirements defines resource limits and requests
type ResourceRequirements struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// StorageSpec defines storage configuration
type StorageSpec struct {
	Size string `json:"size,omitempty"`
}

// ServiceSpec defines service configuration
type ServiceSpec struct {
	Type string `json:"type,omitempty"`
	Port int32  `json:"port,omitempty"`
}

// RaftClusterStatus defines the observed state of RaftCluster
type RaftClusterStatus struct {
	// Phase represents the current phase of the cluster
	Phase string `json:"phase,omitempty"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Leader is the node ID of the current leader
	Leader string `json:"leader,omitempty"`

	// Conditions represent the latest available observations
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition represents a condition
type Condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
//+kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
//+kubebuilder:printcolumn:name="Leader",type="string",JSONPath=".status.leader"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
