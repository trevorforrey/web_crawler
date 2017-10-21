// webcrawler crawls the web given a starting URL

// This project was based on examples in the following Golang book:
// The Go Programming Language, by Alan A. A. Donovan & Brian W. Kernighan
// The base channel and go routine structure has been changed to a pipeline structure
// The web crawler has also been modified to take input from a running Go server
// and display images found from crawling

package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type ResultPageVars struct {
	LinkCountTotal  int
	Links           []string
	Images          []string
	ImageCountTotal int
	ErrorMessage    string
	Time            float64
}

type HomePageVars struct {
	ErrorMessage string
}

type Link struct {
	Url   string
	Depth int
}

var mutex sync.Mutex

func main() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", home)
	http.HandleFunc("/crawl", search)
	http.ListenAndServe(":8090", nil)
}

func home(writer http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("home.html")
	t.Execute(writer, nil)
}

func search(writer http.ResponseWriter, r *http.Request) {
	var resultVars ResultPageVars
	var homeVars HomePageVars
	var resultImgs []string

	// Validating Input
	if r.Method == "GET" {
		homeVars.ErrorMessage = "Search via the search bar"
		t, _ := template.ParseFiles("home.html")
		t.Execute(writer, homeVars)
		return
	}

	if err := r.ParseForm(); err != nil {
		fmt.Println("ERROR READING FROM FORM")
		homeVars.ErrorMessage = "Error Reading from Form"
		t, _ := template.ParseFiles("home.html")
		t.Execute(writer, homeVars)
		return
	}

	var baseUrls []string
	baseUrls = append(baseUrls, r.PostFormValue("baseURLs"))
	var baseLinks []Link

	for _, baseUrl := range baseUrls {
		_, err := url.ParseRequestURI(baseUrl)
		if err != nil {
			homeVars.ErrorMessage = "An invalid url was provided"
			t, _ := template.ParseFiles("home.html")
			t.Execute(writer, homeVars)
			return
		}
		var newLink Link
		newLink.Url = baseUrl
		newLink.Depth = 1
		baseLinks = append(baseLinks, newLink)
	}

	// DONE VALIDATING INPUT

	// CRAWLING LOGIC
	images := make(chan string)
	// Listens for new imgs found and adds to resultImgs
	// Ends once images channel is closed
	go func() {
		for img := range images {
			resultImgs = append(resultImgs, img)
		}
	}()

	seenLinks := make(map[Link]bool)
	start := time.Now()

	// Set up pipelines
	baseChan := gen(baseLinks)
	crawl1Chan := speedyCrawl(baseChan, images, writer)
	filter1Chan := speedyFilter(crawl1Chan, seenLinks)
	crawl2Chan := speedyMerge(speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer), speedyCrawl(filter1Chan, images, writer))
	finalOutput := speedyFilter(crawl2Chan, seenLinks)

	// Consume output
	for outputLink := range finalOutput {
		resultVars.Links = append(resultVars.Links, outputLink.Url)
	}

	elapsedTime := time.Since(start).Seconds()
	resultVars = aggregateResults(resultVars, elapsedTime, resultImgs)

	t, _ := template.ParseFiles("results.html")
	t.Execute(writer, resultVars)

	cleanResults(resultVars)
}

func speedyMerge(linkChans ...<-chan Link) <-chan Link {
	var wg sync.WaitGroup
	mergedChan := make(chan Link)

	deplete := func(linkChan <-chan Link) {
		for discoveredLink := range linkChan {
			mergedChan <- discoveredLink
		}
		wg.Done()
	}

	wg.Add(len(linkChans))
	for _, linkChannel := range linkChans {
		go deplete(linkChannel)
	}

	go func() {
		wg.Wait()
		close(mergedChan)
	}()
	return mergedChan
}

func gen(entryLinks []Link) <-chan Link {
	out := make(chan Link)
	go func() {
		for _, link := range entryLinks {
			out <- link
		}
		close(out)
	}()
	return out
}

func speedyFilter(discoveredLinks <-chan Link, seenLinks map[Link]bool) <-chan Link {
	filteredLinks := make(chan Link)
	go func() {
		for link := range discoveredLinks {
			mutex.Lock()
			if !seenLinks[link] {
				seenLinks[link] = true
				mutex.Unlock()
				filteredLinks <- link
			} else {
				mutex.Unlock()
				continue
			}
		}
		close(filteredLinks)
		fmt.Println("Close of speed filter")
	}()
	return filteredLinks
}

func speedyCrawl(linkChan <-chan Link, images chan<- string, writer http.ResponseWriter) <-chan Link {
	// var resultVars ResultPageVars
	foundLink := make(chan Link)

	go func() {
		for link := range linkChan {
			resp, err := http.Get(link.Url)
			if err != nil {
				fmt.Errorf("Error getting html:%s,  %v", link.Url, err)
				continue
				// TODO error getting url
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				fmt.Errorf("Error on status code:%s,  %v", link.Url, err)
				if resp.StatusCode == http.StatusTooManyRequests {
					// Initiate stopping on crawling
				}
				continue
				// TODO Not good HTTP status code
				// Start termination if code is 429 Too Many Requests
			}
			page, err := html.Parse(resp.Body) // returns root *htmlNode
			resp.Body.Close()
			if err != nil {
				fmt.Errorf("Error parsing html:%s,  %v", link.Url, err)
				continue
				// TODO error parsing the page
			}

			var visitNode func(node *html.Node)
			visitNode = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "a" {
					for _, a := range node.Attr {
						if a.Key == "href" {
							url, err := resp.Request.URL.Parse(a.Val)
							if err != nil { // only accept valid urls
								continue
							}
							var newLink Link
							newLink.Url = url.String()
							newLink.Depth = link.Depth + 1
							foundLink <- newLink
						}
					}
				} else if node.Type == html.ElementNode && node.Data == "img" {
					for _, a := range node.Attr {
						if strings.HasPrefix(a.Val, "https://") {
							fmt.Println(a.Val)
							images <- a.Val
						}
					}
				}
			}
			forEveryNode(page, visitNode, nil)
		}
		close(foundLink)
	}()
	return foundLink
}

func forEveryNode(node *html.Node, pre, post func(n *html.Node)) {
	if pre != nil {
		pre(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		forEveryNode(child, pre, post)
	}
	if post != nil {
		post(node)
	}
}

func aggregateResults(resultVars ResultPageVars, elapsed float64, resultImgs []string) ResultPageVars {
	resultVars.LinkCountTotal = len(resultVars.Links)
	resultVars.ImageCountTotal = len(resultImgs)
	resultVars.Images = resultImgs
	resultVars.Time = elapsed
	return resultVars
}

func cleanResults(resultVars ResultPageVars) {
	resultVars.Images = nil
	resultVars.LinkCountTotal = 0
	resultVars.Links = nil
	resultVars.ImageCountTotal = 0
	resultVars.Images = nil
	resultVars.Time = 0
	resultVars.ErrorMessage = ""
}
