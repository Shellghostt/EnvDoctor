package collector

import (
	"strings"
)

// Redact removes the current user's home directory from the snapshot before it
// is written anywhere. The home directory contains a username, and a snapshot
// is meant to be shareable in a bug report.
//
// The replacement is `~`, which stays actionable: the reader still sees which
// directory is involved. Nothing else is rewritten.
func (s *Snapshot) Redact(home string) {
	if s == nil || home == "" {
		return
	}
	home = strings.TrimRight(home, `\/`)
	if home == "" {
		return
	}

	if s.Shell != nil {
		s.Shell.Path = RedactHome(s.Shell.Path, home)
	}
	if s.Path != nil {
		for i := range s.Path.Entries {
			s.Path.Entries[i].Raw = RedactHome(s.Path.Entries[i].Raw, home)
		}
	}
	for i := range s.Runtimes {
		s.Runtimes[i].Path = RedactHome(s.Runtimes[i].Path, home)
	}
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
