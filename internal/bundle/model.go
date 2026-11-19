// Package bundle implements the separate allowlisted sharing format in ADR 0006.
// Archives are data: no member is extracted, executed or used as a live target.
package bundle

import (
	"errors"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/report"
)

const (
	Version              = 1
	Policy               = "allowlist-v1"
	MaxArchiveBytes      = 2_000_000
	MaxUncompressedBytes = 8 << 20
	MaxEntries           = 8
)

var (
	ErrInvalid   = errors.New("invalid diagnostic bundle")
	ErrBounds    = errors.New("diagnostic bundle exceeds a resource limit")
	ErrVersion   = errors.New("unsupported diagnostic bundle version")
	ErrIntegrity = errors.New("diagnostic bundle integrity check failed")
	ErrEvidence  = errors.New("diagnostic evidence cannot be represented by the sharing policy")
	ErrRead      = errors.New("cannot read diagnostic bundle")
	ErrWrite     = errors.New("cannot publish diagnostic bundle")
)

var memberLimits = map[string]int{
	"manifest.json": 64 << 10, "snapshot.json": 2 << 20,
	"report.json": 3 << 20, "report.md": 2 << 20,
	"versions.json": 64 << 10, "diagnostics.json": 512 << 10,
}

var requiredMembers = []string{"manifest.json", "snapshot.json", "report.json", "report.md", "versions.json"}

type Options struct{ IncludeLogs bool }

type Member struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	BundleVersion   int               `json:"bundleVersion"`
	SnapshotVersion int               `json:"snapshotVersion"`
	ReportVersion   int               `json:"reportVersion"`
	RedactionPolicy string            `json:"redactionPolicy"`
	Tool            report.Tool       `json:"tool"`
	CapturedAt      time.Time         `json:"capturedAt"`
	CaptureExitCode int               `json:"captureExitCode"`
	Omissions       []report.Omission `json:"omissions"`
	Members         []Member          `json:"members"`
}

// SharingSnapshot is deliberately separate from collector.Snapshot. New raw
// collector fields cannot start appearing in exports through struct copying.
type SharingSnapshot struct {
	SchemaVersion     int                `json:"schemaVersion"`
	CollectedAt       time.Time          `json:"collectedAt"`
	Platform          *Platform          `json:"platform,omitempty"`
	Shell             *Shell             `json:"shell,omitempty"`
	Path              *PathInfo          `json:"path,omitempty"`
	Runtimes          []Runtime          `json:"runtimes,omitempty"`
	Ports             []Port             `json:"ports,omitempty"`
	RequestedPorts    []int              `json:"requestedPorts,omitempty"`
	Docker            *Docker            `json:"docker,omitempty"`
	Git               *Git               `json:"git,omitempty"`
	Dependencies      *Dependencies      `json:"dependencies,omitempty"`
	PythonEnvironment *PythonEnvironment `json:"pythonEnvironment,omitempty"`
	Proxies           []Proxy            `json:"proxies,omitempty"`
	NoProxyConfigured bool               `json:"noProxyConfigured,omitempty"`
	Errors            []Failure          `json:"errors,omitempty"`
}

type Platform struct {
	OS       string `json:"os"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	Elevated bool   `json:"elevated"`
}
type Shell struct {
	Name string `json:"name"`
}
type PathInfo struct {
	Entries []PathEntry `json:"entries"`
}
type PathEntry struct {
	Token     string `json:"token"`
	Exists    bool   `json:"exists"`
	IsDir     bool   `json:"isDir"`
	Position  int    `json:"position"`
	Duplicate bool   `json:"duplicate"`
}
type Runtime struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	PathToken        string `json:"pathToken"`
	Manager          string `json:"manager"`
	Active           bool   `json:"active"`
	Expected         bool   `json:"expected,omitempty"`
	Missing          bool   `json:"missing,omitempty"`
	ResolvedToken    string `json:"resolvedToken,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	EnvironmentToken string `json:"environmentToken,omitempty"`
}
type Port struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
}
type Docker struct {
	CLIVersion       string `json:"cliVersion"`
	Installed        bool   `json:"installed"`
	DaemonReachable  bool   `json:"daemonReachable"`
	ProbeStatus      string `json:"probeStatus,omitempty"`
	ClientAPI        string `json:"clientApi,omitempty"`
	ClientMinAPI     string `json:"clientMinApi,omitempty"`
	ClientDefaultAPI string `json:"clientDefaultApi,omitempty"`
	ServerAPI        string `json:"serverApi,omitempty"`
	ServerMinAPI     string `json:"serverMinApi,omitempty"`
	APIOverride      string `json:"apiOverride,omitempty"`
}
type Git struct {
	Version             string `json:"version"`
	UserNamePresent     bool   `json:"userNamePresent"`
	UserEmailPresent    bool   `json:"userEmailPresent"`
	IdentityChecked     bool   `json:"identityChecked"`
	Autocrlf            string `json:"autocrlf"`
	AutocrlfOriginKnown bool   `json:"autocrlfOriginKnown"`
}
type Dependencies struct {
	ProjectToken         string           `json:"projectToken"`
	RequirementsDeclared bool             `json:"requirementsDeclared"`
	Requirements         []Dependency     `json:"requirements,omitempty"`
	Package              *PackageEvidence `json:"package,omitempty"`
	GitAutocrlf          string           `json:"gitAutocrlf,omitempty"`
}
type Dependency struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	PathToken string `json:"pathToken,omitempty"`
	Status    string `json:"status"`
}
type PackageManager struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
type PackageEvidence struct {
	ManifestPresent bool            `json:"manifestPresent"`
	Declared        *PackageManager `json:"declared,omitempty"`
	Runner          *PackageManager `json:"runner,omitempty"`
	Lockfiles       []string        `json:"lockfiles,omitempty"`
}
type PythonEnvironment struct {
	RootToken      string `json:"rootToken"`
	CanonicalToken string `json:"canonicalToken,omitempty"`
	MetadataKnown  bool   `json:"metadataKnown"`
}
type Proxy struct {
	Variable  string `json:"variable"`
	Scheme    string `json:"scheme"`
	HostToken string `json:"hostToken"`
	Port      int    `json:"port"`
	Probe     string `json:"probe"`
}
type Failure struct {
	Collector string `json:"collector"`
	Reason    string `json:"reason"`
}
type Diagnostic struct {
	Collector  string `json:"collector"`
	DurationMs int64  `json:"durationMs"`
	Status     string `json:"status"`
}
type Diagnostics struct {
	SchemaVersion int          `json:"schemaVersion"`
	Collectors    []Diagnostic `json:"collectors"`
}
type RuntimeVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Versions struct {
	SchemaVersion int              `json:"schemaVersion"`
	Tool          report.Tool      `json:"tool"`
	Runtimes      []RuntimeVersion `json:"runtimes"`
	Git           string           `json:"git,omitempty"`
	Docker        string           `json:"docker,omitempty"`
}

type Capture struct {
	Snapshot collector.Snapshot
	Source   report.Source
}
