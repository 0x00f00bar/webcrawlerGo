package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	webcrawler "github.com/0x00f00bar/web-crawler"
	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/models/psql"
	"github.com/0x00f00bar/web-crawler/queue"
)

var (
	version = "0.6.0"
	banner  = `
                            __         
  ______________ __      __/ /__  _____
 / ___/ ___/ __ '/ | /| / / / _ \/ ___/
/ /__/ /  / /_/ /| |/ |/ / /  __/ /    
\___/_/   \__,_/ |__/|__/_/\___/_/     
                                       `

	// timeout used in http.Client and timeout while shutting down
	defaultTimeout    = 5 * time.Second
	defaultUserAgent  = fmt.Sprintf("web-crawler/v%s - Web-crawler in Go", version)
	defaultSavePath   = fmt.Sprintf("./OUT/%s", time.Now().Format("2006-01-02_15-04-05"))
	dateLayout        = "2006-01-02"
	timeStampLayout   = dateLayout + "_15-04-05"
	defaultCutOffDate = fmt.Sprint(time.Now().AddDate(0, 0, -1).Format(dateLayout))
)

type cmdFlags struct {
	nCrawlers      *int
	baseURL        *url.URL
	updateDaysPast *int
	markedURLs     []string
	ignorePattern  []string
	dbDSN          *string
	userAgent      *string
	reqDelay       time.Duration
	idleTimeout    time.Duration
	retryTime      *int
	dbToDisk       bool
	savePath       string
	cutOffDate     time.Time
}

func main() {
	fmt.Printf(Cyan + "\nWho are we?      : " + Red + "web crawlers!" + Reset)
	fmt.Printf(Cyan + "\nWhat do we want? : " + Red + "To crawl the web!" + Reset)
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
	userAgent := flag.String("ua", defaultUserAgent, "User-Agent string to use while crawling\n")
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
	ignorePatternList := flag.String(
		"ignore",
		"",
		"Comma ',' seperated string of url patterns to ignore.",
	)
	retryFailedReq := flag.Int(
		"retry",
		2,
		`Number of times to retry failed GET requests.
With retry=2, crawlers will retry the failed GET urls
twice after initial failure.`,
	)
	dbToDisk := flag.Bool(
		"db2disk",
		false,
		`Use this flag to write the latest crawled content to disk.
Customise using arguments 'path' and 'date'.
Crawler will exit after saving to disk.`,
	)
	savePath := flag.String(
		"path",
		defaultSavePath,
		"Output path to save the content of crawled web pages.\nApplicable only with 'save' flag.",
	)
	cutOffDate := flag.String(
		"date",
		defaultCutOffDate,
		"Cut-off date upto which the latest crawled pages will be saved to disk.\nFormat: YYYY-MM-DD. Applicable only with 'save' flag.\n",
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
		fmt.Printf("error: could not parse base URL: %s\n", err.Error())
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

	parsedCutOffDate, err := time.Parse(dateLayout, *cutOffDate)
	if err != nil {
		fmt.Printf("error: could not parse cut-off date: %s\n", err.Error())
		os.Exit(1)
	}
	// add 24 hours to cutoff time to get the latest pages for the whole date
	parsedCutOffDate = parsedCutOffDate.Add(24*time.Hour - 1*time.Second)

	cmdArgs := cmdFlags{
		nCrawlers:      nCrawlers,
		baseURL:        parsedBaseURL,
		updateDaysPast: updateDaysPast,
		markedURLs:     markedURLSlice,
		ignorePattern:  seperateCmdArgs(*ignorePatternList),
		dbDSN:          dbDSN,
		userAgent:      userAgent,
		reqDelay:       pRequestDelay,
		idleTimeout:    pIdleTime,
		retryTime:      retryFailedReq,
		dbToDisk:       *dbToDisk,
		savePath:       *savePath,
		cutOffDate:     parsedCutOffDate,
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

	fmt.Println(Red + "Running crawler with the following options:" + Reset)
	fmt.Printf(Cyan+"%-16s: %s\n", "Base URL", cmdArgs.baseURL.String())
	fmt.Printf("%-16s: %t\n", "DB-2-Disk", cmdArgs.dbToDisk)
	if cmdArgs.dbToDisk {
		fmt.Printf("%-16s: %s\n", "Save path", cmdArgs.savePath)
		fmt.Printf("%-16s: %s\n", "Cutoff date", cmdArgs.cutOffDate)
		fmt.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
	} else {
		fmt.Printf("%-16s: %s\n", "User-Agent", *cmdArgs.userAgent)
		fmt.Printf("%-16s: %d day(s)\n", "Update interval", *cmdArgs.updateDaysPast)
		fmt.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
		fmt.Printf("%-16s: %s\n", "Ignored Pattern", strings.Join(cmdArgs.ignorePattern, " "))
		fmt.Printf("%-16s: %d\n", "Crawler count", *cmdArgs.nCrawlers)
		fmt.Printf("%-16s: %s\n", "Idle time", cmdArgs.idleTimeout)
		fmt.Printf("%-16s: %s\n", "Request delay", cmdArgs.reqDelay)
	}
	fmt.Print(Reset)

	if len(cmdArgs.markedURLs) < 1 {
		message := Yellow + "WARNING: Marked URLs list is empty. "
		if cmdArgs.dbToDisk {
			fmt.Println(message + "This will save all monitored URLs.")
			fmt.Println("TIP: Use -murls for filtering.")
		} else {
			fmt.Println(message + "Crawlers will update URLs only from model which are set for monitoring.")
		}
		fmt.Print(Reset)
	}

	// init file and os.Stdout logger
	f, logger := initialiseLogger()
	defer f.Close()

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

	// init queue & push base url
	q := queue.NewQueue()
	q.Push(cmdArgs.baseURL.String())

	// create cancel context to use for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	go listenForSignals(cancel, q, logger)

	if cmdArgs.dbToDisk {
		err = saveDbContentToDisk(
			ctx,
			db,
			cmdArgs.baseURL,
			cmdArgs.savePath,
			cmdArgs.cutOffDate,
			cmdArgs.markedURLs,
			logger,
		)
		if err != nil {
			logger.Fatalf("Error while saving to disk: %v", err)
		}
		logger.Println("Transfer completed")
		os.Exit(0)
	}

	// insert base URL to URL model if not present
	// when present will throw unique constraint error, which can be ignored
	var t time.Time
	u := models.NewURL(cmdArgs.baseURL.String(), t, t, false)
	_ = m.URLs.Insert(u)

	// get all urls from db, put all in queue's map
	loadedURLs := loadUrlsToQueue(
		ctx,
		*cmdArgs.baseURL,
		q,
		&m,
		*cmdArgs.updateDaysPast,
		logger,
		cmdArgs.markedURLs,
	)
	logger.Printf("Loaded %d URLs from model\n", loadedURLs)

	// if retry == 0, don't init request stats map
	var retryRequestStats map[string]int
	if *cmdArgs.retryTime > 0 {
		retryRequestStats = map[string]int{}
	} else {
		retryRequestStats = nil
	}

	crawlerCfg := &webcrawler.CrawlerConfig{
		Queue:          q,
		Models:         &m,
		BaseURL:        cmdArgs.baseURL,
		UserAgent:      *cmdArgs.userAgent,
		MarkedURLs:     cmdArgs.markedURLs,
		IgnorePatterns: cmdArgs.ignorePattern,
		RequestDelay:   cmdArgs.reqDelay,
		IdleTimeout:    cmdArgs.idleTimeout,
		Log:            logger,
		RetryTimes:     *cmdArgs.retryTime,
		FailedRequests: retryRequestStats,
		Ctx:            ctx,
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
		Timeout:   defaultTimeout,
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

func listenForSignals(cancel context.CancelFunc, queue *queue.UniqueQueue, logger *log.Logger) {
	defer cancel()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// wait for signal
	s := <-quit
	// clear queue
	queue.Clear()

	timeOut := 3 * time.Second

	logger.Println("=============== SHUTDOWN INITIATED ===============")
	logger.Printf("%s signal received", s.String())
	logger.Printf("Will shutdown in %s\n", timeOut.String())
	time.Sleep(timeOut)
}
