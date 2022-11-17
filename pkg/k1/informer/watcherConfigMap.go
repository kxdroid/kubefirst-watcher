package informer

import (
	"fmt"

	"k8s.io/client-go/tools/cache"
)

// TODO: Make this more generic

func WatchBasic(conditions []BasicConfigurationCondition, matchConditions chan Condition, stopper chan struct{}, informer cache.SharedIndexInformer) {
	logger.Debug(fmt.Sprintf("Started Wacher for %#v", conditions))

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			mObj := obj.(BasicK8s)
			logger.Debug(fmt.Sprintf("New Pod updated:", mObj.GetName(), mObj.GetNamespace()))
			checkMatchBasicConfigurationCondition(&BasicConfiguration{Namespace: mObj.GetNamespace(), Name: mObj.GetName()}, conditions, matchConditions)

		},
		UpdateFunc: func(old, new interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			newObj := new.(BasicK8s)
			logger.Debug(fmt.Sprintf("Pod updated:", newObj.GetName(), newObj.GetNamespace()))
			checkMatchBasicConfigurationCondition(&BasicConfiguration{Namespace: newObj.GetNamespace(), Name: newObj.GetName()}, conditions, matchConditions)
		},
		DeleteFunc: func(obj interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			mObj := obj.(BasicK8s)
			logger.Debug(fmt.Sprintf("New Pod deleted from Store: %s", mObj.GetName()))
		},
	})
	informer.Run(stopper)
}

func checkMatchBasicConfigurationCondition(obj *BasicConfiguration, conditions []BasicConfigurationCondition, matchCondition chan Condition) {
	//check on conditions list if there is a match
	for k, _ := range conditions {
		if obj.Namespace == conditions[k].Namespace &&
			obj.Name == conditions[k].Name {
			logger.Debug(fmt.Sprintf("Interest BasicConfigurationCondition event found -  status: %#v", obj))
			foundCondition := Condition{
				ID:  conditions[k].ID,
				Met: true,
			}
			logger.Debug(fmt.Sprintf("Sending Condition -  status:  %#v ", foundCondition))
			matchCondition <- foundCondition
			//Remove Condition found
			//https://github.com/golang/go/wiki/SliceTricks
			// conditions = append(conditions[:k], conditions[k+1:]...)
			// it may fail on nil scenarios - extra checks needed
			//This need to be global, as this checks may run in parallel.
			//TODO: need to find an list that is thread safe
			logger.Debug(fmt.Sprintf("Remaning Condition -  status:  %#v ", foundCondition))

		}
	}
}