package main

import (
	"fmt"
	"time"
)

const (
	logFolderName = "logs"

	sqliteDBName          = "crawler.db"
	dbMaxOpenConn         = 25
	dbMaxIdleConn         = 25
	dbMaxConnIdleDuration = 10 * time.Minute
	defaultPageSize       = 20

	// defaultTimeout is used in http.Client and timeout while shutting down.
	// DB queries are not aware of this timeout, db queries timeout at 5s,
	// keeping defaultTimeout lower than 5s may result in program exiting while a query is being processed.
	defaultTimeout  = 5 * time.Second
	dateLayout      = "2006-01-02"
	timeStampLayout = dateLayout + "_15-04-05"
)

var (
	sqliteDBWriterArgs = []string{
		"_busy_timeout=5000",
		"_foreign_keys=1",
		"_journal_mode=WAL",
		"mode=rwc",
		"_synchronous=1",
		"_loc=auto",
	}
	sqliteDBReaderArgs = []string{"_foreign_keys=1", "mode=ro", "_loc=auto"}

	defaultUserAgent  = fmt.Sprintf("webcrawlerGo/v%s - Web crawler in Go", version)
	defaultSavePath   = fmt.Sprintf("./OUT/%s", time.Now().Format("2006-01-02_15-04-05"))
	defaultCutOffDate = fmt.Sprint(time.Now().Format(dateLayout))
)
