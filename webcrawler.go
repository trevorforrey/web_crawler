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

var resultUrls []string
var resultImgs []string
var emptyList []string
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

	for _, baseUrl := range baseUrls {
		_, err := url.ParseRequestURI(baseUrl)
		if err != nil {
			homeVars.ErrorMessage = "An invalid url was provided"
			t, _ := template.ParseFiles("home.html")
			t.Execute(writer, homeVars)
			return
		}
	}

	// Listens for new imgs found and adds to resultImgs
	// Will need to Mutex lock this op. when mult workers
	go func() {
		for imgList := range images {
			for _, img := range imgList {
				resultImgs = append(resultImgs, img)
			}
		}
	}()

	start := time.Now()
	links := crawl(images, writer, baseUrls, 3)
	close(images)
	elapsed := time.Since(start)

	resultVars = aggregateResults(resultVars, links, elapsed.Seconds())

	t, _ := template.ParseFiles("results.html")
	t.Execute(writer, resultVars)

	cleanResults(resultVars)
}

func crawl(images chan<- []string, writer http.ResponseWriter, urls []string, depth int) []string {

	var resultVars ResultPageVars

	depth--
	if depth == 0 {
		fmt.Println("Reached depth of zero")
		return emptyList
	}

	for _, link := range urls {
		fmt.Println(link)
		pageImgs, err := extractImgs(link)

		if err != nil {
			fmt.Print(err)
			resultVars.ErrorMessage = err.Error()
			t, _ := template.ParseFiles("results.html")
			t.Execute(writer, resultVars)
			return emptyList
		}
		fmt.Println("Before sending to images channel")
		images <- pageImgs
		fmt.Println("After sending to the images channel")

		newUrls, err := extractLinks(link)

		if err != nil {
			fmt.Print(err)
			resultVars.ErrorMessage = err.Error()
			t, _ := template.ParseFiles("results.html")
			t.Execute(writer, resultVars)
			return emptyList
		}
		resultUrls = append(newUrls, crawl(images, writer, newUrls, depth)...)
	}
	return resultUrls
}

// Extracts all urls from a web page
func extractLinks(url string) (links []string, err error) {

	resp, err := http.Get(url)
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
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, a := range node.Attr {
				if a.Key == "href" {
					link, err := resp.Request.URL.Parse(a.Val)
					if err != nil { // only accept valid urls
						continue
					}
					links = append(links, link.String())
				}
			}
		}
	}
	forEveryNode(page, visitNode, nil)
	return links, err
}

// Extracts all imgs from a web page
func extractImgs(url string) (images []string, err error) {

	resp, err := http.Get(url)
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
	resultVars.Time = elapsed.Seconds()
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
