// Package config holds the resolved options for one EnvDoctor run.
//
// There is no configuration file and no environment variable that changes
// behaviour. Options come from command-line flags only, which keeps a shared
// team rule set reviewable: it is the flags in a script, not a hidden file.
package config

import (
	"fmt"
	"regexp"
	"time"
)

// SchemaVersion is the version of the snapshot and diagnosis document
// formats. It is written into every JSON document and checked by the tests.
const SchemaVersion = 1

// CollectorTimeout is the hard limit for a single collector. PLAN section 14
// says every collector must finish within two seconds or be reported as
// degraded; the same number is used as the deadline.
const CollectorTimeout = 2 * time.Second

// ToolName is used in reports and JSON documents.
const ToolName = "envdoctor"

var (
	ruleIDPattern    = regexp.MustCompile(`^ENV[0-9]{3}$`)
	componentPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Options is the result of parsing the diagnose flags.
type Options struct {
	JSON      bool
	Verbose   bool
	Component string
	Rule      string
}

// Validate rejects values that cannot possibly select anything. It does not
// check whether the rule exists: that needs the rule registry, which the
// caller owns.
func (o Options) Validate() error {
	if o.Rule != "" && !ruleIDPattern.MatchString(o.Rule) {
		return fmt.Errorf("--rule %q is not a rule id; expected the form ENV001", o.Rule)
	}
	if o.Component != "" && !componentPattern.MatchString(o.Component) {
		return fmt.Errorf("--component %q is not a component name; expected lowercase letters, digits or dashes", o.Component)
	}
	return nil
}
