package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManGeneratesPagesForEveryCommand(t *testing.T) {
	withConfigDir(t)
	dir := t.TempDir()

	root := newRootCommand()
	root.SetArgs([]string{"man", "--dir", dir})
	root.SetOut(new(discard))
	root.SetErr(new(discard))

	if err := root.Execute(); err != nil {
		t.Fatalf("markist man --dir %s error: %v", dir, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatal("man command wrote no files")
	}

	rootPage := filepath.Join(dir, "markist.1")
	info, err := os.Stat(rootPage)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", rootPage, err)
	}
	if info.Size() == 0 {
		t.Errorf("%s is empty", rootPage)
	}

	// Every visible subcommand should get its own page (markist-add.1, etc.).
	for _, name := range []string{"add", "login", "logout", "whoami", "search", "list", "shared", "docs", "version"} {
		page := filepath.Join(dir, "markist-"+name+".1")
		info, err := os.Stat(page)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", page, err)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", page)
		}
	}
}

// discard implements io.Writer, dropping everything written to it -- used
// to keep the man command's own (empty) stdout/stderr out of test output.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
