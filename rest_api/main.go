package main

import (
	"fmt"
	"strings"

	"app/rest_api/logging"
	wikiSteps "app/rest_api/wiki_steps"
)

var start = "https://en.wikipedia.org/wiki/Stevia"
var finish = "https://en.wikipedia.org/wiki/Acne"

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

	serviceWikisteps := wikiSteps.InitWikistepsService(App.log)
	results, err := serviceWikisteps.FindSteps(start, finish, 3)
	if err != nil {
		App.log.Error(err.Error())
	}

	if results == nil {
		App.log.Info("Didnt get results")
	} else {
		App.log.Info(fmt.Sprintf("\nFound %d Valid Paths:\n", len(results)))
		for _, path := range results {
			App.log.Info(fmt.Sprintf("Steps: %d, Path: [%s]\n", len(path)-1, strings.Join(path, ", ")))
		}
	}
}
