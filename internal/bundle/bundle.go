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
	"github.com/valenciajoel/kit/internal/portable"
	"github.com/valenciajoel/kit/internal/target"
)

// DefaultOutDir is where bundles are written when no output directory is given.
const DefaultOutDir = "bundles"

// MetadataFile is the manifest describing a bundle's contents.
const MetadataFile = "bundle.yaml"

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

// Captured describes one file written into the bundle and where it should land.
type Captured struct {
	ComponentID string            `yaml:"component"`
	Source      string            `yaml:"source,omitempty"`
	Rel         string            `yaml:"rel,omitempty"`
	Dest        map[string]string `yaml:"dest,omitempty"`
	DestIsDir   bool              `yaml:"dest_is_dir,omitempty"`
	ArchivePath string            `yaml:"archive_path"`
	Bytes       int64             `yaml:"bytes"`
	Redactions  []Redaction       `yaml:"redactions,omitempty"`
}

// Result is the outcome of an export run.
type Result struct {
	BundlePath string
	Captured   []Captured
	Redactions []Redaction
	Warnings   []string
}

// Metadata is the bundle manifest written to MetadataFile.
type Metadata struct {
	Version    int         `yaml:"version"`
	Created    string      `yaml:"created"`
	Target     string      `yaml:"target"`
	Source     SrcInfo     `yaml:"source"`
	Captured   []Captured  `yaml:"captured"`
	Redactions []Redaction `yaml:"redactions,omitempty"`
}

// SrcInfo describes the machine a bundle was exported from.
type SrcInfo struct {
	OS     string `yaml:"os"`
	Arch   string `yaml:"arch"`
	Distro string `yaml:"distro,omitempty"`
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
					return captureFile(captureSpec{
						componentID: c.ID,
						srcPath:     p,
						rel:         filepath.ToSlash(rel),
						dest:        cf.Dest,
						destIsDir:   true,
						rewrites:    c.Rewrites,
					}, opts, res, staging)
				})
			} else {
				err = captureFile(captureSpec{
					componentID: c.ID,
					srcPath:     src,
					rel:         filepath.Base(src),
					dest:        cf.Dest,
					destIsDir:   false,
					rewrites:    c.Rewrites,
				}, opts, res, staging)
			}
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %v", c.ID, err))
			}
		}

		for _, rw := range c.Rewrites {
			if !rw.Bundle {
				continue
			}
			to := rw.To[string(opts.Target)]
			if to == "" {
				continue
			}
			src := target.ExpandPath(opts.Target, opts.Env, rw.From)
			if _, err := os.Stat(src); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: rewrite source not found (%s)", c.ID, src))
				continue
			}
			base := filepath.Base(filepath.FromSlash(to))
			if err := captureFile(captureSpec{
				componentID: c.ID,
				srcPath:     src,
				rel:         base,
				dest:        rw.To,
				destIsDir:   false,
				archivePath: filepath.ToSlash(filepath.Join("configs", c.ID, "_rewrites", base)),
			}, opts, res, staging); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: rewrite bundle: %v", c.ID, err))
			}
		}
	}

	sort.Slice(res.Captured, func(i, j int) bool {
		return res.Captured[i].ArchivePath < res.Captured[j].ArchivePath
	})

	if opts.DryRun {
		return res, nil
	}

	meta := Metadata{
		Version:    1,
		Created:    time.Now().Format(time.RFC3339),
		Target:     string(opts.Target),
		Source:     SrcInfo{OS: string(opts.Env.OS), Arch: opts.Env.Arch, Distro: strings.TrimSpace(opts.Env.Distro + " " + opts.Env.DistroVersion)},
		Captured:   res.Captured,
		Redactions: res.Redactions,
	}
	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("bundle: encode metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, MetadataFile), yamlBytes, 0o644); err != nil {
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

type captureSpec struct {
	componentID string
	srcPath     string
	rel         string
	dest        map[string]string
	destIsDir   bool
	archivePath string
	rewrites    []manifest.Rewrite
}

func captureFile(cs captureSpec, opts Options, res *Result, staging string) error {
	archivePath := cs.archivePath
	if archivePath == "" {
		archivePath = filepath.ToSlash(filepath.Join("configs", cs.componentID, cs.rel))
	}

	info, err := os.Stat(cs.srcPath)
	if err != nil {
		return err
	}
	if info.Size() > maxFileSize {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s exceeds %d bytes, skipped", cs.componentID, archivePath, maxFileSize))
		return nil
	}

	data, err := os.ReadFile(cs.srcPath)
	if err != nil {
		return err
	}

	var reds []Redaction
	if bytes.IndexByte(data, 0) >= 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s looks binary, copied without sanitizing", cs.componentID, archivePath))
	} else {
		data = portable.Normalize(data, opts.Env)
		data = applyRewrites(data, cs.rewrites, opts)
		data, reds = Sanitize(data, archivePath)
	}

	res.Captured = append(res.Captured, Captured{
		ComponentID: cs.componentID,
		Source:      relativize(opts.Env, cs.srcPath),
		Rel:         cs.rel,
		Dest:        cs.dest,
		DestIsDir:   cs.destIsDir,
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

// applyRewrites repoints third-party config references at their bundled path.
func applyRewrites(data []byte, rewrites []manifest.Rewrite, opts Options) []byte {
	for _, rw := range rewrites {
		to := rw.To[string(opts.Target)]
		if to == "" {
			continue
		}
		data = bytes.ReplaceAll(data, []byte(rw.From), []byte(to))
		if expanded := target.ExpandPath(opts.Target, opts.Env, rw.From); expanded != rw.From {
			data = bytes.ReplaceAll(data, []byte(expanded), []byte(to))
		}
	}
	return data
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

// relativize rewrites a path under the home directory as "~/...", so bundles
// never carry a machine-specific absolute path or the user's account name.
func relativize(env inventory.Environment, p string) string {
	if env.Home != "" && strings.HasPrefix(p, env.Home) {
		return "~" + strings.TrimPrefix(p, env.Home)
	}
	return p
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

// ReadMetadata opens a bundle archive and returns its manifest.
func ReadMetadata(zipPath string) (*Metadata, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("bundle: open %s: %w", zipPath, err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) != MetadataFile {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		var meta Metadata
		if err := yaml.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("bundle: parse %s: %w", MetadataFile, err)
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("bundle: %s not found in %s", MetadataFile, zipPath)
}
