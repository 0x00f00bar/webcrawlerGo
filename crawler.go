package webcrawler

import (
	"errors"
	"fmt"
	"io"
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
	// defaultIdleTimeout   = 10 * time.Second
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
	BaseURL     *url.URL
	MarkedURLs  []string
	IdleTimeout time.Duration
	Log         *log.Logger
}

// NewCrawler return pointer to a new Crawler
func NewCrawler(
	name string,
	q *queue.UniqueQueue,
	m *models.Models,
	baseURL *url.URL,
	markedURLs []string,
	logger *log.Logger,
	idleTimeout time.Duration,
) (*Crawler, error) {
	if q == nil {
		return nil, errors.New("crawler: queue cannot be nil")
	}

	if m == nil {
		return nil, errors.New("crawler: models cannot be nil")
	}

	if !internal.IsValidScheme(baseURL.Scheme) {
		return nil, fmt.Errorf(
			"crawler: invalid scheme '%s'. Supported schemes: HTTP, HTTPS",
			baseURL.Scheme,
		)
	}

	if !internal.IsAbsoluteURL(baseURL.String()) {
		return nil, errors.New("crawler: Base URL should be absolute")
	}

	if logger == nil {
		logger = log.New(os.Stdout, "crawler", log.LstdFlags)
	}

	c := &Crawler{
		Name:        name,
		Queue:       q,
		Models:      m,
		BaseURL:     baseURL,
		MarkedURLs:  markedURLs,
		IdleTimeout: idleTimeout,
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

		resp, err := getURL(urlpath, client)
		if err != nil {
			c.Log.Printf("%s: error in GET request: %v", c.Name, err)
			// and add the url back to queue
			c.Queue.PushForce(urlpath)
			continue
		}

		// if response not 200 OK
		if resp.StatusCode != http.StatusOK {
			c.Log.Printf(
				"%s: error in GET request: HTTP status code received %d",
				c.Name,
				resp.StatusCode,
			)
			continue
		}

		// if OK fetch all urls embedded in the page
		urls, err := c.fetchEmbeddedURLs(resp)
		if err != nil {
			c.Log.Printf("%s: failed to fetch embedded URLs for URL '%s' : %v", c.Name, urlpath, err)
			continue
		}

		// go through fetched urls, if url not in queue(map) save to db and queue
		for _, href := range urls {
			if c.Queue.FirstEncounter(href) {
				// temp time var as time.Time value cannot be set to nil
				// and we don't want to set URL.LastSaved and URL.LastChecked right now
				var t time.Time
				u := models.NewURL(href, t, t, c.isMarkedURL(href))
				c.Models.URLs.Insert(u)
				c.Queue.Push(href)
				// if url is marked set value to true to fetch its content
				if u.IsMonitored {
					c.Queue.SetMapValue(href, true)
				}
			}
		}

		saveURLContent, err := c.Queue.GetMapValue(urlpath)
		if errors.Is(err, queue.ErrItemNotFound) {
			c.Log.Fatalf("%s: URL not found in queue map '%s'. Quitting.", c.Name, urlpath)
		}

		// if current url is to be monitored OR marked, save content to DB and update url
		if c.isMarkedURL(urlpath) || saveURLContent {
			err = c.savePageContent(urlpath, resp)
			if err != nil {
				c.Log.Fatal(err)
			}

			// set key value to false as url is now processed
			c.Queue.SetMapValue(urlpath, false)
		}

		// close response body
		resp.Body.Close()

		// reset startTime
		startTime = time.Now()
	}
}

// savePageContent saves URL response body to models
func (c *Crawler) savePageContent(urlpath string, resp *http.Response) error {
	uModel, err := c.Models.URLs.GetByURL(urlpath)
	if err != nil {
		return fmt.Errorf("%s: could not get URL from model: %v", c.Name, err)
	}
	urlContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: could not read response body: %v", c.Name, err)
	}
	newPage := models.NewPage(uModel.ID, string(urlContent))
	if err = c.Models.Pages.Insert(newPage); err != nil {
		return fmt.Errorf("%s: could not insert page into model: %v", c.Name, err)
	}
	uModel.LastChecked = time.Now()
	uModel.LastSaved = time.Now()
	if err = c.Models.URLs.Update(uModel); err != nil {
		return fmt.Errorf("%s: could not update URL model: %v", c.Name, err)
	}
	return nil
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
			if !internal.IsAbsoluteURL(href) {
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
  - Have base URL if absolute URL
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
	if !internal.IsValidScheme(parsedURL.Scheme) {
		return false
	}

	return true
}

// isMarkedURL checks whether the href should be processed
func (c *Crawler) isMarkedURL(href string) bool {
	for _, mUrl := range c.MarkedURLs {
		if mUrl != "" && strings.Contains(href, mUrl) {
			return true
		}
	}
	return false
}
