// webcrawler crawls the web given a starting URL

// This project was based on examples in the following Golang book:
// The Go Programming Language, by Alan A. A. Donovan & Brian W. Kernighan
// The web crawler has been modified to take input from a running Go server
// and display images found from crawling

// TODO Clean and perfect this version. Research error handling, formatting, and Benchmarking

package main

import (
	"fmt"
	"golang.org/x/net/html"
	"html/template"
	"net/http"
	"strings"
)

type PageVars struct {
	LinkCountTotal int
	Links          []string
	Images         []string
	//TotalTime float64
	ImageCountTotal int
}

var resultUrls []string
var resultImgs []string
var emptyList []string

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

	var resultVars PageVars

	//Make sure http request is NOT a GET
	//If it is, return an error to the user

	if err := r.ParseForm(); err != nil {
		fmt.Println("ERROR READING FROM FORM")
	}

	var baseUrls []string
	baseUrls = append(baseUrls, r.PostFormValue("baseURLs"))

	links := crawl(baseUrls, 2)

	resultVars.LinkCountTotal = len(links)
	resultVars.Links = links
	resultVars.ImageCountTotal = len(resultImgs)
	resultVars.Images = resultImgs

	t, _ := template.ParseFiles("results.html")
	t.Execute(writer, resultVars)
}

func crawl(urls []string, depth int) []string {

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
			continue
		}

		resultImgs = append(resultImgs, pageImgs...)
		newUrls, err := extractLinks(link)

		if err != nil {
			fmt.Print(err)
			continue
		}
		resultUrls = append(newUrls, crawl(newUrls, depth)...)
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
