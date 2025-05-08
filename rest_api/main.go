package main

import (
	"fmt"
	"strings"
	"time"

	"app/rest_api/logging"
	wikiSteps "app/rest_api/wiki_steps"
)

var start = "https://en.wikipedia.org/wiki/Friedrich_Merz"
var finish = "https://en.wikipedia.org/wiki/Machine_translation"

var (
	App Application
)

type Application struct {
	log logging.Logger
}

func initApplication() {
	App = Application{
		logging.NewZerologAdapter(),
	}
}

func main() {
	initApplication()
	App.log.Info("Application initialized!")

	maxSteps := 7
	numWorkers := 25
	stepTimeout := 30 * time.Second

	serviceWikisteps := wikiSteps.NewWikistepsService(App.log, maxSteps, stepTimeout, numWorkers)
	results, err := serviceWikisteps.FindValidPaths(start, finish, 5)
	if err != nil {
		App.log.Error(err.Error())
	}

	if results == nil {
		App.log.Info("Didnt get results")
	} else {
		App.log.Info(fmt.Sprintf("Found %d Valid Paths:", len(results)))
		for _, path := range results {
			App.log.Info(fmt.Sprintf("Steps: %d, Path: [%s]", len(path)-1, strings.Join(path, ", ")))
		}
	}
}
