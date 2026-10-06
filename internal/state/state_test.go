package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valenciajoel/kit/internal/inventory"
)

func testEnv(t *testing.T) inventory.Environment {
	t.Helper()
	home := t.TempDir()
	return inventory.Environment{OS: inventory.Linux, Home: home, ConfigDir: filepath.Join(home, ".config")}
}

func TestRecordFindRemove(t *testing.T) {
	s := &Store{Version: 1, dir: t.TempDir()}
	s.Record(Entry{Path: "/a", Component: "zellij", Hash: "h1"})
	s.Record(Entry{Path: "/a", Component: "zellij", Hash: "h2"}) // upsert by path

	if s.Len() != 1 {
		t.Fatalf("len = %d, want 1", s.Len())
	}
	e, ok := s.Find("/a")
	if !ok || e.Hash != "h2" {
		t.Fatalf("find = %+v ok=%v", e, ok)
	}
	s.Remove("/a")
	if s.Len() != 0 {
		t.Fatalf("len after remove = %d", s.Len())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	env := testEnv(t)
	s, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	s.Record(Entry{Path: "/a", Component: "zellij", Hash: Hash([]byte("x"))})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Find("/a"); !ok {
		t.Fatal("entry lost across save/load")
	}
}

func TestBackupExisting(t *testing.T) {
	env := testEnv(t)
	s, _ := Load(env)

	f := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(f, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	backup, err := s.BackupExisting(f)
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected a backup path")
	}
	data, _ := os.ReadFile(backup)
	if string(data) != "original" {
		t.Fatalf("backup content = %q", data)
	}
}

func TestBackupExistingMissing(t *testing.T) {
	env := testEnv(t)
	s, _ := Load(env)
	b, err := s.BackupExisting(filepath.Join(t.TempDir(), "nope"))
	if err != nil || b != "" {
		t.Fatalf("backup = %q, err = %v", b, err)
	}
}
