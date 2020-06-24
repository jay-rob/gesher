package proxyvalidatingtype

import (
	"context"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	typeFinalizer = "type.finalizer.gesher"
)

func act(c client.Client, state *analyzedState, logger logr.Logger) error {
	if state.update {
		err := manageWebhookConfig(c, state, logger)
		if err != nil {
			return err
		}
	} else {
		logger.Info("Skipping cluster webhook update")
	}

	var fullChange bool
	ret := manageFinalizer(state, logger)
	fullChange = ret || fullChange

	var statusChange bool
	ret = manageGeneration(state, logger)
	statusChange = ret || statusChange

	if fullChange {
		logger.Info("doing full update")
		err := c.Update(context.TODO(), state.customResource)
		if err != nil {
			logger.Error(err, "failed to do full update")
			return err
		}
	} else if statusChange {
		logger.Info("doing status update")
		err := c.Status().Update(context.TODO(), state.customResource)
		if err != nil {
			logger.Error(err, "failed to do status update")
			return err
		}
	}

	proxyTypeData = state.newProxyTypeData

	return nil
}

func manageGeneration(state *analyzedState, logger logr.Logger) bool {
	var ret bool

	if state.customResource.Status.ObservedGeneration != state.customResource.Generation {
		logger.Info("updating observed generation in status")
		state.customResource.Status.ObservedGeneration = state.customResource.Generation
		ret = true
	}

	return ret
}

func manageFinalizer(state *analyzedState, logger logr.Logger) bool {
	var ret bool

	switch state.delete {
	case false:
		if !containsString(state.customResource.ObjectMeta.Finalizers, typeFinalizer) {
			logger.Info("adding finalizer")
			state.customResource.ObjectMeta.Finalizers = append(state.customResource.ObjectMeta.Finalizers, typeFinalizer)
			ret = true
		}
	case true:
		if containsString(state.customResource.ObjectMeta.Finalizers, typeFinalizer) {
			logger.Info("removing finalizer")
			state.customResource.ObjectMeta.Finalizers = removeString(state.customResource.ObjectMeta.Finalizers, typeFinalizer)
			ret = true
		}
	}

	return ret
}

func manageWebhookConfig(c client.Client, state *analyzedState, logger logr.Logger) error {
	if state.create {
		logger.Info("creating webhook")
		err := c.Create(context.TODO(), state.webhook)
		if err != nil {
			logger.Error(err, "failed to create managed cluster webhook")
			return err
		}
	} else {
		logger.Info("updating webhook")
		err := c.Update(context.TODO(), state.webhook)
		if err != nil {
			logger.Error(err, "failed to update managed cluster webhook")
			return err
		}
	}

	return nil
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
