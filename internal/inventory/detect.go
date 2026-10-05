// Package inventory detects the current machine's environment and which kit
// components are already installed.
package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// OS is a normalized operating system identifier.
type OS string

const (
	Linux   OS = "linux"
	Windows OS = "windows"
	Darwin  OS = "darwin"
)

// Environment describes the machine the tool is running on.
type Environment struct {
	OS            OS
	Arch          string
	IsWSL         bool
	Distro        string
	DistroVersion string
	Home          string
	ConfigDir     string
	Shell         string
	Hostname      string
}

// DetectEnv inspects the current machine and returns its environment.
func DetectEnv() Environment {
	env := Environment{
		OS:   OS(runtime.GOOS),
		Arch: runtime.GOARCH,
	}
	if home, err := os.UserHomeDir(); err == nil {
		env.Home = home
	}
	if h, err := os.Hostname(); err == nil {
		env.Hostname = h
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		env.Shell = filepath.Base(sh)
	}

	if env.OS == Windows {
		env.ConfigDir = os.Getenv("APPDATA")
		return env
	}

	env.IsWSL = detectWSL()
	env.ConfigDir = os.Getenv("XDG_CONFIG_HOME")
	if env.ConfigDir == "" && env.Home != "" {
		env.ConfigDir = filepath.Join(env.Home, ".config")
	}
	env.Distro, env.DistroVersion = detectDistro()
	return env
}

// detectWSL reports whether we are running inside the Windows Subsystem for Linux.
func detectWSL() bool {
	for _, p := range []string{"/proc/version", "/proc/sys/kernel/osrelease"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.ToLower(string(b))
		if strings.Contains(s, "microsoft") || strings.Contains(s, "wsl") {
			return true
		}
	}
	return false
}

// detectDistro reads NAME and VERSION_ID from /etc/os-release.
func detectDistro() (name, version string) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(val, `"`)
		switch key {
		case "NAME":
			name = val
		case "VERSION_ID":
			version = val
		}
	}
	return name, version
}
