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

var resultImgs []string
var emptyLinks []Link
var mutex sync.Mutex

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/crawl", search)
	http.ListenAndServe(":8080", nil)
}

func home(writer http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("home.html")
	t.Execute(writer, nil)
}

func search(writer http.ResponseWriter, r *http.Request) {
	var resultVars ResultPageVars
	var homeVars HomePageVars

	// Validating Input
	if r.Method == "GET" {
		homeVars.ErrorMessage = "Search via the search bar"
		t, _ := template.ParseFiles("home.html")
		t.Execute(writer, homeVars)
		return
	}

	if err := r.ParseForm(); err != nil {
		fmt.Println("ERROR READING FROM FORM")
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
	images := make(chan []string)
	// Listens for new imgs found and adds to resultImgs
	// Ends once images channel is closed
	go func() {
		for imgList := range images {
			for _, img := range imgList {
				resultImgs = append(resultImgs, img)
			}
		}
	}()

	seenLinks := make(map[Link]bool)
	start := time.Now()

	// Set up pipelines
	baseChan := gen(baseLinks)
	firstDiscovered := crawler(baseChan, images, writer)
	nextSet := filter(firstDiscovered, seenLinks)
	// Fan out/in of nextSet
	secondDiscovered := merge(crawler(nextSet, images, writer), crawler(nextSet, images, writer), crawler(nextSet, images, writer), crawler(nextSet, images, writer))
	finalOutput := filter(secondDiscovered, seenLinks)

	// Consume output
	for outputLink := range finalOutput {
		resultVars.Links = append(resultVars.Links, outputLink.Url)
	}

	elapsedTime := time.Since(start).Seconds()
	resultVars = aggregateResults(resultVars, elapsedTime)

	t, _ := template.ParseFiles("results.html")
	t.Execute(writer, resultVars)

	cleanResults(resultVars)
}

func merge(linkChans ...<-chan []Link) <-chan []Link {
	var wg sync.WaitGroup
	mergedChan := make(chan []Link)

	deplete := func(linkChan <-chan []Link) {
		for discoveredLinks := range linkChan {
			mergedChan <- discoveredLinks
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

func filter(discoveredLinks <-chan []Link, seenLinks map[Link]bool) <-chan Link {
	filteredLinks := make(chan Link)
	go func() {
		for links := range discoveredLinks {
			for _, link := range links {
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
		}
		close(filteredLinks)
	}()
	return filteredLinks
}

func crawler(linkChan <-chan Link, images chan<- []string, writer http.ResponseWriter) <-chan []Link {
	discoveredLinks := make(chan []Link)
	go func() {
		for link := range linkChan {
			discoveredLinks <- crawl(link, images, writer)
		}
		close(discoveredLinks)
	}()
	return discoveredLinks
}

func crawl(link Link, images chan<- []string, writer http.ResponseWriter) []Link {
	var resultVars ResultPageVars

	fmt.Println(link)

	newLinks, newImgs, err := extract(link)

	if err != nil {
		fmt.Print(err)
		resultVars.ErrorMessage = err.Error()
		t, _ := template.ParseFiles("results.html")
		t.Execute(writer, resultVars)
		return emptyLinks
	}
	images <- newImgs
	return newLinks
}

// Extracts all urls from a web page
func extract(link Link) (links []Link, images []string, err error) {
	resp, err := http.Get(link.Url)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("Error getting: %s | %s", link.Url, resp.Status)
	}
	page, err := html.Parse(resp.Body) // returns root *htmlNode
	resp.Body.Close()
	if err != nil {
		fmt.Errorf("Error parsing html:%s,  %v", link.Url, err)
		return nil, nil, err
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
					links = append(links, newLink)
				}
			}
		} else if node.Type == html.ElementNode && node.Data == "img" {
			for _, a := range node.Attr {
				if strings.HasPrefix(a.Val, "https://") {
					fmt.Println(a.Val)
					images = append(images, a.Val)
				}
			}
		}
	}
	forEveryNode(page, visitNode, nil)
	return links, images, err
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

func aggregateResults(resultVars ResultPageVars, elapsed float64) ResultPageVars {
	resultVars.LinkCountTotal = len(resultVars.Links)
	resultVars.ImageCountTotal = len(resultImgs)
	resultVars.Images = resultImgs
	resultVars.Time = elapsed
	return resultVars
}

func cleanResults(resultVars ResultPageVars) {
	resultImgs = nil
	resultVars.LinkCountTotal = 0
	resultVars.Links = nil
	resultVars.ImageCountTotal = 0
	resultVars.Images = nil
	resultVars.Time = 0
	resultVars.ErrorMessage = ""
}
