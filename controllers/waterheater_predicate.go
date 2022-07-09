package controllers

import (
	v1 "github.com/emilgelman/custom-k8s-api-/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type WaterHeaterStatusChangePredicate struct {
	predicate.Funcs
}

func (WaterHeaterStatusChangePredicate) Create(e event.CreateEvent) bool {
	return true
}

func (p WaterHeaterStatusChangePredicate) Update(e event.UpdateEvent) bool {
	wh, ok := e.ObjectNew.(*v1.WaterHeater)
	if !ok {
		return false
	}
	if wh.Spec.Temperature == wh.Status.Temperature {
		return false
	}
	return wh.Status.Mode == v1.Idle
}

func (p WaterHeaterStatusChangePredicate) Delete(e event.DeleteEvent) bool {
	return false
}
