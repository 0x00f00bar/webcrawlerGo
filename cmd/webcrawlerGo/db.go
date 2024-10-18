package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
)

type dbConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
}

const (
	driverNamePgSQL  = "postgres"
	driverNameSQLite = "sqlite3"
	sqliteDBName     = "crawler.db"
)

var sqliteDBWriterArgs = []string{
	"_busy_timeout=5000",
	"_foreign_keys=1",
	"_journal_mode=WAL",
	"mode=rwc",
	"_synchronous=1",
	"_loc=auto",
}
var sqliteDBReaderArgs = []string{"_foreign_keys=1", "mode=ro", "_loc=auto"}

// openDB opens and tests a connection to database identified
// by the dsn string using driver
func openDB(driver string, dsn string, dbConfig *dbConfig) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	// setup db max connection idle time
	db.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)

	// setup db max connections
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)

	// setup db max idle connection
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// closeDBConns closes multiple DB connections
func closeDBConns(dbConns []*sql.DB) {
	for _, dbConn := range dbConns {
		dbConn.Close()
	}
}

// getDBConnections will create, test and return db connection(s)
// based on dsn
func getDBConnections(
	dsn string,
	logger *log.Logger,
) (driverName string, dbConns []*sql.DB, err error) {
	// driver used for db connection
	// var driverName string
	// actual db connection(s)
	// var dbConns []*sql.DB

	// when DSN is empty use sqlite3 driver
	if dsn == "" {
		logger.Println("Using sqlite3 driver")
		driverName = driverNameSQLite

		// writer config and dsn
		// open writer connection first as this will
		// create the db file if it doesn't exist
		dbconfWriter := &dbConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    2,
			ConnMaxIdleTime: dbMaxConnIdleDuration,
		}
		writerDSN := fmt.Sprintf(
			"file:./%s?%s",
			sqliteDBName,
			strings.Join(sqliteDBWriterArgs, "&"),
		)
		sqWriterDB, err := openDB(driverName, writerDSN, dbconfWriter)
		if err != nil {
			return "", nil, err
		}

		// reader config and dsn
		dbconfReader := &dbConfig{
			MaxOpenConns:    dbMaxOpenConn,
			MaxIdleConns:    dbMaxIdleConn,
			ConnMaxIdleTime: dbMaxConnIdleDuration,
		}
		readerDSN := fmt.Sprintf(
			"file:./%s?%s",
			sqliteDBName,
			strings.Join(sqliteDBReaderArgs, "&"),
		)
		sqReaderDB, err := openDB(driverName, readerDSN, dbconfReader)
		if err != nil {
			return "", nil, err
		}

		// keep reader first and writer second as this will
		// enable sqlite to clean up properly during connection closure.
		// Writer should be the last one to close the connection.
		dbConns = append(dbConns, sqReaderDB, sqWriterDB)

	} else if strings.Contains(dsn, "postgres") {
		logger.Println("Using postgres driver")
		driverName = driverNamePgSQL

		dbconf := &dbConfig{
			MaxOpenConns:    dbMaxOpenConn,
			MaxIdleConns:    dbMaxIdleConn,
			ConnMaxIdleTime: dbMaxConnIdleDuration,
		}
		pgDBConn, err := openDB(driverName, dsn, dbconf)
		if err != nil {
			return "", nil, err
		}
		dbConns = append(dbConns, pgDBConn)
	}

	return driverName, dbConns, nil
}

// saveDbContentToDisk copies page model's content field from DB to disk at path
func saveDbContentToDisk(
	ctx context.Context,
	pageDB models.PageModel,
	baseurl *url.URL,
	savePath string,
	cutOffDate time.Time,
	markedPaths []string,
	logger *log.Logger,
) error {

	internal.CreateDirIfNotExists(savePath)
	logger.Printf("Saving files to path: %s", savePath)

	// delete empty directory at path when no file saved due to err
	var fileSaved bool
	defer func() {
		if !fileSaved {
			err := os.Remove(savePath)
			if err != nil {
				logger.Println("Error removing directory:", err)
			}
		}
	}()

	// append "" to run the following loop atleast once
	// when no murls provided; will get all monitored urls
	if len(markedPaths) < 1 {
		markedPaths = append(markedPaths, "")
	}

	// save pages for each marked path
	for _, markedURL := range markedPaths {
		// 5 Second timeout ctx to use with db query
		recordCount, err := pageDB.GetLatestPageCount(ctx, baseurl, markedURL, cutOffDate)
		if err != nil {
			return err
		}
		msg := fmt.Sprintf("Saving %d records", recordCount)
		if markedURL != "" {
			msg += fmt.Sprintf(" for marked url '%s'", markedURL)
		}
		logger.Println(msg)

		totalPages := int(math.Ceil(float64(recordCount) / float64(defaultPageSize)))

		for pageNum := range totalPages {
			// get pages
			pageContents, err := pageDB.GetLatestPagesPaginated(
				ctx,
				baseurl,
				markedURL,
				cutOffDate,
				pageNum+1,
				defaultPageSize,
			)
			if err != nil {
				return err
			}

			// save pages
			err = savePageContent(pageContents, savePath)
			if err != nil {
				return err
			}
			fileSaved = true
		}
	}
	return nil
}

// savePageContent writes the fetched contents to disk
func savePageContent(pageContents []*models.PageContent, basePath string) error {
	// Unsafe filename characters regex
	unsafeChars := regexp.MustCompile(`[<>:"/\\|?*\ ]`)
	for _, pageContent := range pageContents {
		parsedURL, err := url.Parse(pageContent.URL)
		if err != nil {
			return err
		}
		urlPathSplit := strings.Split(parsedURL.Path, "/")
		pathLen := len(urlPathSplit)

		// Replace unsafe filename characters
		for i, path := range urlPathSplit {
			urlPathSplit[i] = unsafeChars.ReplaceAllString(path, "_")
		}

		// use last item as filename
		safeFileName := urlPathSplit[pathLen-1]
		// URL Encode the filename
		safeFileName = url.QueryEscape(safeFileName)

		// keep path upto second last item
		urlPathSplit = urlPathSplit[:pathLen-1]
		filePath := strings.Join(urlPathSplit, "/")

		// trim trailing / in basePath if exists
		basePath = strings.TrimRight(basePath, "/")

		internal.CreateDirIfNotExists(basePath + filePath)
		completeFilePath := fmt.Sprintf(
			"%s%s/%s_%s.html",
			basePath,
			filePath,
			safeFileName,
			pageContent.AddedAt.Format(timeStampLayout),
		)
		err = os.WriteFile(completeFilePath, []byte(pageContent.Content), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
