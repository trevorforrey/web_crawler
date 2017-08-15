// webcrawler crawls the web given a starting URL

// This project was based on examples in the following Golang book:
// The Go Programming Language, by Alan A. A. Donovan & Brian W. Kernighan
// The web crawler has been modified to take input from a running Go server
// and display images found from crawling

// TODO Don't use channels just as queues - Research and draw out pattern in book
// Create Workers pattern using go routines, (start out with 1 go routine)

package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
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

var resultUrls []string
var resultImgs []string
var emptyLinks []Link
var worklist chan []string

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

	worklist := make(chan []Link)
	unseenLinks := make(chan Link)
	images := make(chan []string)

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

	//////////////////////////////////////////////////////////////

	// send starting links to worklist
	go func() {
		worklist <- baseLinks
	}()

	// Listens for new imgs found and adds to resultImgs
	// Ends once images channel is closed
	go func() {
		for imgList := range images {
			for _, img := range imgList {
				resultImgs = append(resultImgs, img)
			}
		}
	}()

	start := time.Now()
	maxDepth := 3

	go func() {
		for link := range unseenLinks {
			foundLinks := crawl(link, images, writer, maxDepth)
			go func() {
				worklist <- foundLinks
			}()
		}
	}()

	seen := make(map[Link]bool)
	for list := range worklist {
		for _, link := range list {
			if !seen[link] {
				seen[link] = true
				resultUrls = append(resultUrls, link.Url)
				unseenLinks <- link
			}
		}
	}

	close(images)

	elapsed := time.Since(start)

	resultVars = aggregateResults(resultVars, resultUrls, elapsed.Seconds())

	t, _ := template.ParseFiles("results.html")
	t.Execute(writer, resultVars)

	cleanResults(resultVars)
}

func crawl(link Link, images chan<- []string, writer http.ResponseWriter, maxDepth int) []Link {

	var resultVars ResultPageVars

	if link.Depth == maxDepth {
		fmt.Println("Reached max depth")
		return emptyLinks // see if possible to return nil
	}

	fmt.Println(link)
	pageImgs, err := extractImgs(link)

	if err != nil {
		fmt.Print(err)
		resultVars.ErrorMessage = err.Error()
		t, _ := template.ParseFiles("results.html")
		t.Execute(writer, resultVars)
		return emptyLinks
	}
	fmt.Println("Before sending to images channel")
	images <- pageImgs
	fmt.Println("After sending to the images channel")

	newLinks, err := extractLinks(link)

	if err != nil {
		fmt.Print(err)
		resultVars.ErrorMessage = err.Error()
		t, _ := template.ParseFiles("results.html")
		t.Execute(writer, resultVars)
		return emptyLinks
	}
	return newLinks
}

// Extracts all urls from a web page
func extractLinks(link Link) (links []Link, err error) {

	resp, err := http.Get(link.Url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Error getting: %s | %s", link.Url, resp.Status)
	}
	page, err := html.Parse(resp.Body) // returns root *htmlNode
	resp.Body.Close()
	if err != nil {
		fmt.Errorf("Error parsing html:%s,  %v", link.Url, err)
		return nil, err
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
		}
	}
	forEveryNode(page, visitNode, nil)
	return links, err
}

// Extracts all imgs from a web page
func extractImgs(url Link) (images []string, err error) {

	resp, err := http.Get(url.Url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Error getting: %s | %s", url, resp.Status)
	}
	page, err := html.Parse(resp.Body) // returns root *htmlNode
	resp.Body.Close()
	if err != nil {
		fmt.Errorf("Error parsing html:%s,  %v", url, err)
		return nil, err
	}

	var visitNode func(node *html.Node)
	visitNode = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "img" {
			for _, a := range node.Attr {
				if strings.HasPrefix(a.Val, "https://") {
					fmt.Println(a.Val)
					images = append(images, a.Val)
				}
			}
		}
	}
	forEveryNode(page, visitNode, nil)
	return images, err
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

func aggregateResults(resultVars ResultPageVars, links []string, elapsed float64) ResultPageVars {
	resultVars.LinkCountTotal = len(links)
	resultVars.Links = links
	resultVars.ImageCountTotal = len(resultImgs)
	resultVars.Images = resultImgs
	resultVars.Time = elapsed
	return resultVars
}

func cleanResults(resultVars ResultPageVars) {
	resultUrls = nil
	resultImgs = nil
	resultVars.LinkCountTotal = 0
	resultVars.Links = nil
	resultVars.ImageCountTotal = 0
	resultVars.Images = nil
	resultVars.Time = 0
	resultVars.ErrorMessage = ""
}
