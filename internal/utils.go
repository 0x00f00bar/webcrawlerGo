package internal

import "net/url"

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
// <http/https>://google.com/query -> true
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
