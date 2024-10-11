package internal

import (
	"net/url"
	"os"
	"strings"
)

// ValuePresent checks if needle is present in haystack
func ValuePresent(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// isAbsoluteURL checks if href is absolute URL
//
// e.g.
//
// <http|https>://google.com/query -> true
//
// /query -> false
func IsAbsoluteURL(href string) bool {
	parsed, err := url.Parse(href)
	return err == nil && (parsed.Scheme != "" && parsed.Host != "")
}

// isValidScheme tells if the scheme is valid
func IsValidScheme(scheme string) bool {
	return ValuePresent(scheme, []string{"http", "https"})
}

// beginsWith returns true if s begins with any of the
// strings in the provided slice
func BeginsWith(s string, testStr []string) bool {
	for _, test := range testStr {
		if strings.HasPrefix(s, test) {
			return true
		}
	}
	return false
}

// ContainsAny checks if str contains any of the substrings
func ContainsAny(str string, substrings []string) bool {
	for _, sub := range substrings {
		if sub != "" && strings.Contains(str, sub) {
			return true
		}
	}
	return false
}

// CreateDirIfNotExists will create a directory at path if
// it doesn't exist using os.MkdirAll
func CreateDirIfNotExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0744)
		if err != nil {
			panic(err)
		}
	}
}
