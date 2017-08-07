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
  "strings"
)

type PageVars struct {
  TotalCount int
  Images []string
}

var homePage PageVars
var imageCount int
var done chan struct{}

func main() {
  done = make(chan struct{})
  http.HandleFunc("/", search)
  http.HandleFunc("/crawl", crawlForImages)
  // http.HandleFunc("/", layoutTest)
  http.ListenAndServe(":8080", nil)
}


func search(writer http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFiles("search.html")
    t.Execute(writer, nil)
}

func crawlForImages(writer http.ResponseWriter, r *http.Request) {

  err := r.ParseForm()
  if err != nil {
    fmt.Println("ERROR READING FROM FORM")
  }

  formValue := r.PostFormValue("baseURL")
  fmt.Println(formValue)

  var baseUrl []string
  baseUrl = append(baseUrl, formValue)
  if len(baseUrl) == 0 {
    fmt.Println("DIDN'T GET INPUT")
  }

  worklist := make(chan []string)
  unseenLinks := make(chan string)
  seen := make(map[string]bool)
  images := make(chan string)

  go func() { worklist <- baseUrl }()

  var results PageVars

  go func() {
    for image := range images {
      fmt.Println("Image received")
      results.Images = append(results.Images, image)
      results.TotalCount = results.TotalCount + 1
    }
    fmt.Println("reached end of images")
  }()

  // Limits crawling to 20 go routines
  for i := 0; i < 20; i++ {
    go func() {
      select {
      case <- done:
        fmt.Print("crawling cancelled")
        for range unseenLinks {
          // drain unseenLinks
        }
      default:
        for link := range unseenLinks {
          foundLinks := crawl(writer, images, unseenLinks, link)
          go func() {
            if !cancelled() {
              fmt.Print("not cancelled")
              worklist <- foundLinks
            }
          }()
          if cancelled() {
            break
          }
        }
      }
    }()
  }

  // Runs in main go routine
  // "manages" unseenLinks for crawling routines
  // Due to possible links being search by multiple routines
  for list := range worklist {
    for _, link := range list {
      if !seen[link] {
        seen[link] = true;
        unseenLinks <- link
      }
    }
  }

  t, _ := template.ParseFiles("results.html")
  t.Execute(writer, results)
}


func crawl(writer http.ResponseWriter, imagelist chan<- string, unseenLinks chan string, url string) []string {
  if imageCount > 60 {
    if !cancelled() {
      fmt.Println("REACHED LIMIT")
      close(done)
    }
    var emptyList []string
    return emptyList
  }
  links, err := extract(writer, imagelist, url)
  if err != nil {
    fmt.Errorf("Error extracting urls from: %s: %v", url, err)
  }
  return links
}


// Extracts all urls from a web page
func extract(writer http.ResponseWriter, imagelist chan<- string, url string) (contents []string, err error) {

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
    } else if node.Type == html.ElementNode && node.Data == "img" {
      fmt.Fprintf(writer, "IMAGE FOUND: ")
      fmt.Printf("image found %d\n", imageCount)
      imageCount++
      for _, a := range node.Attr {
        if strings.HasPrefix(a.Val, "https://") { // Take only the src attr
          // fmt.Fprintf(writer, "%s\n", a.Val)
          imagelist <- a.Val // send img src through images channel
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

func cancelled() bool {
  select {
  case <- done:
    return true
  default:
    return false
  }
}
