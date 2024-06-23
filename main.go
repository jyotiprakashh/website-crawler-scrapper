package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type SeoData struct {
	URL             string
	Title           string
	H1              string
	MetaDescription string
	StatusCode      int
}

type Parser interface {
	getSEOData(res *http.Response) (SeoData, error)
}
type DefaultParser struct {
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Safari/604.1.38",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:56.0) Gecko/20100101 Firefox/56.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Safari/604.1.38",
}

func randomUserAgent() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	// rand.Seed(time.Now().Unix())
	randNum := rand.Int() % len(userAgents)
	return userAgents[randNum]
}

func extractSiteMapURLs(startURL string) []string {
	worklist := make(chan []string)
	toCrawl := []string{}
	var n int
	n++
	go func() { worklist <- []string{startURL} }()

	for ; n > 0; n-- {

		list := <-worklist
		for _, link := range list {
			n++
			go func(link string) {
				response, err := makeRequest(link)
				if err != nil {
					log.Printf("failed to retrieve url: %s", link)
				}
				urls, _ := extractURLs(response)
				if err != nil {
					log.Printf("failed to extract documents from response, url %s", link)
				}
				sitemapFiles, pages := isSitemap(urls)
				if sitemapFiles != nil {
					worklist <- sitemapFiles
				}
				for _, page := range pages {
					toCrawl = append(toCrawl, page)
				}
			}(link)
		}
	}
	return toCrawl
}

func isSitemap(urls []string) ([]string, []string) {
	sitemapFiles := []string{}
	pages := []string{}
	for _, page := range urls {
		foundSitemap := strings.Contains(page, "xml")
		if foundSitemap {
			fmt.Println("found sitemap", page)
			sitemapFiles = append(sitemapFiles, page)
		} else {
			pages = append(pages, page)
		}
	}
	return sitemapFiles, pages
}

func makeRequest(url string) (*http.Response, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", randomUserAgent())
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func scrapeURLs(urls []string, parser Parser, concurr int) []SeoData {
	tokens := make(chan struct{}, concurr)
	var n int
	n++
	worklist := make(chan []string)
	results := []SeoData{}

	go func() { worklist <- urls }()

	for ; n > 0; n-- {
		list := <-worklist
		for _, url := range list {
			if url != "" {
				n++
				go func(url string, token chan struct{}) {
					log.Printf("scraping %s", url)
					res, err := scrapePage(url, tokens, parser)
					if err != nil {
						log.Printf("failed to scrape url: %s", url)
					} else {
						results = append(results, res)
					}
					worklist <- []string{}
				}(url, tokens)
			}
		}
	}
	return results
}

func extractURLs(res *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	urls := []string{}
	sel := doc.Find("loc")
	for i := range sel.Nodes {
		loca := sel.Eq(i)
		urls = append(urls, loca.Text())
	}
	return urls, nil
}

func scrapePage(url string, token chan struct{}, parser Parser) (SeoData, error) {
	res, err := crawlPage(url, token)
	if err != nil {
		return SeoData{}, err
	}
	data, err := parser.getSEOData(res)
	if err != nil {
		return SeoData{}, err
	}
	return data, nil
}

func crawlPage(url string, tokens chan struct{}) (*http.Response, error) {
	tokens <- struct{}{}
	resp, err := makeRequest(url)
	<-tokens
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d DefaultParser) getSEOData(resp *http.Response) (SeoData, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return SeoData{}, err
	}
	data := SeoData{}
	data.URL = resp.Request.URL.String()
	data.StatusCode = resp.StatusCode
	data.Title = doc.Find("title").First().Text()
	data.H1 = doc.Find("h1").First().Text()
	data.MetaDescription = doc.Find("meta[name=description]").AttrOr("content", "")
	return data, nil
}

func scrapeSiteMap(url string, parser Parser, concurr int) []SeoData {
	results := extractSiteMapURLs(url)
	res := scrapeURLs(results, parser, concurr)
	return res
}

func main() {
	p := DefaultParser{}
	results := scrapeSiteMap("https://www.quicksprout.com/sitemap.xml", p, 10)
	for _, res := range results {
		fmt.Println(res)
	}
}
