package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
)

type cmdFlags struct {
	baseURL        *url.URL      // -baseurl
	cutOffDate     time.Time     // -date
	updateDaysPast *int          // -days
	dbDSN          *string       // -db-dsn
	dbToDisk       bool          // -db2disk
	idleTimeout    time.Duration // -idle-time
	ignorePattern  []string      // -ignore
	markedURLs     []string      // -murls
	nCrawlers      *int          // -n
	savePath       string        // -path
	reqDelay       time.Duration // -req-delay
	retryTime      *int          // -retry
	userAgent      *string       // -ua
}

// pargeCmdFlags will parse cmd flags and validate them.
// Validation failure will exit the program.
func pargeCmdFlags(v *internal.Validator, logger *log.Logger) *cmdFlags {
	// define cmd args; usage message is formatted for better visibility on standard
	// terminal width of 80 chars
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
	dbDSN := flag.String(
		"db-dsn",
		"",
		"DSN string to database.\nSupported DSN: PostgreSQL DSN (optional)."+`
When empty crawler will use sqlite3 driver.`,
	)
	updateDaysPast := flag.Int(
		"days",
		1,
		"Days past which monitored URLs should be updated",
	)
	markedURLs := flag.String(
		"murls",
		"",
		`Comma ',' seperated string of marked url paths to save/update.
If the marked path is unmonitored in the database, the crawler
will mark the URL as monitored.
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
	logger.Println("Running crawler with the following options:")
	logger.Printf("%-16s: %s\n", "Base URL", cmdArgs.baseURL.String())
	logger.Printf("%-16s: %t\n", "DB-2-Disk", cmdArgs.dbToDisk)
	if cmdArgs.dbToDisk {
		fmt.Printf("%-16s: %s\n", "Save path", cmdArgs.savePath)
		fmt.Printf("%-16s: %s\n", "Cutoff date", cmdArgs.cutOffDate)
		fmt.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
		logger.Printf("%-16s: %s\n", "Save path", cmdArgs.savePath)
		logger.Printf("%-16s: %s\n", "Cutoff date", cmdArgs.cutOffDate)
		logger.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
	} else {
		fmt.Printf("%-16s: %s\n", "User-Agent", *cmdArgs.userAgent)
		fmt.Printf("%-16s: %d day(s)\n", "Update interval", *cmdArgs.updateDaysPast)
		fmt.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
		fmt.Printf("%-16s: %s\n", "Ignored Pattern", strings.Join(cmdArgs.ignorePattern, " "))
		fmt.Printf("%-16s: %d\n", "Crawler count", *cmdArgs.nCrawlers)
		fmt.Printf("%-16s: %s\n", "Idle time", cmdArgs.idleTimeout)
		fmt.Printf("%-16s: %s\n", "Request delay", cmdArgs.reqDelay)
		logger.Printf("%-16s: %s\n", "User-Agent", *cmdArgs.userAgent)
		logger.Printf("%-16s: %d day(s)\n", "Update interval", *cmdArgs.updateDaysPast)
		logger.Printf("%-16s: %s\n", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " "))
		logger.Printf("%-16s: %s\n", "Ignored Pattern", strings.Join(cmdArgs.ignorePattern, " "))
		logger.Printf("%-16s: %d\n", "Crawler count", *cmdArgs.nCrawlers)
		logger.Printf("%-16s: %s\n", "Idle time", cmdArgs.idleTimeout)
		logger.Printf("%-16s: %s\n", "Request delay", cmdArgs.reqDelay)
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

	return &cmdArgs
}
