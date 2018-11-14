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

package ballot

import (
	"context"
	"errors"
	"log"

	electionsv1beta1 "github.com/ereslibre/kubernetes-elections/pkg/apis/elections/v1beta1"
	"github.com/ereslibre/kubernetes-elections/pkg/utils"
	machineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Ballot Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBallot{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ballot-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Ballot
	err = c.Watch(&source.Kind{Type: &electionsv1beta1.Ballot{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileBallot{}

// ReconcileBallot reconciles a Ballot object
type ReconcileBallot struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Ballot object and makes changes based on the state read
// and what is in the Ballot.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=elections.containersevent.suse.com,resources=ballots,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileBallot) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Ballot instance
	instance := &electionsv1beta1.Ballot{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if machineryerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Reconcile deletion
	finalizerName := "ballots.finalizers.suse.com"
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(instance.ObjectMeta.Finalizers, finalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}
	} else {
		if utils.ContainsString(instance.ObjectMeta.Finalizers, finalizerName) {
			election, err := fetchElection(r, instance.Spec.Election, instance.ObjectMeta.Namespace)
			if err != nil {
				log.Printf("Could not find election object %s, nothing to reconcile", instance.Spec.Election)
				return reconcile.Result{}, nil
			}

			electionResult, err := fetchElectionResult(r, instance.Spec.Election, instance.ObjectMeta.Namespace)
			if err != nil {
				log.Printf("Could not find election result object for election %s, nothing to reconcile", instance.Spec.Election)
				return reconcile.Result{}, nil
			}

			if election.ObjectMeta.DeletionTimestamp.IsZero() {
				if err := forgetBallot(electionResult, instance.ObjectMeta.Name, instance.Spec.Answers); err != nil {
					return reconcile.Result{}, nil
				}

				err = r.Update(context.TODO(), electionResult)
				if err != nil {
					return reconcile.Result{}, nil
				}

				instance.ObjectMeta.Finalizers = utils.RemoveString(instance.ObjectMeta.Finalizers, finalizerName)
				if err := r.Update(context.Background(), instance); err != nil {
					return reconcile.Result{Requeue: true}, nil
				}
			} else {
				log.Printf("Skipping election results updates, since election is being removed")
			}

			return reconcile.Result{}, nil
		}
	}

	electionResult, err := fetchElectionResult(r, instance.Spec.Election, instance.ObjectMeta.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := recordBallot(electionResult, instance.ObjectMeta.Name, instance.Spec.Answers); err != nil {
		return reconcile.Result{}, err
	}

	err = r.Update(context.TODO(), electionResult)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func fetchElection(r *ReconcileBallot, electionName string, namespace string) (*electionsv1beta1.Election, error) {
	election := &electionsv1beta1.Election{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: electionName, Namespace: namespace}, election)
	if err != nil {
		if machineryerrors.IsNotFound(err) {
			log.Printf("Could not find election object: %s", electionName)
			return nil, errors.New("Could not find election object")
		}
		return nil, err
	}
	return election, nil
}

func fetchElectionResult(r *ReconcileBallot, electionName string, namespace string) (*electionsv1beta1.Result, error) {
	electionResultName := electionName + "-results"
	electionResult := &electionsv1beta1.Result{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: electionResultName, Namespace: namespace}, electionResult)
	if err != nil {
		if machineryerrors.IsNotFound(err) {
			log.Printf("Could not find election result object: %s", electionResultName)
			return nil, errors.New("Could not find election result object")
		}
		return nil, err
	}
	return electionResult, nil
}

func recordBallot(electionResult *electionsv1beta1.Result, ballotName string, answers map[string]string) error {
	for _, ballot := range electionResult.Spec.Ballots {
		if ballot == ballotName {
			log.Printf("Not recording ballot %s, was already recorded\n", ballotName)
			return nil
		}
	}

	if len(electionResult.Spec.Ballots) == 0 {
		electionResult.Spec.Ballots = []string{ballotName}
	} else {
		electionResult.Spec.Ballots = append(electionResult.Spec.Ballots, ballotName)
	}

	log.Printf("Recording ballot %s\n", ballotName)
	for question, answer := range answers {
		electionResult.Spec.Results[question][answer] += 1
	}

	return nil
}

func forgetBallot(electionResult *electionsv1beta1.Result, ballotName string, answers map[string]string) error {
	ballotFound := false

	for _, ballot := range electionResult.Spec.Ballots {
		if ballot == ballotName {
			ballotFound = true
			break
		}
	}

	if !ballotFound {
		log.Printf("Not forgetting ballot %s, was already unknown\n", ballotName)
		return nil
	}

	electionResult.Spec.Ballots = utils.RemoveElementFromList(electionResult.Spec.Ballots, ballotName)

	log.Printf("Forgetting ballot %s\n", ballotName)
	for question, answer := range answers {
		log.Printf("Forgetting answer %s: %s\n", question, answer)
		electionResult.Spec.Results[question][answer] -= 1
	}

	return nil
}
