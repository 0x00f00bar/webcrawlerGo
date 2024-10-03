package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	webcrawler "github.com/0x00f00bar/web-crawler"
	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/models/psql"
	"github.com/0x00f00bar/web-crawler/queue"
)

var (
	version = "0.1.0"
	banner  = `
                            __         
  ______________ __      __/ /__  _____
 / ___/ ___/ __ '/ | /| / / / _ \/ ___/
/ /__/ /  / /_/ /| |/ |/ / /  __/ /    
\___/_/   \__,_/ |__/|__/_/\___/_/     
                                       `

	httpClientTimeout = 5 * time.Second
)

type cmdFlags struct {
	nCrawlers      *int
	baseURL        *url.URL
	updateDaysPast *int
	markedURLs     []string
	ignorePaths    []string
	dbDSN          *string
	reqDelay       time.Duration
	idleTimeout    time.Duration
	retryTime      *int
}

func main() {
	fmt.Printf(Cyan + "\nWho are we?      : " + Red + "web crawlers!" + Reset)
	fmt.Printf(Cyan + "\nWhat do we want? : " + Red + "To fetch all the pages!" + Reset)
	fmt.Println(Red + banner + Reset)
	fmt.Printf(Cyan+"v%s\n\n"+Reset, version)

	printVersion := flag.Bool("v", false, "Display app version")
	nCrawlers := flag.Int("n", 10, "Number of crawlers to invoke")
	idleTimeout := flag.String(
		"idle-time",
		"10s",
		"Idle time after which crawler quits when queue is empty.\nMin: 1s",
	)
	baseURL := flag.String(
		"baseurl",
		"",
		"Absolute base URL to crawl (required).\nE.g. <http/https>://<domain-name>",
	)
	reqDelay := flag.String("req-delay", "50ms", "Delay between subsequent requests.\nMin: 1ms")
	dbDSN := flag.String("db-dsn", "", "PostgreSQL DSN (required)")
	updateDaysPast := flag.Int(
		"days",
		1,
		"Days past which monitored URLs should be updated",
	)
	markedURLs := flag.String(
		"murls",
		"",
		`Comma ',' seperated string of marked url paths to save/update.
When empty, crawler will update monitored URLs from the model.`,
	)
	ignorePathList := flag.String("ignore", "", "Comma ',' seperated string of url paths to ignore.")
	retryFailedReq := flag.Int(
		"retry",
		2,
		`Number of times to retry failed GET requests.
With retry=2, crawlers will retry the failed GET urls
twice after initial failure.`,
	)

	flag.Parse()

	if *printVersion {
		fmt.Printf("Version %s\n", version)
		os.Exit(0)
	}

	// trim whitespace and drop trailing '/'
	*baseURL = strings.TrimSpace(*baseURL)
	*baseURL = strings.TrimRight(*baseURL, "/")

	v := internal.NewValidator()

	parsedBaseURL, err := url.Parse(*baseURL)
	if err != nil {
		fmt.Printf("could not parse base URL: %s\n", *baseURL)
		os.Exit(1)
	}

	markedURLSlice := getMarkedURLS(*markedURLs)

	// validate request delay and idle-time
	pRequestDelay, err := time.ParseDuration(*reqDelay)
	if err != nil {
		v.AddError("req-delay", err.Error())
	}
	pIdleTime, err := time.ParseDuration(*idleTimeout)
	if err != nil {
		v.AddError("idle-time", err.Error())
	}

	cmdArgs := cmdFlags{
		nCrawlers:      nCrawlers,
		baseURL:        parsedBaseURL,
		updateDaysPast: updateDaysPast,
		markedURLs:     markedURLSlice,
		ignorePaths:    seperateCmdArgs(*ignorePathList),
		dbDSN:          dbDSN,
		reqDelay:       pRequestDelay,
		idleTimeout:    pIdleTime,
		retryTime:      retryFailedReq,
	}

	validateFlags(v, &cmdArgs)
	if !v.Valid() {
		fmt.Fprintf(os.Stderr, "Invalid flag values:\n")
		for k, v := range v.Errors {
			fmt.Fprintf(os.Stderr, "%-9s : %s\n", k, v)
		}
		fmt.Println("")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Println(Red + "Flags parsed:" + Reset)
	fmt.Printf(Cyan+"%-16s: %s\n", "Base URL", cmdArgs.baseURL.String())
	fmt.Printf("%-16s: %d day(s)\n", "Update interval", *cmdArgs.updateDaysPast)
	fmt.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
	fmt.Printf("%-16s: %d\n", "Crawlers count", *cmdArgs.nCrawlers)
	fmt.Printf("%-16s: %s\n", "Idle time", cmdArgs.idleTimeout)
	fmt.Printf("%-16s: %s\n"+Reset, "Request delay", cmdArgs.reqDelay)

	if len(cmdArgs.markedURLs) < 1 {
		fmt.Println(
			Yellow + "WARNING: Marked URLs list is empty. Crawlers will update URLs only from model which are set for monitoring." + Reset,
		)
	}

	// init file and os.Stdout logger
	f, logger := initialiseLogger()
	defer f.Close()

	// init queue & push base url
	q := queue.NewQueue()
	q.Push(cmdArgs.baseURL.String())
	// fmt.Println(q.View(q.Size()))

	// init and test db
	db, err := openDB(*cmdArgs.dbDSN)
	if err != nil {
		logger.Fatalln(err)
	}
	logger.Println("DB connection OK.")
	defer db.Close()

	// init models
	var m models.Models
	psqlModels := psql.NewPsqlDB(db)
	m.URLs = psqlModels.URLModel
	m.Pages = psqlModels.PageModel

	// insert base URL to URL model if not present
	// when present will throw unique constraint error, which can be ignored
	var t time.Time
	u := models.NewURL(cmdArgs.baseURL.String(), t, t, false)
	_ = m.URLs.Insert(u)

	// get all urls from db, put all in queue's map
	loadedURLs := loadUrlsToQueue(*cmdArgs.baseURL, q, &m, *cmdArgs.updateDaysPast, logger)
	logger.Printf("Loaded %d URLs from model\n", loadedURLs)

	// if retry == 0, don't init request stats map
	var retryRequestStats map[string]int
	if *cmdArgs.retryTime > 0 {
		retryRequestStats = map[string]int{}
	} else {
		retryRequestStats = nil
	}

	// make a map of known invalid paths for efficient filtering
	knownInvalidURLSs := &webcrawler.InvalidURLCache{}

	crawlerCfg := &webcrawler.CrawlerConfig{
		Queue:            q,
		Models:           &m,
		BaseURL:          cmdArgs.baseURL,
		MarkedURLs:       cmdArgs.markedURLs,
		IgnorePaths:      cmdArgs.ignorePaths,
		RequestDelay:     cmdArgs.reqDelay,
		IdleTimeout:      cmdArgs.idleTimeout,
		Log:              logger,
		RetryTimes:       *cmdArgs.retryTime,
		FailedRequests:   retryRequestStats,
		KnownInvalidURLs: knownInvalidURLSs,
	}

	// init waitgroup
	var wg sync.WaitGroup

	// init n crawlers
	crawlerArmy, err := webcrawler.NNewCrawlers(int(*cmdArgs.nCrawlers), "crawler", crawlerCfg)
	if err != nil {
		logger.Fatalln(err)
	}

	modifiedTransport := http.DefaultTransport.(*http.Transport).Clone()
	modifiedTransport.MaxIdleConnsPerHost = 50

	httpClient := &http.Client{
		Timeout:   httpClientTimeout,
		Transport: modifiedTransport,
	}

	for _, crawler := range crawlerArmy {
		wg.Add(1)

		go func() {
			defer wg.Done()
			crawler.Crawl(httpClient)
		}()

	}

	// wait for crawlers
	wg.Wait()
	logger.Println("Done")
	fmt.Print(Red, "Done", Reset, "\n")
}
