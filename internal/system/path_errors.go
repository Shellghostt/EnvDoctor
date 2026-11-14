package system

import "os"

// IsMissingPath distinguishes absent or malformed paths from inspection failures.
// A permission or I/O error is not evidence that a PATH directory is missing.
func IsMissingPath(err error) bool {
	return os.IsNotExist(err) || isMalformedPath(err)
}
