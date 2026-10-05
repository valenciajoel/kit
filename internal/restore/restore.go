// Package restore applies a previously exported bundle onto the current
// machine, rendering portable paths for the target.
package restore

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/valenciajoel/kit/internal/bundle"
	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/portable"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/kits"
)

// Options configures a restore run.
type Options struct {
	BundlePath string
	Manifest   *manifest.Manifest
	Env        inventory.Environment
	Target     target.Target
	DryRun     bool
	Force      bool
	Only       []string
}

// selected reports whether a component id passes the Only filter. An empty
// filter selects everything.
func (o Options) selected(id string) bool {
	if len(o.Only) == 0 {
		return true
	}
	for _, x := range o.Only {
		if x == id {
			return true
		}
	}
	return false
}

// Result is the outcome of a restore run.
type Result struct {
	Written  []string
	Skipped  []string
	Warnings []string
}

// Restore writes every captured config to its target destination. When
// BundlePath is empty it only writes seed configs. Existing files are left
// untouched unless Force is set.
func Restore(opts Options) (*Result, error) {
	res := &Result{}
	covered := make(map[string]bool)

	if opts.BundlePath != "" {
		meta, err := bundle.ReadMetadata(opts.BundlePath)
		if err != nil {
			return nil, err
		}
		zr, err := zip.OpenReader(opts.BundlePath)
		if err != nil {
			return nil, fmt.Errorf("restore: open %s: %w", opts.BundlePath, err)
		}
		defer zr.Close()

		index := make(map[string]*zip.File, len(zr.File))
		for _, f := range zr.File {
			index[filepath.ToSlash(f.Name)] = f
		}
		applyBundle(opts, meta, index, covered, res)
	}

	if opts.Manifest != nil {
		restoreSeeds(opts, covered, res)
	}

	sort.Strings(res.Written)
	sort.Strings(res.Skipped)
	return res, nil
}

// Seeds writes baseline configs for every component that declares one. It is
// the no-bundle path used by `kit setup` on a machine with nothing exported yet.
func Seeds(opts Options) (*Result, error) {
	opts.BundlePath = ""
	return Restore(opts)
}

func applyBundle(opts Options, meta *bundle.Metadata, index map[string]*zip.File, covered map[string]bool, res *Result) {
	for _, c := range meta.Captured {
		covered[c.ComponentID] = true
		if !opts.selected(c.ComponentID) {
			continue
		}

		destT := c.Dest[string(opts.Target)]
		if destT == "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: no destination for target %s", c.ArchivePath, opts.Target))
			continue
		}
		dest := target.ExpandPath(opts.Target, opts.Env, destT)
		if c.DestIsDir {
			dest = filepath.Join(dest, filepath.FromSlash(c.Rel))
		}

		zf, ok := index[c.ArchivePath]
		if !ok {
			res.Warnings = append(res.Warnings, c.ArchivePath+": missing from archive")
			continue
		}
		data, err := readZipEntry(zf)
		if err != nil {
			res.Warnings = append(res.Warnings, c.ArchivePath+": "+err.Error())
			continue
		}
		data = portable.Render(data, opts.Env)

		if !opts.DryRun {
			if exists(dest) && !opts.Force {
				res.Skipped = append(res.Skipped, dest+" (exists)")
				continue
			}
			if err := writeFile(dest, data); err != nil {
				res.Warnings = append(res.Warnings, "write "+dest+": "+err.Error())
				continue
			}
		}
		res.Written = append(res.Written, dest)
	}
}

// restoreSeeds writes a baseline config for components the bundle never
// captured, so a fresh machine is not left without one.
func restoreSeeds(opts Options, covered map[string]bool, res *Result) {
	for _, c := range opts.Manifest.Components {
		if covered[c.ID] || c.Seed == "" || len(c.Configs) == 0 || !opts.selected(c.ID) {
			continue
		}
		destT := c.Configs[0].Dest[string(opts.Target)]
		if destT == "" {
			continue
		}
		dest := target.ExpandPath(opts.Target, opts.Env, destT)
		seed, err := kits.FS.ReadFile(c.Seed)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("seed %s: %v", c.ID, err))
			continue
		}
		if !opts.DryRun {
			if exists(dest) && !opts.Force {
				res.Skipped = append(res.Skipped, dest+" (exists, seed)")
				continue
			}
			if err := writeFile(dest, seed); err != nil {
				res.Warnings = append(res.Warnings, "seed "+c.ID+": "+err.Error())
				continue
			}
		}
		res.Written = append(res.Written, dest+" (seed)")
	}
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func writeFile(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
