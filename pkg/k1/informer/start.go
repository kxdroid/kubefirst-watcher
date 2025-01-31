package informer

import (
	"fmt"
	"os"
	"time"

	"github.com/kxdroid/k8s-watcher/pkg/k1/crd"
	"github.com/kxdroid/k8s-watcher/pkg/k1/v1beta1"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// StartWatcher - starts watcher tooling
var logger *zap.Logger

const (
	StatusCompleted string = "Satisfied"
	StatusTimeout   string = "Timeout"
)

func StartCRDWatcher(clientSet *kubernetes.Clientset, clientCrd *crd.CRDClient, loggerIn *zap.Logger) error {
	logger = clientCrd.Logger
	myCRD, err := clientCrd.GetCRD()
	if err != nil {
		return fmt.Errorf("Error getting CRD")
	}
	//Setup channels
	interestingPods := make(chan Condition)
	defer close(interestingPods)
	stopper := make(chan struct{})
	defer close(stopper)

	//Process Conditions into goals
	exitScenario, exitScenarioState, _ := loadExitScenarioFromCRD(myCRD.Spec)
	logger.Info(fmt.Sprintf("%#v", exitScenario))
	logger.Info(fmt.Sprintf("%#v", exitScenarioState))
	//Process Conditions into watchers
	//Start Goals tracker
	go checkConditions(exitScenarioState, clientCrd, interestingPods, stopper)
	//Start Watchers
	//go WatchSecrets(exitScenario.Secrets, interestingPods, stopper)
	startWatchers(clientSet, exitScenario, interestingPods, stopper)
	//Check Current State - to catch events pre-informers are started
	time.Sleep(time.Duration(exitScenario.Timeout) * time.Second)
	logger.Error("Timeout - Fail to match conditions")
	clientCrd.UpdateStatus(StatusTimeout)
	return fmt.Errorf("timeout - Failed to meet exit condition")
}

func startWatchers(clientSet *kubernetes.Clientset, exitScenario *v1beta1.WatcherSpec, interestingEvents chan Condition, stopper chan struct{}) {
	factory := informers.NewSharedInformerFactory(clientSet, 0)
	if len(exitScenario.Pods) > 0 {
		go WatchPods(exitScenario.Pods, interestingEvents, stopper, factory.Core().V1().Pods().Informer())
	}
	if len(exitScenario.ConfigMaps) > 0 {
		go WatchBasic(exitScenario.ConfigMaps, interestingEvents, stopper, factory.Core().V1().ConfigMaps().Informer())
	}
	if len(exitScenario.Secrets) > 0 {
		go WatchBasic(exitScenario.Secrets, interestingEvents, stopper, factory.Core().V1().Secrets().Informer())

	}
	if len(exitScenario.Services) > 0 {
		go WatchBasic(exitScenario.Services, interestingEvents, stopper, factory.Core().V1().Services().Informer())
	}
	if len(exitScenario.Jobs) > 0 {
		go WatchJobs(exitScenario.Jobs, interestingEvents, stopper, factory.Batch().V1().Jobs().Informer())
	}
	if len(exitScenario.Deployments) > 0 {
		go WatchDeployments(exitScenario.Deployments, interestingEvents, stopper, factory.Apps().V1().Deployments().Informer())
	}
	if len(exitScenario.StatefulSets) > 0 {
		go WatchStatefulSets(exitScenario.StatefulSets, interestingEvents, stopper, factory.Apps().V1().StatefulSets().Informer())
	}

	logger.Info("All conditions checkers started")
}

func checkConditions(goal *ExitScenarioState, clientCrd *crd.CRDClient, in <-chan Condition, stopper chan struct{}) {
	logger.Debug("Started Listener")
	logger.Info(fmt.Sprintf("%#v", goal))
	pendingConditions := len(goal.Conditions)

	for {
		receivedResource := <-in
		fmt.Println("\nInteresting Resource:", receivedResource)
		for key, currentCondition := range goal.Conditions {
			fmt.Println("Key:", key, "Value:", currentCondition)
			if currentCondition.ID == receivedResource.ID &&
				currentCondition.Met == false {
				goal.Conditions[key].Met = true
				logger.Debug("\n Condition  Met:" + fmt.Sprintf("%#v", currentCondition))
				pendingConditions = pendingConditions - 1
				logger.Debug("\n Pending Conditions:" + fmt.Sprintf("%#v", pendingConditions))
				break
			}
		}

		fmt.Println("\n State of Conditions:", goal.Conditions)

		if pendingConditions < 1 {
			logger.Debug("All required objects found, ready to close waiting channels")
			logger.Debug(fmt.Sprintf("%#v", goal.Conditions))
			clientCrd.UpdateStatus(StatusCompleted)
			os.Exit(int(goal.Exit))
		}
	}
}
func loadExitScenarioFromCRD(watcherSpec v1beta1.WatcherSpec) (*v1beta1.WatcherSpec, *ExitScenarioState, error) {

	exitScenarioState, err := processExitScenario(&watcherSpec)
	if err != nil {
		logger.Info(fmt.Sprintf("Error processing Scenario State: %v", err))
		return nil, nil, err
	}
	logger.Info(fmt.Sprintf("Log processing exitScenarioState: %v", exitScenarioState))
	return &watcherSpec, exitScenarioState, nil
}

func processExitScenario(exitScenario *v1beta1.WatcherSpec) (*ExitScenarioState, error) {
	exitScenarioState := &ExitScenarioState{}
	exitScenarioState.Exit = exitScenario.Exit
	exitScenarioState.Timeout = exitScenario.Timeout
	exitScenarioState.Conditions = []Condition{}

	id := 1
	for k, _ := range exitScenario.Pods {
		exitScenario.Pods[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Pods[k])})
		id++
	}
	for k, _ := range exitScenario.ConfigMaps {
		exitScenario.ConfigMaps[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.ConfigMaps[k])})
		id++
	}
	for k, _ := range exitScenario.Secrets {
		exitScenario.Secrets[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Secrets[k])})
		id++
	}
	for k, _ := range exitScenario.Services {
		exitScenario.Services[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Services[k])})
		id++
	}
	for k, _ := range exitScenario.Jobs {
		exitScenario.Jobs[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Jobs[k])})
		id++
	}
	for k, _ := range exitScenario.Deployments {
		exitScenario.Deployments[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Deployments[k])})
		id++
	}
	for k, _ := range exitScenario.StatefulSets {
		exitScenario.StatefulSets[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.StatefulSets[k])})
		id++
	}
	for k, _ := range exitScenario.Watchers {
		exitScenario.Watchers[k].ID = id
		exitScenarioState.Conditions = append(exitScenarioState.Conditions, Condition{ID: id, Met: false, Description: fmt.Sprintf("%#v", exitScenario.Watchers[k])})
		id++
	}
	return exitScenarioState, nil
}
