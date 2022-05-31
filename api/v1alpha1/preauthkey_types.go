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

// PreAuthKeySpec defines the desired state of PreAuthKey
type PreAuthKeySpec struct {
	Namespace string `json:"namespace"`
	Reusable  bool   `json:"reusable"`
	Ephemeral bool   `json:"ephemeral"`
	Duration  string `json:"duration"`
}

// PreAuthKeyStatus defines the observed state of PreAuthKey
type PreAuthKeyStatus struct {
	Used       bool   `json:"used"`
	ID         string `json:"id"`
	Expiration string `json:"expiration"`
	CreatedAt  string `json:"createdAt"`
	Key        string `json:"key"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PreAuthKey is the Schema for the preauthkeys API
type PreAuthKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreAuthKeySpec   `json:"spec,omitempty"`
	Status PreAuthKeyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PreAuthKeyList contains a list of PreAuthKey
type PreAuthKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreAuthKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreAuthKey{}, &PreAuthKeyList{})
}
