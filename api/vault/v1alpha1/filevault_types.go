/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FilevaultConditionType string

const (
	FilevaultReady FilevaultConditionType = "FilevaultReady"
)

func NewFilevaultReadyCondition() metav1.Condition {
	return metav1.Condition{
		Type:    string(FilevaultReady),
		Status:  metav1.ConditionTrue,
		Reason:  "DependenciesSetup",
		Message: "all dependencies are set and initiated",
	}
}

// FilevaultSpec defines the desired state of Filevault
type FilevaultSpec struct {
	//+kubebuilder:default:="localhost"
	Host string `json:"host,omitempty"`

	//+kubebuilder:default:=8080
	ContainerPort int `json:"containerPort,omitempty"`

	//+kubebuilder:default:="/mnt/filevault"
	MountPath string `json:"mountPath,omitempty"`

	// Name of the PersistentVolumeClaim
	// to use for the filevault server
	ClaimName string `json:"claimName,omitempty"`

	// +kubebuilder:validation:optional
	// +kubebuilder:default:=latest
	Version string `json:"version,omitempty"`
}

// FilevaultStatus defines the observed state of Filevault
type FilevaultStatus struct {
	// Name of the pod which is hosting
	// the filevault server
	ServerName string `json:"serverName,omitempty"`

	// Name of the PersistentVolumeClaim
	// to use for the filevault server
	ClaimName string `json:"claimName,omitempty"`

	// Current used version of filevault
	Version string `json:"version,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (n *FilevaultStatus) Default() {
	const u = "unknown"
	if n.ServerName == "" {
		n.ServerName = u
	}
	if n.ClaimName == "" {
		n.ClaimName = u
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Filevault is the Schema for the filevaults API
type Filevault struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FilevaultSpec   `json:"spec,omitempty"`
	Status FilevaultStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FilevaultList contains a list of Filevault
type FilevaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Filevault `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Filevault{}, &FilevaultList{})
}
