package collector

import (
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

// Redact removes the current user's home directory from the snapshot before it
// is written anywhere. The home directory contains a username, and a snapshot
// is meant to be shareable in a bug report.
//
// The replacement is `~`, which stays actionable: the reader still sees which
// directory is involved. Credential-bearing URLs are also scrubbed.
func (s *Snapshot) Redact(home string) {
	if s == nil {
		return
	}
	// Preserve documented identity visibility while scrubbing URLs in those values.
	var name, email string
	if s.Git != nil {
		if s.Git.UserName != nil {
			name = *s.Git.UserName
		}
		if s.Git.UserEmail != nil {
			email = *s.Git.UserEmail
		}
	}
	redactValue(reflect.ValueOf(s), home)
	if s.Git != nil {
		if s.Git.UserName != nil {
			*s.Git.UserName = RedactText(name, "")
		}
		if s.Git.UserEmail != nil {
			*s.Git.UserEmail = RedactText(email, "")
		}
	}
}

// Walking the typed snapshot prevents a newly added string field from silently
// bypassing redaction. Unexported time.Time internals are never changed.
func redactValue(value reflect.Value, home string) {
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			redactValue(value.Elem(), home)
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanSet() {
				redactValue(value.Field(i), home)
			}
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			redactValue(value.Index(i), home)
		}
	case reflect.String:
		if value.CanSet() {
			value.SetString(RedactText(value.String(), home))
		}
	}
}

var credentialURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s"'<>]+`)

// filepath.Join collapses URI double slashes in malformed path inputs. Scrub
// those credentials too, without treating a Windows drive letter as a scheme.
var collapsedURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]+:/[^/\s"'<>][^\s"'<>]*`)

func redactURLs(value, home string) string {
	value = collapsedURL.ReplaceAllStringFunc(value, func(raw string) string {
		scheme, rest, _ := strings.Cut(raw, ":/")
		u, err := url.Parse(scheme + "://" + rest)
		if err != nil {
			return "[redacted URL]"
		}
		if u.User == nil && u.RawQuery == "" && u.Fragment == "" {
			return raw
		}
		return u.Scheme + "://" + u.Host
	})
	value = credentialURL.ReplaceAllStringFunc(value, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[redacted URL]"
		}
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			u.User, u.RawQuery, u.Fragment = nil, "", ""
			u.Path, u.RawPath, u.ForceQuery = "", "", false
		} else {
			path := u.Path
			if strings.HasPrefix(path, "/") && len(home) >= 2 && home[1] == ':' {
				path = path[1:]
			}
			redacted := RedactHome(path, home)
			if redacted == path {
				return raw
			}
			u.Path, u.RawPath = "/"+redacted, ""
		}
		return u.String()
	})
	return strings.Map(func(ch rune) rune {
		if ch < 32 && ch != '\n' && ch != '\t' || ch == 127 {
			return '\uFFFD'
		}
		return ch
	}, value)
}

// RedactText also handles quoted paths and paths embedded in an OS error.
// Segment boundaries prevent replacing another user's home or a nested path.
func RedactText(value, home string) string {
	value = redactURLs(value, home)
	home = strings.TrimRight(home, `\/`)
	if home == "" {
		return value
	}
	needle := normalizeSeparators(home)
	var result strings.Builder
	last := 0
	for at := 0; at+len(home) <= len(value); at++ {
		if !strings.EqualFold(normalizeSeparators(value[at:at+len(home)]), needle) {
			continue
		}
		end := at + len(home)
		if at > 0 && isPathByte(value[at-1]) {
			continue
		}
		if end < len(value) && value[end] != '/' && value[end] != '\\' && !strings.ContainsRune(" \t\r\n\"':,;)]}", rune(value[end])) {
			continue
		}
		result.WriteString(value[last:at])
		result.WriteByte('~')
		last = end
		at = end - 1
	}
	if last == 0 {
		return value
	}
	result.WriteString(value[last:])
	return result.String()
}

func isPathByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.ContainsRune("/_-.\\", rune(b))
}

// RedactHome replaces a leading home directory with `~`. The comparison is
// case-insensitive and treats both path separators as equal, because a
// Windows path can appear with either and a false negative would leak a
// username. A partial segment match is not rewritten: `C:\Users\ann` must not
// rewrite `C:\Users\anna`.
func RedactHome(value, home string) string {
	if value == "" || home == "" {
		return value
	}
	home = strings.TrimRight(home, `\/`)
	if home == "" {
		return value
	}
	if len(value) < len(home) {
		return value
	}
	if !strings.EqualFold(normalizeSeparators(value[:len(home)]), normalizeSeparators(home)) {
		return value
	}
	rest := value[len(home):]
	if rest == "" {
		return "~"
	}
	if rest[0] != '\\' && rest[0] != '/' {
		return value
	}
	return "~" + rest
}

func normalizeSeparators(value string) string {
	return strings.ReplaceAll(value, `\`, "/")
}
