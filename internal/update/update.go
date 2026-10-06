// Package update checks GitHub releases and replaces the running binary with a
// newer one.
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultRepo is the GitHub repository releases are fetched from.
const DefaultRepo = "valenciajoel/kit"

// Options configures an update run.
type Options struct {
	Repo           string
	CurrentVersion string
	AssetOS        string
	AssetArch      string
	CheckOnly      bool
	Force          bool
	Out            io.Writer
	Client         *http.Client
	APIBase        string
}

// Result describes what an update run found or did.
type Result struct {
	Current  string
	Latest   string
	AssetURL string
	Updated  bool
	UpToDate bool
}

type release struct {
	tag    string
	assets map[string]string
}

// Run checks the latest release and, unless CheckOnly is set, replaces the
// running executable when a newer version is available.
func Run(opts Options) (*Result, error) {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	repo := or(opts.Repo, DefaultRepo)
	apiBase := or(opts.APIBase, "https://api.github.com")
	assetOS := or(opts.AssetOS, runtime.GOOS)
	assetArch := or(opts.AssetArch, runtime.GOARCH)

	rel, err := latestRelease(client, apiBase, repo)
	if err != nil {
		return nil, err
	}

	res := &Result{Current: opts.CurrentVersion, Latest: rel.tag}
	if !opts.Force && !isNewer(rel.tag, opts.CurrentVersion) {
		res.UpToDate = true
		return res, nil
	}

	name := assetName(assetOS, assetArch)
	url, ok := rel.assets[name]
	if !ok {
		return nil, fmt.Errorf("release %s has no asset %q", rel.tag, name)
	}
	res.AssetURL = url

	if opts.CheckOnly {
		return res, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate current executable: %w", err)
	}
	if err := downloadAndReplace(client, url, exe); err != nil {
		return nil, err
	}
	res.Updated = true
	return res, nil
}

func latestRelease(client *http.Client, apiBase, repo string) (release, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("check release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("check release: HTTP %d", resp.StatusCode)
	}

	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return release{}, fmt.Errorf("parse release: %w", err)
	}

	r := release{tag: body.TagName, assets: make(map[string]string, len(body.Assets))}
	for _, a := range body.Assets {
		r.assets[a.Name] = a.URL
	}
	return r, nil
}

// downloadAndReplace writes the downloaded binary over exePath. The temp file
// is created next to the executable so the rename stays on one filesystem.
func downloadAndReplace(client *http.Client, url, exePath string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".kit-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s (permission?): %w", exePath, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := exePath + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return fmt.Errorf("replace: %w", err)
		}
		if err := os.Rename(tmpName, exePath); err != nil {
			_ = os.Rename(old, exePath)
			return fmt.Errorf("replace: %w", err)
		}
		_ = os.Remove(old)
		return nil
	}

	if err := os.Rename(tmpName, exePath); err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	return nil
}

func assetName(goos, goarch string) string {
	name := "kit-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// isNewer reports whether latest is strictly newer than current. An unparsable
// current version (for example "dev") counts as older.
func isNewer(latest, current string) bool {
	l, ok := parseSemver(latest)
	if !ok {
		return false
	}
	c, ok := parseSemver(current)
	if !ok {
		return true
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return [3]int{}, false
	}
	core := strings.SplitN(v, "-", 2)[0]
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, n := range nums {
		x, err := strconv.Atoi(n)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = x
	}
	return out, true
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
