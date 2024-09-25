package webcrawler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/queue"
	"github.com/PuerkitoBio/goquery"
)

var defaultSleepDuration = 500 * time.Microsecond

// Crawler crawls the URL fetched from Queue and saves
// the contents to Models.
//
// Crawler will quit after IdleTimeout when no item received
// from queue
type Crawler struct {
	Name        string
	Queue       *queue.UniqueQueue
	Models      *models.Models
	IdleTimeout time.Duration
	BaseURL     *url.URL // base URI
}

// NewCrawler return pointer to a new Crawler
func NewCrawler(name string, m *models.Models, baseURI *url.URL, idleTimeout time.Duration) *Crawler {
	return &Crawler{
		Name:        name,
		Queue:       queue.NewQueue(),
		Models:      m,
		IdleTimeout: idleTimeout,
		BaseURL:     baseURI,
	}
}

// GetURL fetches first item from queue, if queue
// is empty returns queue.ErrEmptyQueue
// func (c *Crawler) GetURL() (string, error) {
// 	return c.Queue.Pop()
// }

// Crawl to begin crawling
func (c *Crawler) Crawl(clientTimeout time.Duration) {
	startTime := time.Now()

	client := &http.Client{
		Timeout: clientTimeout,
	}

	for {
		// get item from queue
		url, err := c.Queue.Pop()

		// if queue is empty wait for defaultSleepDuration; retry upto idle timeout before quitting
		if errors.Is(err, queue.ErrEmptyQueue) {
			if time.Since(startTime) > c.IdleTimeout {
				fmt.Printf("%s: Queue is empty, quitting.", c.Name)
				return
			}
			time.Sleep(defaultSleepDuration)
			continue
		}

		// add base URI if relative url

		// validate URL: not empty, of the base domain

		// if url -> fetch all urls embedded in the page
		resp, err := getURL(url, client)
		if err != nil {
			// error occured in GET request, log error
			log.Printf("%s: error in GET request: %v", c.Name, err)
			// and add the url back to queue
			c.Queue.PushForce(url)
			continue
		}

		// if response status != 200, log error and continue
		if resp.StatusCode != http.StatusOK {
			log.Printf("%s: error in GET request: HTTP status code received %d", c.Name, resp.StatusCode)
			continue
		}

		urls, err := c.fetchEmbeddedURLs(resp)

		// go through fetched urls, if url not in queue(map) save to db and queue

		// if current url is to be monitored/saved, save content to DB and update url

		// close response body
		resp.Body.Close()

		startTime = time.Now()
	}
}

// fetchEmbeddedURLs will fetch all values in href attribute of <a> tag
func (c *Crawler) fetchEmbeddedURLs(resp *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	hrefs := []string{}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if href, found := s.Attr("href"); found {
			if c.isValidURL(href) {
				hrefs = append(hrefs, href)
			} else {
				log.Printf("%s: invalid url: %s", c.Name, href)
			}
		}
	})
	return hrefs, nil
}

/*
	validateURL checks if the URL is valid

Rules:
  - Have base URI if absolute URL
  - Is not empty
  - Scheme is either HTTP/HTTPS
*/
func (c *Crawler) isValidURL(href string) bool {

	// URL is not empty
	if href == "" {
		return false
	}

	parsedURL, err := url.Parse(href)
	if err != nil {
		return false
	}

	// check if URL is absolute and have same hostname as crawler BaseURL
	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		if parsedURL.Hostname() != c.BaseURL.Hostname() {
			return false
		}
	}

	// valid scheme: HTTP/HTTPS
	if !c.isValidScheme(parsedURL.Scheme) {
		return false
	}

	return true
}

// isAbsoluteURL checks if href is absolute URL
//
// e.g.
//
// <http/https>://google.com/query -> true
//
// /query -> false
func (c *Crawler) isAbsoluteURL(href string) bool {
	parsed, err := url.Parse(href)
	return err == nil && (parsed.Scheme != "" && parsed.Host != "")
}

// isValidScheme tells if the scheme is valid
func (c *Crawler) isValidScheme(scheme string) bool {
	return internal.ValuePresent(scheme, []string{"http", "https"})
}
