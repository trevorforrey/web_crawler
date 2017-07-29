// webcrawler crawls the web given a starting URL

// This project was based on examples in the following Golang book:
// The Go Programming Language, by Alan A. A. Donovan & Brian W. Kernighan

package main

import (
  "fmt"
  "net/http"
  "os"
  "golang.org/x/net/html"
)
// TODOs
// * Create server to take Url(s)
// * Create HTML template for results

func main() {
  // contents, _ := extract(os.Args[1])
  // for _, link := range contents {
  //   fmt.Printf("%s\n", link);
  // }
  worklist := make(chan []string)
  unseenLinks := make(chan string)
  seen := make(map[string]bool)

  go func() { worklist <- os.Args[1:] }()

  // Limits crawling to 20 go routines
  for i := 0; i < 20; i++  {
    go func() {
      for link := range unseenLinks {
        foundLinks := crawl(link)
        go func() { worklist <- foundLinks }()
      }
    }()
  }

  // Runs in main go routine to "manage" unseenLinks for crawling routines
  for list := range worklist {
    for _, link := range list {
      if !seen[link] {
        seen[link] = true;
        unseenLinks <- link
      }
    }
  }
}


func crawl(url string) []string {
  fmt.Print(url)
  links, err := extract(url)
  if err != nil {
    fmt.Errorf("Error extracting urls from: %s: %v", url, err)
  }
  return links
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
          if err != nil {
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
