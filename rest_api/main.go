package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"app/rest_api/logging"
	wikiSteps "app/rest_api/wiki_steps"

	"github.com/gorilla/mux"
)

// var start = "https://en.wikipedia.org/wiki/Friedrich_Merz"
// var finish = "https://en.wikipedia.org/wiki/Machine_translation"

var (
	App             Application
	WikiStepService *wikiSteps.WikiSteps
)

type Application struct {
	log logging.Logger
}

func initApplication() {
	App = Application{
		logging.NewZerologAdapter(),
	}
}

//curl -X GET "http://localhost:8000/wikisteps?start=https%3A%2F%2Fen.wikipedia.org%2Fwiki%2FFriedrich_Merz&target=https%3A%2F%2Fen.wikipedia.org%2Fwiki%2FMachine_translation&steps=5"
func invokeWikiStepService(w http.ResponseWriter, r *http.Request) {
	var err error
	quaryParams := r.URL.Query()
	start := quaryParams.Get("start")
	target := quaryParams.Get("target")
	steps := quaryParams.Get("steps")

	if start, err = url.QueryUnescape(start); err != nil {
		http.Error(w, "One or more required query parameters is invalid", http.StatusBadRequest)
	}

	if target, err = url.QueryUnescape(target); err != nil {
		http.Error(w, "One or more required query parameters is invalid", http.StatusBadRequest)
	}

	if start == "" || target == "" || steps == "" {
		http.Error(w, "Missing one or more required query parameters", http.StatusBadRequest)
	}

	stepsNum, err := strconv.Atoi(steps)
	if err != nil {
		http.Error(w, "One or more required query parameters is invalid", http.StatusBadRequest)
	}

	paths, err := WikiStepService.FindValidPaths(start, target, stepsNum)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error occored when finding valid paths: %s", err.Error()), http.StatusInternalServerError)
	}

	response := map[string]interface{}{
		"start":      start,
		"target":     target,
		"steps":      stepsNum,
		"validPaths": paths,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	initApplication()
	App.log.Info("Application initialized!")

	maxSteps := 7
	numWorkers := 25
	stepTimeout := 30 * time.Second

	WikiStepService = wikiSteps.NewWikistepsService(App.log, maxSteps, stepTimeout, numWorkers)

	router := mux.NewRouter()

	router.HandleFunc("/wikisteps", invokeWikiStepService).Methods("GET")

	App.log.Fatal(http.ListenAndServe(":8000", router).Error())
}
