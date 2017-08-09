// webcrawler crawls the web given a starting URL

// This project was based on examples in the following Golang book:
// The Go Programming Language, by Alan A. A. Donovan & Brian W. Kernighan
// The web crawler has been modified to take input from a running Go server
// and display images found from crawling

package main

import (
  "fmt"
  "net/http"
  "golang.org/x/net/html"
  "html/template"
)

type PageVars struct {
  TotalCount int
  Links []string
  //TotalTime float64
	//URLsVisisted int
}

var resultUrls []string


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
  //Create Page Vars variable
  var resultVars PageVars

	//Make sure http request is NOT a GET
	//If it is, return an error to the user

  if err := r.ParseForm(); err != nil {
    fmt.Println("ERROR READING FROM FORM")
  }

  // seen := make(map[string]bool)
  var baseUrls []string
  fmt.Print("starting urls")
	baseUrls = append(baseUrls, r.PostFormValue("baseURLs"))
  for _, url := range baseUrls {
    fmt.Println(url)
  }
  fmt.Print("ending urls")
	links := crawl(baseUrls, 2)

  resultVars.TotalCount = len(links)
  resultVars.Links = links

  t, _ := template.ParseFiles("results.html")
  t.Execute(writer, resultVars)
}


func crawl(urls []string, depth int) []string {
  var emptyList []string
  depth--
	if depth == 0 {
    fmt.Println("Reached depth of zero")
		return emptyList
	}
	for _, link := range urls {
		// if seen(link) {
		// 	continue
		// }
    // seen[link] = true
    fmt.Println(link)
    newUrls, err := extract(link)
    if err != nil {
      fmt.Print(err)
    }

		resultUrls = append(newUrls, crawl(newUrls, depth)...) // returns string[] of links
	}
	return resultUrls
}


// Extracts all urls from a web page
func extract(url string) (contents []string, err error) {

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
  }

  var links []string
  var visitNode func(node *html.Node)
  visitNode = func(node *html.Node) {
    if node.Type == html.ElementNode && node.Data == "a" {
      for _, a := range node.Attr {
        if a.Key == "href" {
          link, err := resp.Request.URL.Parse(a.Val)
          if err != nil { // only accept valid urls
            continue;
          }
          links = append(links, link.String())
        }
      }
    }
  }
  forEveryNode(page, visitNode, nil)
  return links, err
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
