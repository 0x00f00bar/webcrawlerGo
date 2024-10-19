package webcrawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jimsmart/grobotstxt"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/0x00f00bar/webcrawlerGo/queue"
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

// InvalidURLCache is the cache for invalid URLs
type InvalidURLCache struct {
	cache sync.Map
}

// CrawlerConfig to configure a crawler
type CrawlerConfig struct {
	Queue            *queue.UniqueQueue // global queue
	Models           *models.Models     // models to use
	BaseURL          *url.URL           // base URL to crawl
	UserAgent        string             // user-agent to use while crawling
	MarkedURLs       []string           // marked URL to save to model
	IgnorePatterns   []string           // URL pattern to ignore
	RequestDelay     time.Duration      // delay between subsequent requests
	IdleTimeout      time.Duration      // timeout after which crawler quits when queue is empty
	Log              *log.Logger        // logger to use
	RetryTimes       int                // no. of times to retry failed request
	FailedRequests   map[string]int     // map to store failed requests stats
	KnownInvalidURLs *InvalidURLCache   // known map of invalid URLs
	Ctx              context.Context    // context to quit on SIGINT/SIGTERM
	robotsTxt        *string            // robots.txt as string (internal)
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
		cfg.Log = log.New(os.Stdout, "crawler", log.LstdFlags|log.Lshortfile)
	}

	// get robots.txt file
	if cfg.robotsTxt == nil {
		robotTxt, err := getRobotsTxt(cfg.BaseURL, cfg.UserAgent)
		if err != nil {
			return err
		}
		cfg.robotsTxt = robotTxt
	}

	// make a map of known invalid paths for efficient filtering
	if cfg.KnownInvalidURLs == nil {
		cfg.KnownInvalidURLs = &InvalidURLCache{}
	}

	return nil
}

// Crawl to begin crawling
func (c *Crawler) Crawl(client *http.Client) {
	startTime := time.Now()

	for {
		select {
		case <-c.Ctx.Done():
			c.Log.Printf("%s: Termination signal received. Shutting down\n", c.Name)
			return
		default:
			// get item from queue
			urlpath, err := c.Queue.Remove()

			// if queue is empty wait for defaultSleepDuration; retry upto idle timeout before quitting
			if errors.Is(err, queue.ErrEmptyQueue) {
				if time.Since(startTime) > c.IdleTimeout {
					c.Log.Printf("%s: Queue is empty, quitting.\n", c.Name)
					return
				}
				time.Sleep(defaultSleepDuration)
				continue
			}

			resp, err := c.getURL(urlpath, client)
			if err != nil {
				c.Log.Printf("%s: Error in GET request: %v for url: '%s'\n", c.Name, err, urlpath)
				// check that FailedRequests is not nil (when map is not initialised i.e. RetryTimes==0)
				if c.FailedRequests != nil && c.FailedRequests[urlpath] < c.RetryTimes {
					// and add the url back to queue
					c.Queue.InsertForce(urlpath)
					c.FailedRequests[urlpath] += 1
				}
				continue
			}

			// if response not 200 OK
			if resp.StatusCode != http.StatusOK {
				c.Log.Printf(
					"%s: Invalid HTTP status code received %d for url: '%s'\n",
					c.Name,
					resp.StatusCode,
					urlpath,
				)
				continue
			}

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				c.Log.Printf("%s: Could not read response body: %v", c.Name, err)
				continue
			}

			// if status OK fetch all hrefs embedded in the page
			hrefs, err := c.fetchEmbeddedURLs(doc)
			if err != nil {
				c.Log.Printf(
					"%s: Failed to fetch embedded URLs for URL '%s' : %v\n",
					c.Name,
					urlpath,
					err,
				)
				continue
			}

			// go through fetched urls, if url not in queue(map) save to db and queue
			for _, href := range hrefs {
				if c.isValidURL(href) {
					if ok := c.Queue.Insert(href); ok {
						c.Log.Printf("%s: Added url '%s' to queue\n", c.Name, href)
						// temp time var as time.Time value cannot be set to nil
						// and we don't want to set URL.LastSaved and URL.LastChecked right now
						var t time.Time
						u := models.NewURL(href, t, t, c.isMarkedURL(href))
						err = c.Models.URLs.Insert(u)
						if err != nil {
							c.Log.Printf(
								"%s: FATAL : Failed to insert url '%s' to model: %v\n",
								c.Name,
								href,
								err,
							)
							runtime.Goexit()
						}
						// if url is marked set value to true to fetch its content
						if u.IsMonitored {
							c.Queue.SetMapValue(href, true)
						}
					}
				} else {
					c.Log.Printf("%s: Invalid url: %s\n", c.Name, href)
					c.KnownInvalidURLs.cache.Store(href, true)
				}
			}

			// map value of current URL
			saveURLContent, err := c.Queue.GetMapValue(urlpath)
			if errors.Is(err, queue.ErrItemNotFound) {
				c.Log.Printf(
					"%s: FATAL : URL not found in queue map '%s'. Quitting.\n",
					c.Name,
					urlpath,
				)
				runtime.Goexit()
			}

			// if current url is to be monitored OR marked, save content to DB and update url
			if c.isMarkedURL(urlpath) || saveURLContent {
				err = c.savePageContent(urlpath, doc)
				if err != nil {
					c.Log.Printf("%s: FATAL. %s\n", c.Name, err)
					runtime.Goexit()
				}
				c.Log.Printf("%s: Saved content of url '%s'\n", c.Name, urlpath)

				// set key value to false as url is now processed
				c.Queue.SetMapValue(urlpath, false)
			} else {
				// else update LastChecked field
				err = c.updateLastCheckedDate(urlpath, time.Now())
				if err != nil {
					c.Log.Printf("%s: FATAL. %s\n", c.Name, err)
					runtime.Goexit()
				}
			}

			// close response body
			resp.Body.Close()

			// take rest for RequestDelay
			time.Sleep(c.RequestDelay)

			// reset startTime
			startTime = time.Now()
		}
	}
}

// savePageContent saves URL response body to models
func (c *Crawler) savePageContent(urlpath string, doc *goquery.Document) error {
	// GetByURL should not fail because whenever a new URL is encountered
	// it is saved to queue AND db
	uModel, err := c.Models.URLs.GetByURL(urlpath)
	if err != nil {
		return fmt.Errorf("%s: could not get URL from model: %v", c.Name, err)
	}
	contentStr, err := doc.Html()
	if err != nil {
		return fmt.Errorf("%s: could not read page content: %v", c.Name, err)
	}
	if len(contentStr) < 100 {
		c.Log.Printf("FATAL. Empty/no content. url: '%s'; len: %d", urlpath, len(contentStr))
		runtime.Goexit()
	}
	newPage := models.NewPage(uModel.ID, contentStr)
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

// updateLastCheckedDate updates the LastChecked field of URL
func (c *Crawler) updateLastCheckedDate(urlpath string, datetime time.Time) error {
	// GetByURL should not fail because whenever a new URL is encountered
	// it is saved to queue AND db
	uModel, err := c.Models.URLs.GetByURL(urlpath)
	if err != nil {
		return fmt.Errorf("%s: could not get URL '%s' from model: %v", c.Name, urlpath, err)
	}
	uModel.LastChecked = datetime
	if err = c.Models.URLs.Update(uModel); err != nil {
		return fmt.Errorf("%s: could not update URL model: %v", c.Name, err)
	}
	return nil
}

// fetchEmbeddedURLs will fetch all values in href attribute of <a> tag
func (c *Crawler) fetchEmbeddedURLs(doc *goquery.Document) ([]string, error) {
	hrefs := []string{}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if href, found := s.Attr("href"); found {
			href = strings.TrimSpace(href)
			// if href is not absolute add BaseURL to href
			if !internal.IsAbsoluteURL(href) && !internal.BeginsWith(href, invalidHrefPrefixs) {
				if !strings.HasPrefix(href, "/") {
					href = "/" + href
				}
				href = c.BaseURL.String() + href
			}
			// convert to lower to make queue case insensitive
			href = strings.ToLower(href)

			// if href is known to be invalid, ignore
			if _, knownInvalid := c.KnownInvalidURLs.cache.Load(href); !knownInvalid {
				hrefs = append(hrefs, href)
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
  - Not in ignore paths list
  - Not Disallowed by robots.txt
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

	// check if path in ignore paths list
	if internal.ContainsAny(parsedURL.Path, c.IgnorePatterns) {
		return false
	}

	// check if path is allowed by robots.txt
	if !grobotstxt.AgentAllowed(*c.robotsTxt, c.UserAgent, href) {
		c.Log.Printf("%s: Not allowed by robots.txt: %s\n", c.Name, href)
		return false
	}

	return true
}

// isMarkedURL checks whether the href should be processed
func (c *Crawler) isMarkedURL(href string) bool {
	return internal.ContainsAny(href, c.MarkedURLs)
}

// getURL fetchs the URL with c.UserAgent
func (c *Crawler) getURL(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.UserAgent)

	return client.Do(req)
}

// getRobotsTxt will return the <baseURL>/robots.txt file if found as string
func getRobotsTxt(baseUrl *url.URL, userAgent string) (*string, error) {

	urlString := fmt.Sprintf("%s://%s/robots.txt", baseUrl.Scheme, baseUrl.Host)
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not get robots.txt, error: %v", err)
	}

	// following google's policy, failure when HTTP Status 429 or >= 500
	// but following 10 redirections
	// see: https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt#http-status-codes
	if resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf(
			"could not get robots.txt, received HTTP status %d: %s",
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
		)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading robots.txt: %v", err)
	}
	defer resp.Body.Close()
	robotsTxt := string(respBytes)

	return &robotsTxt, nil
}
