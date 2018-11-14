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

package election

import (
	"context"
	"log"

	electionsv1beta1 "github.com/ereslibre/kubernetes-elections/pkg/apis/elections/v1beta1"
	"github.com/ereslibre/kubernetes-elections/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Election Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileElection{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("election-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Election
	err = c.Watch(&source.Kind{Type: &electionsv1beta1.Election{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileElection{}

// ReconcileElection reconciles a Election object
type ReconcileElection struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Election object and makes changes based on the state read
// and what is in the Election.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=elections.containersevent.suse.com,resources=elections,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileElection) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Election instance
	instance := &electionsv1beta1.Election{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Reconcile deletion
	finalizerName := "elections.finalizers.suse.com"
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(instance.ObjectMeta.Finalizers, finalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}
	} else {
		if utils.ContainsString(instance.ObjectMeta.Finalizers, finalizerName) {
			ballotList := &electionsv1beta1.BallotList{}
			if err := r.List(context.TODO(), client.InNamespace(instance.ObjectMeta.Namespace), ballotList); err == nil {
				log.Printf("Started deleting ballots matching election %s", instance.ObjectMeta.Name)
				for _, ballot := range ballotList.Items {
					if ballot.Spec.Election == instance.ObjectMeta.Name {
						log.Printf("Deleting ballot %s\n", ballot.ObjectMeta.Name)
						if ballot.ObjectMeta.DeletionTimestamp.IsZero() {
							if err := r.Delete(context.TODO(), &ballot); err != nil {
								log.Printf("Could not delete ballot %s, reason: %s\n", ballot.ObjectMeta.Name, err)
								return reconcile.Result{}, err
							} else {
								log.Printf("Ballot %s deleted\n", ballot.ObjectMeta.Name)
							}
						}
						ballot.ObjectMeta.Finalizers = []string{}
						if err := r.Update(context.Background(), &ballot); err != nil {
							log.Printf("Could not update ballot object %s, reason: %s\n", ballot.ObjectMeta.Name, err)
							return reconcile.Result{}, err
						} else {
							log.Printf("Finalizers removed from ballot object %s", instance.ObjectMeta.Name)
						}
					}
				}
				log.Printf("Done deleting ballots matching election %s", instance.ObjectMeta.Name)
			}

			electionResultName := instance.ObjectMeta.Name + "-results"
			electionResult := &electionsv1beta1.Result{}
			if err := r.Get(context.TODO(), types.NamespacedName{Name: electionResultName, Namespace: instance.ObjectMeta.Namespace}, electionResult); err == nil {
				if err := r.Delete(context.TODO(), electionResult); err != nil {
					return reconcile.Result{}, err
				}
			}

			instance.ObjectMeta.Finalizers = utils.RemoveString(instance.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}

		return reconcile.Result{}, nil
	}

	// Reconcile results
	results := make(map[string]map[string]uint32)
	for question, options := range instance.Spec.Options {
		results[question] = make(map[string]uint32)
		for _, option := range options {
			results[question][option] = 0
		}
	}

	electionsResult := &electionsv1beta1.Result{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-results",
			Namespace: instance.Namespace,
		},
		Spec: electionsv1beta1.ResultSpec{
			Results: results,
			Ballots: []string{},
		},
	}

	if err := controllerutil.SetControllerReference(instance, electionsResult, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundElectionsResult := &electionsv1beta1.Result{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: electionsResult.Name, Namespace: electionsResult.Namespace}, foundElectionsResult)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating election results %s/%s\n", electionsResult.Namespace, electionsResult.Name)
		err = r.Create(context.TODO(), electionsResult)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
