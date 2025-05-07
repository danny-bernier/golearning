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
	results, err := serviceWikisteps.FindSteps(start, finish, 5)
	if err != nil {
		App.log.Error(err.Error())
	}

	if results == nil {
		fmt.Print("Didnt get results")
	} else {
		fmt.Printf("\nFound %d Valid Paths:\n", len(results))
		for _, path := range results {
			fmt.Printf("Steps: %d, Path: [%s]\n", len(path)-1, strings.Join(path, ", "))
		}
	}
}
