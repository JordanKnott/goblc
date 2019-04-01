package commands

import (
	"container/list"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PrimaryURL TODO
var PrimaryURL *url.URL

func IsTelURL(targetURL string) bool {
	return strings.HasPrefix(targetURL, "tel:")
}

func IsMailtoURL(targetURL string) bool {
	return strings.HasPrefix(targetURL, "mailto:")
}

// LinkStatus TODO
type LinkStatus struct {
	StatusCode int
}

// Link TODO
type Link struct {
	Status LinkStatus
	Src    url.URL
	URL    url.URL
}

func (t *Link) IsExternal() bool {
	if PrimaryURL.Hostname() == t.URL.Hostname() {
		return false
	}
	return true
}

// IsOK checks if link is considered "valid"
func (t *LinkStatus) IsOK() bool {
	if t.StatusCode == 200 {
		return true
	}

	if t.StatusCode == -2 || t.StatusCode == -3 {
		return true
	}
	return false
}

func (t *LinkStatus) String() string {
	messages := map[int]string{
		-3:  "Phone",
		-2:  "Mailto",
		-1:  "Invalid",
		200: "HTTP 200",
		403: "HTTP 403",
		404: "HTTP 404",
	}

	if val, ok := messages[t.StatusCode]; ok {
		return val
	}
	return "Unknown"
}

// Tags TODO
var Tags = map[int]map[string][]string{
	0: {
		"a":    []string{"href"},
		"area": []string{"href"},
	},
	1: {
		"a":        []string{"href"},
		"area":     []string{"href"},
		"audio":    []string{"src"},
		"embed":    []string{"src"},
		"iframe":   []string{"src"},
		"img":      []string{"src"},
		"input":    []string{"src"},
		"menuitem": []string{"icon"},
		"meta":     []string{"content"},
		"object":   []string{"data"},
		"source":   []string{"src"},
		"track":    []string{"src"},
		"video":    []string{"src"},
	},
}

const level = 1

// GetLinks TODO
func GetLinks(token html.Token) (urls []string) {
	targetAttrs := Tags[level][token.Data]
	for _, targetAttr := range targetAttrs {
		for _, attr := range token.Attr {
			if attr.Key == targetAttr {
				urls = append(urls, attr.Val)
			}
		}
	}
	return
}

var ResponseCache = cache.New(10*time.Minute, 10*time.Minute)

// IsValidElement TODO
func IsValidElement(targetElement string) bool {
	tags := Tags[level]
	for element := range tags {
		if element == targetElement {
			return true
		}
	}
	return false
}

// GetLink TODO
func GetLink(url string) (*http.Response, LinkStatus) {
	resp, err := http.Get(url)
	if err != nil {
		return resp, LinkStatus{-1}
	}
	return resp, LinkStatus{resp.StatusCode}
}

func CheckLink(src *url.URL, target *url.URL) Link {
	if link, ok := ResponseCache.Get(target.String()); ok {
		jww.DEBUG.Printf("retrieved %s from cache", target.String())
		return link.(Link)
	}
	resp, err := http.Head(target.String())
	if err != nil {
		return Link{LinkStatus{-1}, *src, *target}
	}
	link := Link{LinkStatus{resp.StatusCode}, *src, *target}
	ResponseCache.Set(target.String(), link, cache.NoExpiration)
	return link
}

// CrawlPage TODO
func CrawlPage(currentPage url.URL) (list.List, *[]Link, error) {
	jww.DEBUG.Printf("crawing page %s", currentPage.String())

	resp, err := http.Get(currentPage.String())
	if err != nil {
		return list.List{}, &[]Link{}, err
	}
	tokenizer := html.NewTokenizer(resp.Body)
	addToQueue := list.New()
	checkedLinks := []Link{}
	for {
		tokenType := tokenizer.Next()
		switch {
		case tokenType == html.ErrorToken: // End of the DOM
			resp.Body.Close()
			return *addToQueue, &checkedLinks, nil
		case tokenType == html.SelfClosingTagToken:
			fallthrough
		case tokenType == html.StartTagToken:
			token := tokenizer.Token()
			isValid := IsValidElement(token.Data)
			jww.DEBUG.Printf("token %v", token)

			if isValid {
				urls := GetLinks(token)
				for _, url := range urls {
					if parsedURL, ok := ParseURL(url); ok {
						link := CheckLink(&currentPage, parsedURL)
						fmt.Printf("\t%s\n", link.URL.String())
						jww.DEBUG.Printf("checked %v", link.URL.String())
						if !link.IsExternal() {
							jww.DEBUG.Printf("adding to queue %v", link.URL.String())
							addToQueue.PushBack(link.URL.String())
						}
						checkedLinks = append(checkedLinks, link)
					} else {
						jww.DEBUG.Printf("skipping %s", parsedURL.String())
					}

				}
			}
		}
	}
}

func ParseURL(target string) (*url.URL, bool) {
	urlTarget, err := url.Parse(target)
	if err != nil {
		return &url.URL{}, false
	}
	if !urlTarget.IsAbs() {
		urlTarget = urlTarget.ResolveReference(PrimaryURL)
	}

	if IsMailtoURL(urlTarget.String()) || IsTelURL(urlTarget.String()) {
		return urlTarget, false
	}

	if urlTarget.String() == "" {
		return urlTarget, false
	}
	return urlTarget, true
}

// IsCrawled TODO
func IsCrawled(urlCache *[]string, target string) bool {
	for _, url := range *urlCache {
		if url == target {
			return true
		}
	}
	return false
}

func newScan() *cobra.Command {
	scan := &cobra.Command{
		Use:   "scan",
		Short: "Scan a site for broken links",
		Long:  "Scan a site for broken links",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			PrimaryURL, _ = url.Parse(args[0])
			crawledUrls := []string{}
			urlQueue := list.New()
			urlQueue.PushBack(args[0])
			links := []Link{}

			for urlQueue.Len() > 0 {
				nextURL := urlQueue.Front()

				if parsedURL, ok := ParseURL(nextURL.Value.(string)); ok {
					if !IsCrawled(&crawledUrls, parsedURL.String()) {
						fmt.Printf("[%d]: Crawling %s\n", urlQueue.Len(), parsedURL.String())
						newUrls, checkedLinks, err := CrawlPage(*parsedURL)
						if err != nil {
							jww.WARN.Printf("error when attemptingt to crawl %s", parsedURL.String())
							panic(err)
						}
						urlQueue.PushBackList(&newUrls)
						crawledUrls = append(crawledUrls, parsedURL.String())
						links = append(links, *checkedLinks...)
					} else {
						jww.DEBUG.Printf("already crawled %s", parsedURL.String())
					}

				}
				urlQueue.Remove(nextURL)
			}

			for _, link := range links {
				if !link.Status.IsOK() {
					fmt.Printf("[%s] %s - %s\n", link.Src.String(), link.URL.String(), link.Status.String())
				}
			}
			fmt.Printf("total links: %d\n", len(links))
			return
		},
	}
	return scan
}
