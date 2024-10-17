package main

import (
	"fmt"
	"time"
)

var (
	logFolderName         = "logs"
	dbMaxConnIdleDuration = 10 * time.Minute
	dbMaxOpenConn         = 25
	dbMaxIdleConn         = 25
	defaultPageSize       = 20

	// defaultTimeout is used in http.Client and timeout while shutting down
	defaultTimeout    = 5 * time.Second
	defaultUserAgent  = fmt.Sprintf("webcrawlerGo/v%s - Web crawler in Go", version)
	defaultSavePath   = fmt.Sprintf("./OUT/%s", time.Now().Format("2006-01-02_15-04-05"))
	dateLayout        = "2006-01-02"
	timeStampLayout   = dateLayout + "_15-04-05"
	defaultCutOffDate = fmt.Sprint(time.Now().Format(dateLayout))
)
