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
	"github.com/guilhem/headscale-operator/pkg/headscale"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerSpec defines the desired state of Server
type ServerSpec struct {
	//+optional
	Version string `json:"version"`

	//+optional
	//+kubebuilder:default=false
	Debug bool `json:"debug"`

	//+optional
	Issuer string `json:"issuer,omitempty"`

	//+optional
	GrpcServiceName string `json:"grpcServiceName,omitempty"`

	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Config headscale.Config `json:"config,omitempty"`

	// +kubebuilder:validation:Format=hostname
	// +kubebuilder:validation:Required
	Host string `json:"host,omitempty"`

	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Ingress *networkingv1.Ingress `json:"ingress,omitempty"`
}

// ServerStatus defines the observed state of Server
type ServerStatus struct {
	GrpcAddress string `json:"grpcAddress,omitempty"`

	DeploymentName string `json:"deploymentName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Server is the Schema for the servers API
type Server struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerSpec   `json:"spec,omitempty"`
	Status ServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServerList contains a list of Server
type ServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Server `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Server{}, &ServerList{})
}
