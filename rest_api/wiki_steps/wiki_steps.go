package wikiSteps

import (
	"app/rest_api/logging"
	"app/rest_api/util"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"slices"

	"golang.org/x/net/html"
)

const (
	WikipediaDomain string        = "https://en.wikipedia.org"
	WikiPrefix      string        = "/wiki/"
	StepsMaximum    int           = 7
	SteppingTimeout time.Duration = 30 * time.Second
)

var (
	Service             *WikiSteps
	initialized         bool
	wikiBlockPrefixList []string       = []string{"/wiki/Main_Page"}
	reWikiBlockPrefix   *regexp.Regexp = regexp.MustCompile(`^/wiki/\w+:.*`)
)

type WikiSteps struct {
	log        logging.Logger
	httpClient *http.Client
}

func InitWikistepsService(log logging.Logger) *WikiSteps {
	if !initialized {
		Service = &WikiSteps{
			log,
			&http.Client{},
		}
		initialized = true
	}
	return Service
}

func (w WikiSteps) FindSteps(start string, target string, steps int) ([][]string, error) {
	if !isValidWikiStepUrl(start, target) {
		return nil, fmt.Errorf("start or target is invalid")
	}

	if steps > StepsMaximum || steps < 0 {
		return nil, fmt.Errorf("steps cannot be negative")
	}

	// prepparing to start stepping
	resCh := make(chan []string, 1000)
	linkCh := make(chan struct{})
	path := make([]string, 1)
	path[0] = start
	timeout := time.After(SteppingTimeout)
	results := make([][]string, 0)

	w.log.Info(fmt.Sprintf("Started stepping! start: %s, target: %s, steps: %d, timeout: %d", start, target, steps, SteppingTimeout))
	go w.step(path, target, steps, linkCh, resCh)

	// monitoring for results
	for {
		select {
		case r, ok := <-resCh:
			results = append(results, r)
			if !ok {
				return results, nil
			}
		case <-timeout:
			util.SafeClose(linkCh)
			return results, fmt.Errorf("execution timed out")
		}
	}
}

func (w WikiSteps) step(path []string, target string, step int, linkCh chan struct{}, resCh chan<- []string) {
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

	links, err := w.fetchWikiLinks(nextUrl)
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

func (w WikiSteps) fetchWikiLinks(url string) ([]string, error) {
	wikiReqSemaphore <- struct{}{}
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

	time.Sleep(1 * time.Second)
	w.log.Debug(fmt.Sprintf("Doing http GET request on client for URL: %s", url))
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
