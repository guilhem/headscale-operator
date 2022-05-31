/*
Copyright 2022 Guilhem Lettron.

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

// NamespaceSpec defines the desired state of Namespace
type NamespaceSpec struct {
	Server string `json:"server"`
}

// NamespaceStatus defines the observed state of Namespace
type NamespaceStatus struct {
	Created bool `json:"created"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Namespace is the Schema for the namespaces API
type Namespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceSpec   `json:"spec,omitempty"`
	Status NamespaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NamespaceList contains a list of Namespace
type NamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Namespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Namespace{}, &NamespaceList{})
}
