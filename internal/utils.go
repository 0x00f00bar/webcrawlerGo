package internal

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Unsafe filename characters regex
var unsafeChars = regexp.MustCompile(`[<>:"/\\|?*\ ]`)

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

// PrefixString returns []string with every element of
// s prefixed with prefix
func PrefixString(s []string, prefix string) []string {
	if len(s) < 1 {
		return s
	}
	var prefixed []string
	for _, item := range s {
		prefixed = append(prefixed, prefix+item)
	}
	return prefixed
}

// SavePageContent will write the pageContent of the urlPath to saveDir
func SavePageContent(urlPath, pageContent, saveDir string, addedAt time.Time) error {
	parsedURL, err := url.Parse(urlPath)
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

	// trim trailing / in saveDir if exists
	saveDir = strings.TrimRight(saveDir, "/")

	CreateDirIfNotExists(saveDir + filePath)
	completeFilePath := fmt.Sprintf(
		"%s%s/%s_%s.html",
		saveDir,
		filePath,
		safeFileName,
		addedAt.Format("2006-01-02_15-04-05"),
	)
	err = os.WriteFile(completeFilePath, []byte(pageContent), 0644)
	if err != nil {
		return err
	}
	return nil
}
