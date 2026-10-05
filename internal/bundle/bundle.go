// Package bundle captures live configuration into a portable, secret-sanitized
// archive that can be installed on another machine or target.
package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

// DefaultOutDir is where bundles are written when no output directory is given.
const DefaultOutDir = "bundles"

// maxFileSize bounds how large a single captured file may be.
const maxFileSize = 8 << 20 // 8 MiB

// skipDirs are directory names never descended into while capturing configs.
var skipDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	".cache":        true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	".mypy_cache":   true,
	".pytest_cache": true,
}

// Options configures an export run.
type Options struct {
	Manifest *manifest.Manifest
	Env      inventory.Environment
	Target   target.Target
	OutDir   string
	DryRun   bool
	Only     []string
}

// Redaction records a single secret that was replaced during sanitization.
type Redaction struct {
	File string `yaml:"file"`
	Key  string `yaml:"key"`
	Line int    `yaml:"line,omitempty"`
}

// Captured describes one file written into the bundle.
type Captured struct {
	ComponentID string      `yaml:"component"`
	Source      string      `yaml:"source"`
	ArchivePath string      `yaml:"archive_path"`
	Bytes       int64       `yaml:"bytes"`
	Redactions  []Redaction `yaml:"redactions,omitempty"`
}

// Result is the outcome of an export run.
type Result struct {
	BundlePath string
	Captured   []Captured
	Redactions []Redaction
	Warnings   []string
}

type metadata struct {
	Version    int         `yaml:"version"`
	Created    string      `yaml:"created"`
	Target     string      `yaml:"target"`
	Source     srcInfo     `yaml:"source"`
	Captured   []Captured  `yaml:"captured"`
	Redactions []Redaction `yaml:"redactions,omitempty"`
}

type srcInfo struct {
	OS     string `yaml:"os"`
	Arch   string `yaml:"arch"`
	Distro string `yaml:"distro,omitempty"`
}

// relativize rewrites a path under the home directory as "~/...", so bundles
// never carry a machine-specific absolute path or the user's account name.
func relativize(env inventory.Environment, p string) string {
	if env.Home != "" && strings.HasPrefix(p, env.Home) {
		return "~" + strings.TrimPrefix(p, env.Home)
	}
	return p
}

// Export captures the configured components for the current target. When
// DryRun is true it reads and sanitizes in memory to report exactly what would
// happen, but writes nothing.
func Export(opts Options) (*Result, error) {
	if opts.Manifest == nil {
		return nil, fmt.Errorf("bundle: nil manifest")
	}
	outDir := opts.OutDir
	if outDir == "" {
		outDir = DefaultOutDir
	}

	res := &Result{}
	name := "kit-bundle-" + time.Now().Format("20060102-150405")

	staging := ""
	if !opts.DryRun {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, fmt.Errorf("bundle: create output dir: %w", err)
		}
		staging = filepath.Join(outDir, name)
		if err := os.MkdirAll(staging, 0o755); err != nil {
			return nil, fmt.Errorf("bundle: create staging dir: %w", err)
		}
	}

	for _, c := range selectComponents(opts) {
		for _, cf := range c.Configs {
			src := target.ExpandPath(opts.Target, opts.Env, cf.Src)
			info, err := os.Stat(src)
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: source not found (%s)", c.ID, src))
				continue
			}
			if info.IsDir() {
				err = filepath.WalkDir(src, func(p string, d os.DirEntry, werr error) error {
					if werr != nil {
						return werr
					}
					if d.IsDir() {
						if skipDirs[d.Name()] {
							return filepath.SkipDir
						}
						return nil
					}
					rel, rerr := filepath.Rel(src, p)
					if rerr != nil {
						return rerr
					}
					return captureFile(c.ID, p, rel, opts, res, staging)
				})
			} else {
				err = captureFile(c.ID, src, filepath.Base(src), opts, res, staging)
			}
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %v", c.ID, err))
			}
		}
	}

	sort.Slice(res.Captured, func(i, j int) bool {
		return res.Captured[i].ArchivePath < res.Captured[j].ArchivePath
	})

	if opts.DryRun {
		return res, nil
	}

	meta := metadata{
		Version:    1,
		Created:    time.Now().Format(time.RFC3339),
		Target:     string(opts.Target),
		Source:     srcInfo{OS: string(opts.Env.OS), Arch: opts.Env.Arch, Distro: strings.TrimSpace(opts.Env.Distro + " " + opts.Env.DistroVersion)},
		Captured:   res.Captured,
		Redactions: res.Redactions,
	}
	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("bundle: encode metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "bundle.yaml"), yamlBytes, 0o644); err != nil {
		return nil, fmt.Errorf("bundle: write metadata: %w", err)
	}

	zipPath := filepath.Join(outDir, name+".zip")
	if err := zipDir(staging, zipPath); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(staging); err != nil {
		res.Warnings = append(res.Warnings, "could not remove staging dir: "+err.Error())
	}
	res.BundlePath = zipPath
	return res, nil
}

func captureFile(componentID, srcPath, rel string, opts Options, res *Result, staging string) error {
	archivePath := filepath.ToSlash(filepath.Join("configs", componentID, rel))

	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if info.Size() > maxFileSize {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s exceeds %d bytes, skipped", componentID, archivePath, maxFileSize))
		return nil
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	var reds []Redaction
	if bytes.IndexByte(data, 0) >= 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s looks binary, copied without sanitizing", componentID, archivePath))
	} else {
		data, reds = Sanitize(data, archivePath)
	}

	res.Captured = append(res.Captured, Captured{
		ComponentID: componentID,
		Source:      relativize(opts.Env, srcPath),
		ArchivePath: archivePath,
		Bytes:       int64(len(data)),
		Redactions:  reds,
	})
	res.Redactions = append(res.Redactions, reds...)

	if staging != "" {
		dest := filepath.Join(staging, filepath.FromSlash(archivePath))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func selectComponents(opts Options) []manifest.Component {
	if len(opts.Only) == 0 {
		return opts.Manifest.Components
	}
	want := make(map[string]bool, len(opts.Only))
	for _, id := range opts.Only {
		want[id] = true
	}
	var out []manifest.Component
	for _, c := range opts.Manifest.Components {
		if want[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

func zipDir(srcDir, zipPath string) (err error) {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	zw := zip.NewWriter(f)
	walkErr := filepath.WalkDir(srcDir, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(srcDir, p)
		if rerr != nil {
			return rerr
		}
		w, cerr := zw.Create(filepath.ToSlash(rel))
		if cerr != nil {
			return cerr
		}
		in, oerr := os.Open(p)
		if oerr != nil {
			return oerr
		}
		defer in.Close()
		_, cerr = io.Copy(w, in)
		return cerr
	})
	if walkErr != nil {
		_ = zw.Close()
		return walkErr
	}
	return zw.Close()
}
