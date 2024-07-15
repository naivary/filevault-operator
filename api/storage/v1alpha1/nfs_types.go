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

type NFSConditionType string

const (
    NFSReady NFSConditionType = "NFSReady"
)

func NewNFSReadyCondition() metav1.Condition {
    return metav1.Condition{
        Type: string(NFSReady),
        Status: metav1.ConditionTrue,
        Reason: "ServerReady",
        Message: "nfs server is ready for request",
    }
}

// NFSSpec defines the desired state of NFS
type NFSSpec struct {
	//+kubebuilder:default:="10Gi"
	Capacity string `json:"capacity,omitempty"`

	//+kubebuilder:validation:required
	ClaimName string `json:"claimName,omitempty"`
}

// NFSStatus defines the observed state of NFS
type NFSStatus struct {
	// Name of the pod hosting the NFS server
	ServerName string `json:"serverName,omitempty"`

	// Name of the service which is allowing
	// traffic to the NFS server
	ServiceName string `json:"serviceName,omitempty"`

	// Name of the PersistentVolume connected to the NFS server
	VolumeName string `json:"volumeName,omitempty"`

	// Name of the PersistentVolumeClaim connected
	// to the PersistentVolume
	ClaimName string `json:"claimName,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NFS is the Schema for the nfs API
type NFS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NFSSpec   `json:"spec,omitempty"`
	Status NFSStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NFSList contains a list of NFS
type NFSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NFS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NFS{}, &NFSList{})
}
