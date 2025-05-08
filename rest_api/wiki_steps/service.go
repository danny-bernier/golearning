package wikiSteps

import (
	"app/rest_api/logging"
	"app/rest_api/util"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"math"

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

type wikiStepToolbelt struct {
	Target      string
	Timeout     <-chan time.Time
	JobCh       chan wikiStepJob
	CompletedCh chan wikiStepJob
	ExitCh      chan struct{}
	ErrCh       chan error
	IdleCh      chan struct{}
	Wg          *sync.WaitGroup
	WgCh        chan struct{}
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
	exitCh := make(chan struct{})
	errCh := make(chan error)
	jobCh := make(chan wikiStepJob, int(math.Pow(10.0, float64(steps))))
	completedCh := make(chan wikiStepJob, 1000*w.numWorkers)
	idleCh := make(chan struct{}, w.numWorkers)
	wgCh := make(chan struct{})

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
	go func() {
		wg.Wait()
		close(wgCh)
	}()

	timeout := time.After(w.stepsTimeout)

	w.log.Trace("Initializing toolbelt...")
	toolbelt := wikiStepToolbelt{
		Target:      target,
		Timeout:     timeout,
		JobCh:       jobCh,
		CompletedCh: completedCh,
		ExitCh:      exitCh,
		ErrCh:       errCh,
		IdleCh:      idleCh,
		Wg:          &wg,
		WgCh:        wgCh,
	}

	w.log.Trace("Starting workers...")
	workerNames := util.RandomNames(w.numWorkers/100, w.numWorkers)
	for i := 0; i < w.numWorkers; i += 1 {
		go w.wikiStepWorker(workerNames[i], toolbelt)
		w.log.Debug(fmt.Sprintf("Started worker %s", workerNames[i]))
	}

	results, err := w.wikiStepSupervisor(toolbelt)
	if err != nil {
		return results, fmt.Errorf("error in wikiStepSupervisor; %w", err)
	}

	return append(results, w.wikiStepCleanup(toolbelt)...), nil
}

func (w WikiSteps) wikiStepCleanup(toolbelt wikiStepToolbelt) [][]string {
	results := make([][]string, 0)
	for {
		select {
		case completedJob := <-toolbelt.CompletedCh:
			for _, url := range completedJob.LastPathUrls {
				if url == toolbelt.Target {
					w.log.Debug("WikiSteps found a valid path to the target in cleanup")
					results = append(results, append(completedJob.Path, url))
				}
			}
		case <-toolbelt.WgCh:
			return results
		}
	}
}

func (w WikiSteps) wikiStepSupervisor(toolbelt wikiStepToolbelt) ([][]string, error) {
	results := make([][]string, 0)
	for {
		select {
		case <-toolbelt.Timeout:
			w.log.Info(fmt.Sprintf("WikiSteps timed out after %.0f seconds, signaling exit...", w.stepsTimeout.Seconds()))
			close(toolbelt.ExitCh)
			return results, nil

		case err := <-toolbelt.ErrCh:
			w.log.Info("WikiSteps is signaling exit after encountering an error...")
			close(toolbelt.ExitCh)
			return results, err

		case <-time.After(3 * time.Second):
			if len(toolbelt.IdleCh) >= w.numWorkers && len(toolbelt.JobCh) == 0 {
				w.log.Debug("WikiSteps workers are all idle and job queue is empty, signaling exit...")
				close(toolbelt.ExitCh)
				return results, nil
			}

		case completedJob := <-toolbelt.CompletedCh:
			for _, url := range completedJob.LastPathUrls {
				if url == toolbelt.Target {
					w.log.Debug("WikiSteps found a valid path to the target")
					results = append(results, append(completedJob.Path, url))

				} else if completedJob.NumStepsRemaining-1 > 0 {
					w.log.Debug("WikiSteps did not find a valid path to target yet, resubmitting job")
					var j wikiStepJob
					j.Path = append(completedJob.Path, url)
					j.NumStepsRemaining = completedJob.NumStepsRemaining - 1
					j.CompletedCh = toolbelt.CompletedCh
					toolbelt.JobCh <- j

				} else {
					w.log.Debug("WikiSteps dead end! A path ran out of steps")
				}
			}
		}
	}
}

func (w WikiSteps) wikiStepWorker(name string, toolbelt wikiStepToolbelt) {
	defer toolbelt.Wg.Done()
	for {
		select {
		case <-toolbelt.ExitCh:
			w.log.Debug(fmt.Sprintf("Worker %s received exit signal, closing...", name))
			return
		case job := <-toolbelt.JobCh:
			select {
			case <-toolbelt.IdleCh:
				w.log.Trace(fmt.Sprintf("Worker %s is now active, removed from IdleCh", name))
			default: // If IdleCh is already empty, no action needed
			}
			w.log.Debug(fmt.Sprintf("Worker %s started a new job", name))
			job, err := w.doWikiStepJob(name, job, toolbelt.ExitCh)
			if err != nil {
				toolbelt.ErrCh <- err
				continue
			}
			toolbelt.CompletedCh <- job
			w.log.Debug(fmt.Sprintf("Worker %s completed a job", name))
			select {
			case toolbelt.IdleCh <- struct{}{}:
				w.log.Trace(fmt.Sprintf("Worker %s is now idle, added to IdleCh", name))
			default: // If IdleCh is full, no action needed
			}
		}
	}
}

func (w WikiSteps) doWikiStepJob(workerName string, job wikiStepJob, exitCh <-chan struct{}) (wikiStepJob, error) {
	if len(job.Path) == 0 {
		return job, fmt.Errorf("worker %s's path slice is empty", workerName) // should never happen
	}

	nextUrl := job.Path[len(job.Path)-1] // isolate the next URL to fetch data for

	resp, err := w.callWikipedia(workerName, nextUrl, exitCh)
	if err != nil {
		return job, fmt.Errorf("worker %s encountered an error when calling Wikipedia; %w", workerName, err)
	}

	if resp == nil {
		return job, nil
	} else {
		defer resp.Close()
	}

	// parsing the resposne body and extracting any valid URLs
	w.log.Debug(fmt.Sprintf("Worker %s is extracting URLs from the response body of URL %s...", workerName, nextUrl))
	urls, err := w.extractWikiLinks(resp, workerName)
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
	w.log.Debug(fmt.Sprintf("Worker %s found %d unique URLs in response body", workerName, len(urls)))

	// updating and returning completed job
	job.LastPathUrls = urls
	return job, nil
}

func (w WikiSteps) callWikipedia(workerName string, url string, exitCh <-chan struct{}) (io.ReadCloser, error) {
	// requesting data from Wikipedia
	w.log.Debug(fmt.Sprintf("Worker %s is waiting for semaphore aquisition...", workerName))
	select {
	case httpSem <- struct{}{}:
		w.log.Debug(fmt.Sprintf("Worker %s aquired semaphore.", workerName))
		defer func() { <-httpSem }()
	case <-exitCh:
		w.log.Debug(fmt.Sprintf("Worker %s recieved exit signal while waiting for semaphore aquisition.", workerName))
		return nil, nil
	}

	w.log.Trace(fmt.Sprintf("Worker %s is building GET request for URL %s", workerName, url))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("worker %s encountered an error when building GET request for URL: %s; %w", workerName, url, err)
	}

	w.log.Debug(fmt.Sprintf("Worker %s is executing a GET request for URL %s", workerName, url))
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker %s encountered an error when executing GET for URL %s; %w", workerName, url, err)
	}

	if resp.Body == nil {
		return nil, fmt.Errorf("worker %s's response body is nil for URL %s", workerName, url)
	}
	w.log.Trace(fmt.Sprintf("Worker %s's GET request for URL %s returned a body of size %d bytes", workerName, url, resp.ContentLength))
	return resp.Body, nil
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
