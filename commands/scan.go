package commands

import (
	"container/list"
	"fmt"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"strings"
)

func IsTelURL(targetURL string) bool {
	return strings.HasPrefix(targetURL, "tel:")
}

func IsMailtoURL(targetURL string) bool {
	return strings.HasPrefix(targetURL, "mailto:")
}

func ParseURL(baseURL url.URL, target string) (*url.URL, bool) {
	urlTarget, err := url.Parse(target)
	if err != nil {
		return &url.URL{}, false
	}
	if !urlTarget.IsAbs() {
		urlTarget = baseURL.ResolveReference(urlTarget)
	}

	if IsMailtoURL(urlTarget.String()) || IsTelURL(urlTarget.String()) {
		return urlTarget, false
	}

	if urlTarget.String() == "" {
		return urlTarget, false
	}
	return urlTarget, true
}

// LinkStatus TODO
type LinkStatus struct {
	StatusCode int
}

// Link TODO
type Link struct {
	Status LinkStatus
	Src    LinkURL
	URL    LinkURL
}

// LinkURL TODO
type LinkURL struct {
	Base     string  // how the url appears in the DOM
	Resolved url.URL // absolute URL
	Final    url.URL // After any redirects
}

// IsExternal TODO
func (t *Link) IsExternal() bool {
	if t.Src.Final.Hostname() == t.URL.Final.Hostname() {
		return false
	}
	return true
}

// IsOK checks if link is considered "valid"
func (t *Link) IsOK() bool {
	if t.Status.StatusCode == 200 {
		return true
	}

	if t.Status.StatusCode == -2 || t.Status.StatusCode == -3 {
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
		"a":      []string{"href"},
		"area":   []string{"href"},
		"audio":  []string{"src"},
		"embed":  []string{"src"},
		"iframe": []string{"src"},
		"img":    []string{"src"},
		"input":  []string{"src"},
		"source": []string{"src"},
		"track":  []string{"src"},
		"video":  []string{"src"},
	},
}

const level = 1

func IsValidElement(targetElement string) bool {
	tags := Tags[level]
	for element := range tags {
		if element == targetElement {
			return true
		}
	}
	return false
}

func FindLinks(token html.Token) (urls []string) {
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

func ScrapeHTML(currentPage url.URL) {
	resp, err := http.Get(currentPage.String())
	if err != nil {
		panic(err)
	}

	total := 0
	tokenizer := html.NewTokenizer(resp.Body)
	for {
		tokenType := tokenizer.Next()
		switch {
		case tokenType == html.ErrorToken:
			fmt.Printf("total: %d\n", total)
			resp.Body.Close()
			return
		case tokenType == html.SelfClosingTagToken:
			fallthrough
		case tokenType == html.StartTagToken:
			token := tokenizer.Token()

			if IsValidElement(token.Data) {
				foundUrls := FindLinks(token)
				for _, url := range foundUrls {
					resolvedURL, isValid := ParseURL(currentPage, url)
					jww.DEBUG.Printf("token: %v", token)
					fmt.Printf("+ %s -> %v [%v]\n", url, resolvedURL, isValid)
					total++
				}
			}
		}
	}
}

func newScan() *cobra.Command {
	scan := &cobra.Command{
		Use:   "scan",
		Short: "Scan a site for broken links",
		Long:  "Scan a site for broken links",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// crawledUrls := []string{}
			urlQueue := list.New()
			urlQueue.PushBack(args[0])

			for urlQueue.Len() > 0 {
				next := urlQueue.Front()

				nextURL := next.Value.(string)
				jww.DEBUG.Printf("crawling %v", nextURL)

				// Crawl page
				nextURLResolved, err := url.Parse(nextURL)
				if err != nil {
					panic(err)
				}
				ScrapeHTML(*nextURLResolved)

				urlQueue.Remove(next)
			}

			return
		},
	}
	return scan
}
