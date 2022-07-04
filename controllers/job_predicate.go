package controllers

import (
	kbatch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// JobStatusChangePredicate triggers an update event
// when a Job status changes.
type JobStatusChangePredicate struct {
	predicate.Funcs
}

func (JobStatusChangePredicate) Create(e event.CreateEvent) bool {
	return false
}

func (p JobStatusChangePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		return false
	}

	newJob, ok := e.ObjectNew.(*v1.Job)
	if !ok {
		return false
	}

	if newJob.ObjectMeta.DeletionTimestamp != nil {
		return false
	}

	finished, _ := IsJobFinished(newJob)
	return finished
}

func (p JobStatusChangePredicate) Delete(e event.DeleteEvent) bool {
	return false
}

func IsJobFinished(job *kbatch.Job) (bool, kbatch.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}
