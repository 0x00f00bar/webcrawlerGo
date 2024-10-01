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
	invalidHrefPrefixs   = []string{"file:", "mailto:", "tel:", "javascript:", "#", "data:"}
)

// Crawler crawls the URL fetched from Queue and saves
// the contents to Models.
//
// Crawler will quit after IdleTimeout when queue is empty
type Crawler struct {
	Name string // Name of crawler for easy identification
	*CrawlerConfig
}

// CrawlerConfig to configure a crawler
type CrawlerConfig struct {
	Queue          *queue.UniqueQueue // global queue
	Models         *models.Models     // models to use
	BaseURL        *url.URL           // base URL to crawl
	MarkedURLs     []string           // marked URL to save to model
	RequestDelay   time.Duration      // delay between subsequent requests
	IdleTimeout    time.Duration      // timeout after which crawler quits when queue is empty
	Log            *log.Logger        // logger to use
	RetryTimes     int                // no. of times to retry failed request
	FailedRequests map[string]int     // map to store failed requests stats
}

// NewCrawler return pointer to a new Crawler
func NewCrawler(name string, cfg *CrawlerConfig) (*Crawler, error) {
	err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Crawler{name, cfg}, nil
}

// NNewCrawlers returns N new Crawlers configured with cfg.
// Crawlers will be named with namePrefix.
func NNewCrawlers(n int, namePrefix string, cfg *CrawlerConfig) ([]*Crawler, error) {
	if n < 1 {
		return nil, fmt.Errorf("too few crawlers")
	}

	err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}

	var crawlers []*Crawler

	for i := range n {
		name := fmt.Sprintf("%s#%03d", namePrefix, i+1)
		crawlers = append(crawlers, &Crawler{name, cfg})
	}

	return crawlers, nil
}

// validateConfig verifies crawler config
// if Log is nil, creates new os.Stdout default logger
func validateConfig(cfg *CrawlerConfig) error {
	if cfg.Queue == nil {
		return errors.New("crawler: queue cannot be nil")
	}

	if cfg.Models == nil {
		return errors.New("crawler: models cannot be nil")
	}

	if !internal.IsValidScheme(cfg.BaseURL.Scheme) {
		return fmt.Errorf(
			"crawler: invalid scheme '%s'. Supported schemes: HTTP, HTTPS",
			cfg.BaseURL.Scheme,
		)
	}

	if !internal.IsAbsoluteURL(cfg.BaseURL.String()) {
		return errors.New("crawler: Base URL should be absolute")
	}

	if cfg.Log == nil {
		cfg.Log = log.New(os.Stdout, "crawler", log.LstdFlags)
	}

	return nil
}

// GetURL fetches first item from queue, if queue
// is empty returns queue.ErrEmptyQueue
// func (c *Crawler) GetURL() (string, error) {
// 	return c.Queue.Pop()
// }

// Crawl to begin crawling
func (c *Crawler) Crawl(client *http.Client) {
	startTime := time.Now()

	for {
		// get item from queue
		urlpath, err := c.Queue.Pop()

		// if queue is empty wait for defaultSleepDuration; retry upto idle timeout before quitting
		if errors.Is(err, queue.ErrEmptyQueue) {
			if time.Since(startTime) > c.IdleTimeout {
				c.Log.Printf("%s: Queue is empty, quitting.\n", c.Name)
				return
			}
			time.Sleep(defaultSleepDuration)
			continue
		}

		resp, err := getURL(urlpath, client)
		if err != nil {
			c.Log.Printf("%s: error in GET request: %v for url: '%s'\n", c.Name, err, urlpath)
			// check that FailedRequests is not nil (when map not init; RetryTimes==0)
			if c.FailedRequests != nil && c.FailedRequests[urlpath] < c.RetryTimes {
				// and add the url back to queue
				c.Queue.PushForce(urlpath)
				c.FailedRequests[urlpath] += 1
			}
			continue
		}

		// if response not 200 OK
		if resp.StatusCode != http.StatusOK {
			c.Log.Printf(
				"%s: invalid HTTP status code received %d for url: '%s'\n",
				c.Name,
				resp.StatusCode,
				urlpath,
			)
			continue
		}

		// if OK fetch all urls embedded in the page
		urls, err := c.fetchEmbeddedURLs(resp)
		if err != nil {
			c.Log.Printf(
				"%s: failed to fetch embedded URLs for URL '%s' : %v\n",
				c.Name,
				urlpath,
				err,
			)
			continue
		}

		// go through fetched urls, if url not in queue(map) save to db and queue
		for _, href := range urls {
			if c.Queue.FirstEncounter(href) {
				// temp time var as time.Time value cannot be set to nil
				// and we don't want to set URL.LastSaved and URL.LastChecked right now
				var t time.Time
				u := models.NewURL(href, t, t, c.isMarkedURL(href))
				err = c.Models.URLs.Insert(u)
				if err != nil {
					c.Log.Fatalf("%s: failed to insert url '%s' to model: %v\n", c.Name, href, err)
				}
				if ok := c.Queue.Push(href); ok {
					c.Log.Printf("%s: added url '%s' to queue\n", c.Name, href)
					// if url is marked set value to true to fetch its content
					if u.IsMonitored {
						c.Queue.SetMapValue(href, true)
					}
				}
			}
		}

		saveURLContent, err := c.Queue.GetMapValue(urlpath)
		if errors.Is(err, queue.ErrItemNotFound) {
			c.Log.Fatalf(
				"%s: FATAL : URL not found in queue map '%s'. Quitting.\n",
				c.Name,
				urlpath,
			)
		}

		// if current url is to be monitored OR marked, save content to DB and update url
		if c.isMarkedURL(urlpath) || saveURLContent {
			err = c.savePageContent(urlpath, resp)
			if err != nil {
				c.Log.Fatalln(err)
			}
			c.Log.Printf("%s: saved content of url '%s'\n", c.Name, urlpath)

			// set key value to false as url is now processed
			c.Queue.SetMapValue(urlpath, false)
		}

		// close response body
		resp.Body.Close()

		// take rest for RequestDelay
		time.Sleep(c.RequestDelay)

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
			if !internal.IsAbsoluteURL(href) && !internal.BeginsWith(href, invalidHrefPrefixs) {
				if !strings.HasPrefix(href, "/") {
					href = "/" + href
				}
				c.Log.Printf("%s: converted relative url to absolute : %s\n", c.Name, href)
				href = c.BaseURL.String() + href
			}
			if c.isValidURL(href) {
				hrefs = append(hrefs, href)
			} else {
				c.Log.Printf("%s: invalid url: %s\n", c.Name, href)
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
