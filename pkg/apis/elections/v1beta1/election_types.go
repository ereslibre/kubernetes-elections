/*
Copyright 2018 Rafael Fernández López.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ElectionSpec defines the desired state of Election
type ElectionSpec struct {
	Options map[string][]string `json:"options,omitempty"`
}

// ElectionStatus defines the observed state of Election
type ElectionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Election is the Schema for the elections API
// +k8s:openapi-gen=true
type Election struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElectionSpec   `json:"spec,omitempty"`
	Status ElectionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElectionList contains a list of Election
type ElectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Election `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Election{}, &ElectionList{})
}
