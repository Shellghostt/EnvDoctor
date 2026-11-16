package collector

import (
	"time"
)

// Snapshot is an immutable, typed view of the machine at one moment. Every
// field is either collected fact or absent; nothing in it is guessed.
//
// The JSON shape is documented in docs/schema/snapshot-v1.schema.json and is
// versioned by SchemaVersion. Adding a field is a schema change.
type Snapshot struct {
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
	Errors            []CollectorError   `json:"errors,omitempty"`
}

// Platform describes the operating system the process runs on.
type Platform struct {
	OS       string `json:"os"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	Elevated bool   `json:"elevated"`
}

// Shell describes the shell that launched the process.
type Shell struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

// PathInfo is the raw PATH split into entries, in order.
type PathInfo struct {
	Entries []PathEntry `json:"entries"`
}

// PathEntry is one PATH element. Raw is exactly what the environment held;
// Exists and IsDir come from file metadata, never from executing the entry.
type PathEntry struct {
	Raw       string `json:"raw"`
	Exists    bool   `json:"exists"`
	IsDir     bool   `json:"isDir"`
	Position  int    `json:"position"`
	Duplicate bool   `json:"duplicate"`
}

// Runtime is one interpreter or toolchain installation. Filled from M2.
type Runtime struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Path            string `json:"path"`
	Manager         string `json:"manager"`
	Active          bool   `json:"active"`
	Expected        bool   `json:"expected,omitempty"`
	Missing         bool   `json:"missing,omitempty"`
	ResolvedPath    string `json:"resolvedPath,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	EnvironmentRoot string `json:"environmentRoot,omitempty"`
}

// Port is one listening socket. Filled from M2.
type Port struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	PID      int    `json:"pid,omitempty"`
	Process  string `json:"process,omitempty"`
}

// Docker keeps the M2 local-default fields and optional M3 API/context evidence.
type Docker struct {
	CLIVersion       string `json:"cliVersion"`
	DaemonReachable  bool   `json:"daemonReachable"`
	Context          string `json:"context"`
	Installed        bool   `json:"installed"`
	ProbeStatus      string `json:"probeStatus,omitempty"`
	ClientAPI        string `json:"clientApi,omitempty"`
	ClientMinAPI     string `json:"clientMinApi,omitempty"`
	ClientDefaultAPI string `json:"clientDefaultApi,omitempty"`
	ServerAPI        string `json:"serverApi,omitempty"`
	ServerMinAPI     string `json:"serverMinApi,omitempty"`
	APIOverride      string `json:"apiOverride,omitempty"`
	SelectedContext  string `json:"selectedContext,omitempty"`
	SelectedHost     string `json:"selectedHost,omitempty"`
}

// Git is the Git version and identity. Filled from M2.
type Git struct {
	Version         string  `json:"version"`
	UserName        *string `json:"userName"`
	UserEmail       *string `json:"userEmail"`
	Autocrlf        string  `json:"autocrlf"`
	IdentityChecked bool    `json:"identityChecked,omitempty"`
	UserNameOrigin  string  `json:"userNameOrigin,omitempty"`
	UserEmailOrigin string  `json:"userEmailOrigin,omitempty"`
	AutocrlfOrigin  string  `json:"autocrlfOrigin,omitempty"`
}

// Dependencies contains only supported, explicitly declared project evidence.
type Dependencies struct {
	ProjectRoot      string           `json:"projectRoot"`
	RequirementsFile string           `json:"requirementsFile,omitempty"`
	Requirements     []Dependency     `json:"requirements,omitempty"`
	Package          *PackageEvidence `json:"package,omitempty"`
	GitAutocrlf      string           `json:"gitAutocrlf,omitempty"`
}

type Dependency struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"` // present, missing or unknown
}

type PackageManager struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type PackageEvidence struct {
	Manifest  string          `json:"manifest"`
	Declared  *PackageManager `json:"declared,omitempty"`
	Runner    *PackageManager `json:"runner,omitempty"`
	Lockfiles []string        `json:"lockfiles,omitempty"`
}

type PythonEnvironment struct {
	Root          string `json:"root"`
	CanonicalRoot string `json:"canonicalRoot,omitempty"`
	MetadataKnown bool   `json:"metadataKnown"`
}

// Proxy never contains userinfo, a request path or a query. Probe defaults to untested.
type Proxy struct {
	Variable string `json:"variable"`
	URL      string `json:"url"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Probe    string `json:"probe"`
}

// CollectorError marks a collector that failed or timed out. Its component's
// rules are skipped; the rest of the diagnosis still runs.
type CollectorError struct {
	Collector string `json:"collector"`
	Message   string `json:"message"`
}
