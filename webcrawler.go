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
// TODOs
// * Create server to take Url(s)
// * Create HTML template for results

type PageVars struct {
  TotalCount int
  Images []string
}

n := 0
var homePage PageVars

// Make Template html file (with javascript & css)
// Parse template
// Execute Struct Variables on the template
//t, err := template.ParseFiles("select.html")
//err = t.Execute(w, MyPageVariables)

func main() {
  http.HandleFunc("/", search)
  http.HandleFunc("/crawl", crawlForImages)
  // http.HandleFunc("/", layoutTest)
  http.ListenAndServe(":8080", nil)
}

// var foundImages Images

// func layoutTest(writer http.ResponseWriter, r *http.Request) {
//
//   i1 := Image{url: "https://upload.wikimedia.org/wikipedia/commons/thumb/2/2f/Space_Needle002.jpg/1200px-Space_Needle002.jpg"}
//   i2 := Image{url: "http://doubletree3.hilton.com/resources/media/dt/CTAC-DT/en_US/img/shared/full_page_image_gallery/main/DT_spaceneedle_20_677x380_FitToBoxSmallDimension_Center.jpg"}
//   i3 := Image{url: "https://upload.wikimedia.org/wikipedia/commons/thumb/f/fb/Seattle_Columbia_Pano2.jpg/640px-Seattle_Columbia_Pano2.jpg"}
//   i4 := Image{url: "https://image.dynamixse.com/s/crop/1600x1000/https://cdn.dynamixse.com/seattlegreatwheelcom/seattlegreatwheelcom_408653813.jpg"}
//   i5 := Image{url: "https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRoMqsnbBF6SVjnYIU_ZytViG4cTEZskVkXpmzL_zRDYtsO5r4QJg"}
//
//   imagesV := make([]string, 5)
//   imagesV[0] = i1.url
//   imagesV[1] = i2.url
//   imagesV[2] = i3.url
//   imagesV[3] = i4.url
//   imagesV[4] = i5.url
//
//   var tester PageVars
//   tester.TotalCount = 4
//   tester.Images = imagesV
//
//   t, _ := template.ParseFiles("results.html")
//   t.Execute(writer, tester)
// }

func search(writer http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFiles("search.html")
    t.Execute(writer, nil)
}

func crawlForImages(writer http.ResponseWriter, r *http.Request) {
  t, _ := template.ParseFiles("results.html")

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

  go func() { worklist <- baseUrl }()

  // Limits crawling to 20 go routines
  for i := 0; i < 20; i++ {
    go func() {
      for link := range unseenLinks {
        foundLinks := crawl(writer, link)
        go func() { worklist <- foundLinks }()
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
  // TODO make struct of page variables and execute them here
  // clean up crapper code
}


func crawl(writer http.ResponseWriter, url string) []string {
  //fmt.Print(url)
  links, err := extract(writer, url)
  if err != nil {
    fmt.Errorf("Error extracting urls from: %s: %v", url, err)
  }
  return links
}


// Extracts all urls from a web page
func extract(writer http.ResponseWriter, url string) (contents []string, err error) {

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
      fmt.Println("image found")
      n++
      if n > 15 {
        return
      }
      for _, a := range node.Attr {
        if strings.HasPrefix(a.Val, "https://") { // Take only the src attr
          fmt.Fprintf(writer, "%s\n", a.Val)
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
