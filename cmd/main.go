package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

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
)

type cmdFlags struct {
	nCrawlers      uint
	baseURL        string
	updateDaysPast uint
	markedURLs     string
	dbDSN          string
}

func main() {
	fmt.Printf(Cyan + "\nWhat do we want: " + Red + "To fetch all the pages!" + Reset)
	fmt.Println(Red + banner + Reset)
	fmt.Printf(Cyan+"v%s\n\n"+Reset, version)

	printVersion := flag.Bool("v", false, "Display app version")
	nCrawlers := flag.Uint("n", 10, "Number of crawlers to invoke")
	baseURL := flag.String("baseurl", "", "Base URL to crawl (required)")
	dbDSN := flag.String("db-dsn", "", "PostgreSQL DSN")
	updateDaysPast := flag.Uint(
		"days",
		1,
		"Days past which monitored URLs in models should be updated",
	)
	markedURLs := flag.String(
		"murls",
		"",
		`Comma seperated string of marked page paths to save/update in model
When empty, crawler will update URLs set as monitored in model`,
	)

	flag.Parse()

	if *printVersion {
		fmt.Printf("Version %s\n", version)
		os.Exit(0)
	}

	// trim whitespace and drop '/'
	*baseURL = strings.TrimSpace(*baseURL)
	*baseURL = strings.TrimRight(*baseURL, "/")

	cmdArgs := cmdFlags{
		nCrawlers:      *nCrawlers,
		baseURL:        *baseURL,
		updateDaysPast: *updateDaysPast,
		markedURLs:     *markedURLs,
		dbDSN:          *dbDSN,
	}

	err := validateFlags(&cmdArgs)
	if err != nil {
		fmt.Println("")
		flag.Usage()
		log.Fatalf("error: %v", err)
	}

	fmt.Println(Red + "Flags parsed:" + Reset)
	fmt.Printf(Cyan+"%-16s: %s\n", "Base URL", *baseURL)
	fmt.Printf("%-16s: %d day(s)\n", "Update interval", *updateDaysPast)
	fmt.Printf("%-16s: %s\n", "Marked URL(s)", *markedURLs)
	fmt.Printf("%-16s: %d\n"+Reset, "Crawlers count", *nCrawlers)

	if len(*markedURLs) < 1 {
		fmt.Println(
			Yellow + "WARNING: Marked URLs list is empty. Crawlers will update URLs only from DB which are set for monitoring." + Reset,
		)
	}

	// init file and os.Stdout logger
	logFileName := fmt.Sprintf(
		"./%s/logfile-%s.log",
		logFolderName,
		time.Now().Format("02-01-2006-15-04-05"),
	)
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	logger := log.New(io.MultiWriter(os.Stdout, f), "crawler: ", log.LstdFlags)

	logger.Println("LOLX")

	parsedBaseURL, err := url.Parse(*baseURL)
	if err != nil {
		logger.Fatalf("could not parse base URL: %s", *baseURL)
	}
	fmt.Println(parsedBaseURL)

	// push base url in queue
	q := queue.NewQueue()
	q.Push(*baseURL)

	// get all urls from db, put all in queue
	db, err := openDB(*dbDSN)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.Close()

	var m models.Models
	psqlModels := psql.NewPsqlDB(db)
	m.URLs = psqlModels.URLModel
	// m.Pages = psqlModels.PageModel
	t := time.Now()
	urlMod := models.NewURL("https://bankofbaroda.in/personal-banking/books", t, t, false)
	err = m.URLs.Insert(urlMod)
	if err != nil {
		fmt.Println(err)
	}
	dburls, err := m.URLs.GetAll("is_monitored")
	if err != nil {
		fmt.Println(err)
	}
	for _, ur := range dburls {
		fmt.Println(*ur)
	}

	// webcrawler.NewCrawler()
	// if is monitored true in db, set them as true, others false to not process
	// init waitgroup
	// init n crawlers
	// wait for crawlers

	// client := &http.Client{
	// 	Timeout: 5 * time.Second,
	// }

	// resp, err := getURL(prodURL, client)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// defer resp.Body.Close()
	// // body, err := io.ReadAll(resp.Body)
	// // if err != nil {
	// // 	log.Fatalln(err)
	// // }
	// for k, v := range resp.Header {
	// 	fmt.Println(k, v)
	// }

	// on load validate URL: not empty, of the base domain
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}
