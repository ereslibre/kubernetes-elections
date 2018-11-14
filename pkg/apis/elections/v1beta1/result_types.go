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

// ResultSpec defines the desired state of Result
type ResultSpec struct {
	Results map[string]map[string]uint32 `json:"results,omitempty"`
	Ballots []string                     `json:"ballots,omitempty"`
}

// ResultStatus defines the observed state of Result
type ResultStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Result is the Schema for the results API
// +k8s:openapi-gen=true
type Result struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResultSpec   `json:"spec,omitempty"`
	Status ResultStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResultList contains a list of Result
type ResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Result `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Result{}, &ResultList{})
}
