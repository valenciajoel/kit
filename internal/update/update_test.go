package update

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.1.7", "v0.1.6", true},
		{"v0.2.0", "0.1.9", true},
		{"v0.1.6", "v0.1.6", false},
		{"v0.1.5", "v0.1.6", false},
		{"v0.1.0", "dev", true},
		{"garbage", "v0.1.6", false},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("linux", "amd64"); got != "kit-linux-amd64" {
		t.Fatalf("linux asset = %q", got)
	}
	if got := assetName("windows", "amd64"); got != "kit-windows-amd64.exe" {
		t.Fatalf("windows asset = %q", got)
	}
}

func TestRunCheckOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"kit-linux-amd64","browser_download_url":"http://example/kit-linux-amd64"}]}`))
	}))
	defer srv.Close()

	res, err := Run(Options{
		Client: srv.Client(), APIBase: srv.URL,
		CurrentVersion: "v0.1.6", AssetOS: "linux", AssetArch: "amd64", CheckOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Latest != "v9.9.9" || res.AssetURL == "" || res.Updated {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRunUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.6","assets":[]}`))
	}))
	defer srv.Close()

	res, err := Run(Options{Client: srv.Client(), APIBase: srv.URL, CurrentVersion: "v0.1.6"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.UpToDate {
		t.Fatalf("expected up to date, got %+v", res)
	}
}

func TestDownloadAndReplace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new-binary-bytes"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	exe := filepath.Join(dir, "kit")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := downloadAndReplace(srv.Client(), srv.URL+"/bin", exe); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary-bytes" {
		t.Fatalf("executable not replaced: %q", got)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(exe)
		if fi.Mode().Perm()&0o100 == 0 {
			t.Fatal("replaced executable is not executable")
		}
	}
}
