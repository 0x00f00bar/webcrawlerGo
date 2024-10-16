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

	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
)

type dbConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
}

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

// openDBConns opens and tests multiple DB connections using a common driver but
// with different dns and dbConfigs
func openDBConns(driver string, dbDSNs []string, dbConfigs []*dbConfig) ([]*sql.DB, error) {
	if len(dbDSNs) != len(dbConfigs) {
		return nil, fmt.Errorf("length of DSNs does not match with length of dbConfigs provided")
	}

	var dbConns []*sql.DB

	for i, dsn := range dbDSNs {
		dbConn, err := openDB(driver, dsn, dbConfigs[i])
		if err != nil {
			return nil, err
		}
		dbConns = append(dbConns, dbConn)
	}

	return dbConns, nil
}

// closeDBConns closes multiple DB connections
func closeDBConns(dbConns []*sql.DB) {
	for _, dbConn := range dbConns {
		dbConn.Close()
	}
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
