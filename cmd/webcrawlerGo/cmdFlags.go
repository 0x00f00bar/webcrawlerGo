package main

import (
	"flag"
	"fmt"
	"io"
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
	updateHrefs    bool          // -update-hrefs
	runserver      bool          // -server
	verbose        bool          // -verbose
}

// parseCmdFlags will parse cmd flags and validate them.
// Validation failure will exit the program.
func parseCmdFlags(v *internal.Validator) *cmdFlags {
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
		"Output path to save the content of crawled web pages.\nApplicable only with 'db2disk' flag.",
	)
	cutOffDate := flag.String(
		"date",
		defaultCutOffDate,
		"Cut-off date upto which the latest crawled pages will be saved to disk.\nFormat: YYYY-MM-DD. Applicable only with 'db2disk' flag.\n",
	)
	updateHrefs := flag.Bool(
		"update-hrefs",
		false,
		`Use this flag to update embedded HREFs in all saved and alive URLs
belonging to the baseurl.`,
	)
	server := flag.Bool(
		"server",
		false,
		`Open a local server on port 8100 to manage db. If provided, all other
options will be ignored (except db-dsn and verbose).`,
	)
	verbose := flag.Bool("verbose", false, "Prints additional info while logging")

	flag.Parse()

	if *printVersion {
		fmt.Printf("Version %s\n", version)
		os.Exit(0)
	}

	if *server {
		// validate db-dsn
		v.Check(
			strings.Contains(*dbDSN, "postgres") || *dbDSN == "",
			"db-dsn",
			"only postgres dsn are supported, when empty will use sqlite3 driver",
		)
		if !v.Valid() {
			printInvalidFlagErrors(v)
		}
		return &cmdFlags{
			dbDSN:     dbDSN,
			runserver: *server,
			verbose:   *verbose,
		}
	}

	parsedBaseURL, err := trimAndParseURL(baseURL)
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
		updateHrefs:    *updateHrefs,
		verbose:        *verbose,
	}

	validateFlags(v, &cmdArgs)
	if !v.Valid() {
		printInvalidFlagErrors(v)
	}

	return &cmdArgs
}

func printInvalidFlagErrors(v *internal.Validator) {
	fmt.Println(redStyle.Render("Invalid flag values:"))
	for k, v := range v.Errors {
		fmt.Printf("%-9s : %s\n", k, v)
	}
	fmt.Println("")
	flag.Usage()
	os.Exit(1)
}

func logCmdArgs(cmdArgs *cmdFlags, f io.Writer) {
	printAndLog(printRed, f, "Running crawler with the following options:")
	printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Log file", currentLogFileName))
	printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Base URL", cmdArgs.baseURL.String()))
	printAndLog(printCyan, f, fmt.Sprintf("%-16s: %t", "DB-2-Disk", cmdArgs.dbToDisk))
	if cmdArgs.dbToDisk {
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Save path", cmdArgs.savePath))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Cutoff date", cmdArgs.cutOffDate))
		printAndLog(
			printCyan,
			f,
			fmt.Sprintf("%-16s: %s", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " ")),
		)
	} else {
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "User-Agent", *cmdArgs.userAgent))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %t", "Updating HREFs", cmdArgs.updateHrefs))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %d day(s)", "Update interval", *cmdArgs.updateDaysPast))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Marked URL(s)", strings.Join(cmdArgs.markedURLs, " ")))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Ignored Pattern", strings.Join(cmdArgs.ignorePattern, " ")))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %d", "Crawler count", *cmdArgs.nCrawlers))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Idle time", cmdArgs.idleTimeout))
		printAndLog(printCyan, f, fmt.Sprintf("%-16s: %s", "Request delay", cmdArgs.reqDelay))
	}

	if len(cmdArgs.markedURLs) < 1 {
		message := "WARNING: Marked URLs list is empty. "
		if cmdArgs.dbToDisk {
			message += "This will fetch all monitored URLs.\nTIP: Use -murls for filtering."
		} else {
			message += "Crawlers will update URLs only from model which are set for monitoring."
		}
		printYellow(message)
	}
}
