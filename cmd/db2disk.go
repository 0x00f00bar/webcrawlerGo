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
)

var defaultPageSize = 20

type pageContent struct {
	url     string
	addedAt time.Time
	content string
	rn      int // row number from query
}

// saveDbContentToDisk copies page model's content field from DB to disk at path
func saveDbContentToDisk(
	ctx context.Context,
	db *sql.DB,
	baseurl *url.URL,
	path string,
	cutOffDate time.Time,
	markedPaths []string,
	logger *log.Logger,
) error {

	countQuery := `WITH LatestPages AS (
		SELECT u.url, p.id, p.added_at,
			ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY p.added_at DESC) AS rn
		FROM pages p
		JOIN urls u ON p.url_id = u.id
		WHERE u.is_monitored=true AND u.url LIKE $1 || '%'
		AND u.url LIKE '%' || $2 || '%'
		AND p.added_at <= $3
	)
	SELECT COUNT(*)
	FROM LatestPages
	WHERE rn = 1`

	contentPaginatedQuery := `WITH LatestPages AS (
		SELECT u.url, p.added_at, p.content,
			ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY p.added_at DESC) AS rn
		FROM pages p
		JOIN urls u ON p.url_id = u.id
		WHERE u.is_monitored=true AND u.url LIKE $1 || '%'
		AND u.url LIKE '%' || $2 || '%'
		AND p.added_at <= $3
	)
	SELECT *
	FROM LatestPages
	WHERE rn = 1
	LIMIT $4 OFFSET ($5 - 1) * $4;`

	internal.CreateDirIfNotExists(path)
	logger.Printf("Saving files to path: %s", path)

	// delete empty directory at path when no file saved due to err
	var fileSaved bool
	defer func() {
		if !fileSaved {
			err := os.Remove(path)
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
		timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var recordCount int

		commonArgs := []interface{}{baseurl.String(), markedURL, cutOffDate}

		// get total records for the marked url
		err := db.QueryRowContext(timeOutCtx, countQuery, commonArgs...).
			Scan(&recordCount)
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
			pageContents, err := getContentPage(
				ctx,
				db,
				contentPaginatedQuery,
				baseurl.String(),
				markedURL,
				cutOffDate,
				defaultPageSize,
				pageNum+1,
			)
			if err != nil {
				return err
			}

			// save pages
			err = savePageContent(pageContents, path)
			if err != nil {
				return err
			}
			fileSaved = true
		}
	}
	return nil
}

// getContentPage fetches []pageContents from db
func getContentPage(
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...interface{},
) ([]*pageContent, error) {
	timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(timeOutCtx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pageContents []*pageContent

	for rows.Next() {
		var pageContent pageContent

		err = rows.Scan(
			&pageContent.url,
			&pageContent.addedAt,
			&pageContent.content,
			&pageContent.rn,
		)

		if err != nil {
			return nil, err
		}

		pageContents = append(pageContents, &pageContent)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pageContents, nil
}

// savePageContent writes the fetched contents to disk
func savePageContent(pageContents []*pageContent, basePath string) error {
	// Unsafe filename characters regex
	unsafeChars := regexp.MustCompile(`[<>:"/\\|?*\ ]`)
	for _, pageContent := range pageContents {
		parsedURL, err := url.Parse(pageContent.url)
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
			pageContent.addedAt.Format(timeStampLayout),
		)
		err = os.WriteFile(completeFilePath, []byte(pageContent.content), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
