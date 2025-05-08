package wikiSteps

import (
	"app/rest_api/logging"
	"app/rest_api/util"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"slices"

	"golang.org/x/net/html"
)

const (
	WikipediaDomain string = "https://en.wikipedia.org"
	WikiPrefix      string = "/wiki/"
)

var (
	wikiBlockPrefixList []string       = []string{"/wiki/Main_Page"}
	reWikiBlockPrefix   *regexp.Regexp = regexp.MustCompile(`^/wiki/\w+:.*`)
	httpSem             chan struct{}  = make(chan struct{}, 10) // semaphore for limiting the number of requests to wikipedia
)

type WikiSteps struct {
	log          logging.Logger
	httpClient   *http.Client
	maxSteps     int
	stepsTimeout time.Duration
	numWorkers   int
}

type wikiStepJob struct {
	Path              []string
	LastPathUrls      []string
	NumStepsRemaining int
	CompletedCh       chan<- wikiStepJob
}

func NewWikistepsService(log logging.Logger, maxSteps int, stepsTimeout time.Duration, numWorkers int) *WikiSteps {
	return &WikiSteps{
		log,
		&http.Client{},
		maxSteps,
		stepsTimeout,
		numWorkers,
	}
}

func (w WikiSteps) FindValidPaths(start string, target string, steps int) ([][]string, error) {
	if !isValidWikiStepUrl(start, target) {
		return nil, fmt.Errorf("start or target is invalid")
	}

	if steps > w.maxSteps || steps < 0 {
		return nil, fmt.Errorf("steps cannot be negative")
	}

	w.log.Trace("Initializing resources...")
	results := make([][]string, 0) // to aggregate valid results
	exitCh := make(chan struct{})
	errCh := make(chan error)
	jobCh := make(chan wikiStepJob, 100*w.numWorkers)
	completedCh := make(chan wikiStepJob, 100*w.numWorkers)

	w.log.Trace("Initializing starting job...")
	var startingJob wikiStepJob
	path := make([]string, 1)
	path[0] = start
	startingJob.Path = path
	startingJob.NumStepsRemaining = steps
	startingJob.CompletedCh = completedCh
	jobCh <- startingJob

	w.log.Trace("Initializing wait group...")
	var wg sync.WaitGroup
	wg.Add(w.numWorkers)

	w.log.Trace("Starting workers...")
	for i := 0; i < w.numWorkers; i += 1 {
		workerName := util.RandomName()
		go w.wikiStepWorker(util.RandomName(), jobCh, exitCh, errCh, &wg)
		w.log.Debug(fmt.Sprintf("Started worker %s", workerName))
	}

	timeout := time.After(w.stepsTimeout)
	for {
		select {
		case <-timeout:
			w.log.Info(fmt.Sprintf("WikiSteps timed out after %.0f seconds, signaling exit...", w.stepsTimeout.Seconds()))
			close(exitCh)
			break

		case err := <-errCh:
			w.log.Error(fmt.Sprintf("WikiSteps is signaling exit after encountering an error %w", err))
			close(exitCh)
			break

		case completedJob := <-completedCh:
			for _, url := range completedJob.LastPathUrls {
				if url == target {
					w.log.Debug(fmt.Sprintf("WikiSteps found a valid path to the target"))
					results = append(results, append(completedJob.Path, url))

				} else if completedJob.NumStepsRemaining-1 > 0 {
					w.log.Trace(fmt.Sprintf("WikiSteps did not find a valid path to target yet, resubmitting job"))
					var j wikiStepJob
					j.Path = append(completedJob.Path, url)
					j.NumStepsRemaining = completedJob.NumStepsRemaining - 1
					j.CompletedCh = completedCh

				} else {
					w.log.Trace("WikiSteps dead end! A path ran out of steps")
				}
			}

		}
	}
	// push jobs onto the job channel using a supervisor
	//  - recieves on the completed job channels
	//  - update the step count
	//  - check if target has been found
	//  - once timeout triggered, calls exitCh and returns results
	// workers pull jobs off the channel
	//	- use the callback to return the updated job
	//  - workers keep chugging until the exitCh is used

	w.log.Info(fmt.Sprintf("Started stepping! start: %s, target: %s, steps: %d, workers: %d, timeout: %d", start, target, steps, w.numWorkers, w.stepsTimeout))
	go w.step(path, target, steps, exitCh, resCh)

	// monitoring for results
	for {
		select {
		case r, ok := <-resCh:
			results = append(results, r)
			if !ok {
				util.SafeClose(exitCh)
				return results, nil
			}
		case <-timeout:
			util.SafeClose(exitCh)
			return results, fmt.Errorf("execution timed out")
		}
	}
}

func (w WikiSteps) wikiStepWorker(name string, jobCh <-chan wikiStepJob, exitCh <-chan struct{}, errCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-exitCh:
			w.log.Debug(fmt.Sprintf("Worker %s received exit signal, closing..."))
			return
		case job := <-jobCh:
			wg.Add(1)
			w.log.Debug(fmt.Sprintf("Worker %s started a new job"))
			job, err := w.doWikiStepJob(name, job, exitCh)
			if err != nil {
				errCh <- err
				continue
			}
			w.log.Debug(fmt.Sprintf("Worker %s completed a job"))
		}
	}
}

func (w WikiSteps) doWikiStepJob(workerName string, job wikiStepJob, exitCh <-chan struct{}) (wikiStepJob, error) {
	if len(job.Path) == 0 {
		return job, fmt.Errorf("worker %s's path slice is empty", workerName) // should never happen
	}

	nextUrl := job.Path[len(job.Path)-1] // isolate the next URL to fetch data for

	// requesting data from Wikipedia
	w.log.Debug(fmt.Sprintf("Worker %s is waiting for semaphore aquisition...", workerName))
	select {
	case httpSem <- struct{}{}:
		w.log.Debug(fmt.Sprintf("Worker %s aquired semaphore.", workerName))
		defer func() { <-wikiReqSemaphore }()
	case <-exitCh:
		w.log.Debug(fmt.Sprintf("Worker %s recieved exit signal while waiting for semaphore aquisition.", workerName))
		return job, nil
	}

	w.log.Trace(fmt.Sprintf("Worker %s is building GET request for URL %s", workerName, nextUrl))
	req, err := http.NewRequest("GET", nextUrl, nil)
	if err != nil {
		return job, fmt.Errorf("worker %s encountered an error when building GET request for URL: %s; %w", workerName, nextUrl, err)
	}

	w.log.Debug(fmt.Sprintf("Worker %s is executing a GET request for URL %s", workerName, nextUrl))
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return job, fmt.Errorf("worker %s encountered an error when executing GET for URL %s; %w", workerName, nextUrl, err)
	}

	if resp.Body == nil {
		return job, fmt.Errorf("worker %s's response body is nil for URL %s", workerName, nextUrl)
	} else {
		defer resp.Body.Close()
	}
	w.log.Trace(fmt.Sprintf("Worker %s's GET request for URL %s returned a body of size %d bytes", workerName, nextUrl, resp.ContentLength))

	// parsing the resposne body and extracting any valid URLs
	w.log.Debug(fmt.Sprintf("Worker %s is extracting URLs from the response body of URL %s...", workerName, nextUrl))
	urls, err := w.extractWikiLinks(req.Body, workerName)
	if err != nil {
		return job, fmt.Errorf("worker %s encountered an error when extracting URLs from the response body for URL %s; %w", workerName, nextUrl, err)
	}

	w.log.Trace(fmt.Sprintf("Worker %s found %d preliminary unique URLs in resoponse body for URL %s. Removing URLs present in current path...", workerName, len(urls), nextUrl))
	isDuplicateStep := func(url string) bool {
		for _, p := range job.Path {
			if url == p {
				return true
			}
		}
		return false
	}
	for i, url := range urls {
		if isDuplicateStep(url) {
			w.log.Trace(fmt.Sprintf("Worker %s found a preliminary URL that already exists in path. Removing URL %s...", workerName, url))
			urls = slices.Delete(urls, i, i+1)
		}
	}
	w.log.Debug(fmt.Sprintf("Worker %s found %d unique URLs in response body", workerName))

	// updating and returning completed job
	job.LastPathUrls = urls
	return job, nil
}

func (w WikiSteps) extractWikiLinks(body io.ReadCloser, workerName string) ([]string, error) {
	w.log.Trace(fmt.Sprintf("Worker %s is parsing response body to html node...", workerName))
	root, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("error when parsing html response body; %w", err)
	}

	urlSet := make(map[string]struct{}) // set to keep only unique URLs in the path

	var traverse func(n *html.Node) // defining function to traverse nodes
	traverse = func(n *html.Node) {
		if n == nil {
			return
		}

		// checking if node is element <a> and has a valid Wikipedia URI
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && isValidWikistepUri(a.Val) {
					url := WikipediaDomain + a.Val
					if _, exists := urlSet[url]; exists {
						w.log.Trace(fmt.Sprintf("Worker %s's node %p has a valid duplicate URL: '%s'. Unique URL set length: %d", workerName, n, url, len(urlSet)))
					} else {
						urlSet[url] = struct{}{}
						w.log.Trace(fmt.Sprintf("Worker %s's node %p has a valid new URL: '%s'. Unique URL set length: %d", workerName, n, url, len(urlSet)))
					}
				}
			}
		}

		// recursive call to traverse children
		for c := range n.ChildNodes() {
			traverse(c)
		}
	}

	traverse(root) // call the traverse fucntion on the root node

	urls := make([]string, len(urlSet))
	i := 0
	for k, _ := range urlSet {
		urls[i] = k
		i += 1
	}
	return urls, nil
}

func (w WikiSteps) step(path []string, target string, step int, stopCh chan struct{}, resCh chan<- []string) {
	// slices of URLs that are potential paths, i want to get back a slice of a completed path, or close the channel if the step max was reached
	if len(path) == 0 {
		panic(fmt.Sprintf("Provided path slice is empty or nil. Path: %s", path))
	}

	nextUrl := path[len(path)-1]

	// if that url is our target publish that completed path
	if nextUrl == target {
		w.log.Debug(fmt.Sprintf("Found valid path: [%s]", strings.Join(path, ", ")))
		resCh <- path
		close(linkCh)
		return
	}

	if step <= 0 {
		close(linkCh)
		return
	}

	links, err := w.fetchWikiLinks(nextUrl, linkCh)
	if util.IsChannelClosed(linkCh) {
		return
	}

	if err != nil {
		// TODO return the error using a channel
		w.log.Error(fmt.Sprintf("error when fetching wiki links for url %s; %e", nextUrl, err))
		close(linkCh)
		return
	}

	w.log.Debug(fmt.Sprintf("Found %d links for URL: %s", len(links), nextUrl))

	// initialize a slice of channels
	linkChs := make([]chan struct{}, len(links))
	for i := range linkChs {
		linkChs[i] = make(chan struct{})
	}

	// start a go routine to check each subsequent link
	for i, l := range links {
		nextPath := make([]string, len(path))
		copy(nextPath, path)
		nextPath = append(path, l)
		go w.step(nextPath, target, step-1, linkChs[i], resCh)
	}

	// using reflect, select until all channels are closed (this means they ran out of steps)
	var cases []reflect.SelectCase
	for _, ch := range linkChs {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		})
	}
	// adding the linkCh channel parameter
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(linkCh),
	})

	// loop until every channel is closed
	for len(cases) > 1 {
		chosen, _, ok := reflect.Select(cases)
		if !ok {
			// if linkCh parameter channel is chosen, close all channels and return
			if chosen == len(cases)-1 {
				for _, ch := range linkChs {
					util.SafeClose(ch)
				}
				return
			} else {
				// channel is closed, remove it
				cases = slices.Delete(cases, chosen, chosen+1)
				w.log.Trace(fmt.Sprintf("Channel is closed and will be removed. Cases slice size is now: %d", len(cases)))
			}
		}
	}

	close(linkCh)
}

var wikiReqSemaphore = make(chan struct{}, 10)

func (w WikiSteps) fetchWikiLinks(url string, linkCh chan struct{}) ([]string, error) {
	waiting := true
	for waiting {
		select {
		case wikiReqSemaphore <- struct{}{}:
			waiting = false
		case _, ok := <-linkCh:
			if !ok {
				return nil, fmt.Errorf("exit was triggered by link channel")
			}
		}
	}
	defer func() { <-wikiReqSemaphore }()

	w.log.Trace(fmt.Sprintf("Checking validity of URL: %s", url))
	if !isValidWikiStepUrl(url) {
		w.log.Debug(fmt.Sprintf("URL is invalid, nothing to fetch. URL: %s", url))
		return []string{}, nil
	}

	w.log.Trace(fmt.Sprintf("Creating request for URL: %s", url))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error when building http GET request; %w", err)
	}

	w.log.Info(fmt.Sprintf("Doing http GET request on client for URL: %s", url))
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error when doing http request on client; %w", err)
	}

	if resp.Body == nil {
		return nil, fmt.Errorf("response body is nil for URL: %s", url)
	} else {
		defer resp.Body.Close()
	}

	w.log.Trace("Parsing response body...")
	node, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error when parsing html response body; %w", err)
	}

	w.log.Trace("Extracting wiki links from html...")
	return w.parseWikiLinks(node), nil
}

func (w WikiSteps) parseWikiLinks(root *html.Node) []string {
	urlSet := make(map[string]struct{})

	// creating function to traverse nodes
	var traverse func(n *html.Node)
	traverse = func(n *html.Node) {
		if n == nil {
			return
		}

		// checking if node is element <a> and has a Wiki URI
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && isValidWikistepUri(a.Val) {
					url := WikipediaDomain + a.Val
					if _, exists := urlSet[url]; exists {
						w.log.Trace(fmt.Sprintf("Node %p has valid duplicate URI: '%s'. URL: '%s'. Unique URL set length: %d", n, a.Val, url, len(urlSet)))
					} else {
						urlSet[url] = struct{}{}
						w.log.Trace(fmt.Sprintf("Node %p has valid new URI: '%s'. URL: '%s'. Unique URL set length: %d", n, a.Val, url, len(urlSet)))
					}
				}
			}
		}

		// recursive call to traverse children
		for c := range n.ChildNodes() {
			traverse(c)
		}
	}

	traverse(root)

	// gathering all URLs
	uniqueUrls := make([]string, len(urlSet))
	i := 0
	for k, _ := range urlSet {
		uniqueUrls[i] = k
		i += 1
	}
	return uniqueUrls
}

func isValidWikiStepUrl(urls ...string) bool {
	for _, s := range urls {
		if !strings.HasPrefix(s, WikipediaDomain+WikiPrefix) {
			return false
		}
		if reWikiBlockPrefix.MatchString(s) {
			return false
		}
		for _, b := range wikiBlockPrefixList {
			if strings.HasPrefix(s, b) {
				return false
			}
		}
	}
	return true
}

func isValidWikistepUri(uris ...string) bool {
	for _, s := range uris {
		if !strings.HasPrefix(s, WikiPrefix) {
			return false
		}
		if reWikiBlockPrefix.MatchString(s) {
			return false
		}
		for _, b := range wikiBlockPrefixList {
			if strings.HasPrefix(s, b) {
				return false
			}
		}
	}
	return true
}
