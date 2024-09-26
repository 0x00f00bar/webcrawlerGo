package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/0x00f00bar/web-crawler/internal"
)

func validateFlags(args *cmdFlags) error {
	v := internal.NewValidator()

	// validate baseurl
	v.Check(args.baseURL != "", "baseurl", "must be provided")
	v.Check(internal.IsAbsoluteURL(args.baseURL), "baseurl", "must be absolute URL")
	parsedURL, err := url.Parse(args.baseURL)
	if err != nil {
		return err
	}
	v.Check(internal.IsValidScheme(parsedURL.Scheme), "baseurl", "scheme must be http/https")

	// validate crawler
	v.Check(args.nCrawlers >= 1, "n", fmt.Sprintf("how do you crawl with %d crawlers?", args.nCrawlers))

	// validate db-dsn
	v.Check(args.dbDSN != "", "db-dsn", "must be provided")

	if !v.Valid() {
		fmt.Fprintf(os.Stderr, "Invalid flag values:\n")
		for k, v := range v.Errors {
			fmt.Fprintf(os.Stderr, "%-7s : %s\n", k, v)
		}
		return fmt.Errorf("invalid flag values")
	}

	return nil
}
