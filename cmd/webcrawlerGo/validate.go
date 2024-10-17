package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/0x00f00bar/web-crawler/internal"
)

func validateFlags(v *internal.Validator, args *cmdFlags) {
	// validate baseurl
	v.Check(args.baseURL.String() != "", "baseurl", "must be provided")
	v.Check(internal.IsAbsoluteURL(args.baseURL.String()), "baseurl", "must be absolute URL")
	v.Check(internal.IsValidScheme(args.baseURL.Scheme), "baseurl", "scheme must be http/https")

	// validate crawler
	v.Check(
		*args.nCrawlers >= 1,
		"n",
		fmt.Sprintf("how do you crawl with %d crawlers?", *args.nCrawlers),
	)

	// validate update days past
	v.Check(
		*args.updateDaysPast >= 0,
		"days",
		fmt.Sprintf("invalid update interval: %d", *args.updateDaysPast),
	)

	// validate db-dsn
	v.Check(
		strings.Contains(*args.dbDSN, "postgres") || *args.dbDSN == "",
		"db-dsn",
		"only postgres dsn are supported, when empty will use sqlite3 driver",
	)

	// validate user-agent
	v.Check(*args.userAgent != "", "ua", "must be provided")

	// validate request delay & idle-time
	v.Check(args.reqDelay >= time.Microsecond, "req-delay", "cannot be less than 1ms")
	v.Check(args.idleTimeout >= time.Second, "idle-time", "cannot be less than 1s")

	// validate retry times
	v.Check(
		*args.retryTime >= 0,
		"retry",
		fmt.Sprintf("invalid retry time: %d. Should be >= 0.", *args.retryTime),
	)

	// validate cut-off date when save to disk flag is true
	if args.dbToDisk {
		v.Check(args.cutOffDate.Before(time.Now()), "date", "cannot be after today")
		v.Check(args.savePath != "", "path", "must be provided with 'db2disk' flag")
	}
}
