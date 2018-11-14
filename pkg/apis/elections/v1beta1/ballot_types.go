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

// BallotSpec defines the desired state of Ballot
type BallotSpec struct {
	Election string            `json:"election,omitempty"`
	Answers  map[string]string `json:"answers,omitempty"`
}

// BallotStatus defines the observed state of Ballot
type BallotStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Ballot is the Schema for the ballots API
// +k8s:openapi-gen=true
type Ballot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BallotSpec   `json:"spec,omitempty"`
	Status BallotStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BallotList contains a list of Ballot
type BallotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ballot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ballot{}, &BallotList{})
}
