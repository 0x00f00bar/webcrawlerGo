package webcrawler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/queue"
)

var (
	defaultSleepDuration = 500 * time.Microsecond
	defaultIdleTimeout   = 10 * time.Second
)

// Crawler crawls the URL fetched from Queue and saves
// the contents to Models.
//
// Crawler will quit after IdleTimeout when no item received
// from queue
type Crawler struct {
	Name        string
	Queue       *queue.UniqueQueue
	Models      *models.Models
	BaseURL     *url.URL // base URI
	MarkedURLs  []string
	IdleTimeout time.Duration
	Log         *log.Logger
}

// NewCrawler return pointer to a new Crawler
func NewCrawler(
	name string,
	q *queue.UniqueQueue,
	m *models.Models,
	baseURI *url.URL,
	logger *log.Logger,
	idleTimeout time.Duration,
) (*Crawler, error) {
	if q == nil {
		return nil, errors.New("crawler: queue cannot be nil")
	}

	if m == nil {
		return nil, errors.New("crawler: models cannot be nil")
	}

	if !isValidScheme(baseURI.Scheme) {
		return nil, errors.New(
			fmt.Sprintf(
				"crawler: invalid scheme '%s'. Supported schemes: HTTP, HTTPS",
				baseURI.Scheme,
			),
		)
	}

	if !isAbsoluteURL(baseURI.String()) {
		return nil, errors.New("crawler: Base URI should be absolute")
	}

	if logger == nil {
		logger = log.New(os.Stdout, "crawler", log.LstdFlags)
	}

	c := &Crawler{
		Name:        name,
		Queue:       q,
		Models:      m,
		IdleTimeout: idleTimeout,
		BaseURL:     baseURI,
		Log:         logger,
	}
	return c, nil
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
		urlpath, err := c.Queue.Pop()

		// if queue is empty wait for defaultSleepDuration; retry upto idle timeout before quitting
		if errors.Is(err, queue.ErrEmptyQueue) {
			if time.Since(startTime) > c.IdleTimeout {
				c.Log.Printf("%s: Queue is empty, quitting.", c.Name)
				return
			}
			time.Sleep(defaultSleepDuration)
			continue
		}

		// add base URI if relative url

		// validate URL: not empty, of the base domain

		// if url -> fetch all urls embedded in the page
		resp, err := getURL(urlpath, client)
		if err != nil {
			// error occured in GET request, log error
			c.Log.Printf("%s: error in GET request: %v", c.Name, err)
			// and add the url back to queue
			c.Queue.PushForce(urlpath)
			continue
		}

		// if response status != 200, log error and continue
		if resp.StatusCode != http.StatusOK {
			c.Log.Printf(
				"%s: error in GET request: HTTP status code received %d",
				c.Name,
				resp.StatusCode,
			)
			continue
		}

		urls, err := c.fetchEmbeddedURLs(resp)

		// go through fetched urls, if url not in queue(map) save to db and queue
		for _, href := range urls {
			if c.Queue.FirstEncounter(href) {
				// temp time var as time.Time value cannot be set to nil
				// and we don't want to set URL.LastSaved and URL.LastChecked right now
				var t time.Time
				u := models.NewURL(href, t, t, c.isMarkedURL(href))
				c.Models.URLs.Insert(u)
				c.Queue.Push(href)
			}
		}

		// if current url is to be monitored/saved, save content to DB and update url

		// close response body
		resp.Body.Close()

		// reset startTime
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
			// if href is not absolute add BaseURL to href
			if !isAbsoluteURL(href) {
				c.Log.Printf("%s: converted relative url to absolute : %s", c.Name, href)
				href = c.BaseURL.String() + href
			}
			if c.isValidURL(href) {
				c.Log.Printf("%s: added url '%s' to queue", c.Name, href)
				hrefs = append(hrefs, href)
			} else {
				c.Log.Printf("%s: invalid url: %s", c.Name, href)
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
	if !isValidScheme(parsedURL.Scheme) {
		return false
	}

	return true
}

func (c *Crawler) isMarkedURL(href string) bool {
	for _, mUrl := range c.MarkedURLs {
		if strings.Contains(href, mUrl) {
			return true
		}
	}
	return false
}

// isAbsoluteURL checks if href is absolute URL
//
// e.g.
//
// <http/https>://google.com/query -> true
//
// /query -> false
func isAbsoluteURL(href string) bool {
	parsed, err := url.Parse(href)
	return err == nil && (parsed.Scheme != "" && parsed.Host != "")
}

// isValidScheme tells if the scheme is valid
func isValidScheme(scheme string) bool {
	return internal.ValuePresent(scheme, []string{"http", "https"})
}
