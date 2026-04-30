package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAwikiProvenance(t *testing.T, root, repo, ref, commit string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".awiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InitProvenance(filepath.Join(root, ".awiki", "template.json"), repo, ref, "0.0.0", commit); err != nil {
		t.Fatal(err)
	}
}

func TestCheckAvailability_EnvDisable(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abc")
	res, err := CheckAvailability(root, "1", time.Now(), func(repo, ref string) (string, error) {
		t.Fatal("ls-remote should not be called when env disable is set")
		return "", nil
	})
	if err != nil || !res.Skipped || res.Message != "" {
		t.Errorf("res=%#v err=%v want skipped no-message", res, err)
	}
}

func TestCheckAvailability_NoProvenance(t *testing.T) {
	root := t.TempDir()
	res, err := CheckAvailability(root, "", time.Now(), nil)
	if err != nil || !res.Skipped {
		t.Errorf("res=%#v err=%v want skipped", res, err)
	}
}

func TestCheckAvailability_ConfigOptOut(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abc")
	cfg := filepath.Join(root, ".awiki", "config")
	if err := os.WriteFile(cfg, []byte("# header\nno_template_check=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := CheckAvailability(root, "", time.Now(), func(repo, ref string) (string, error) {
		t.Fatal("ls-remote should not be called when config opt-out is set")
		return "", nil
	})
	if err != nil || !res.Skipped {
		t.Errorf("res=%#v err=%v want skipped", res, err)
	}
}

func TestCheckAvailability_FreshStamp(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abc")
	cacheDir := filepath.Join(root, ".awiki", "template-cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(cacheDir, "_check-stamp")
	now := time.Now()
	// Stamp is fresh: 1 hour old (cutoff is 7 days).
	if err := os.WriteFile(stamp, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stamp, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	res, err := CheckAvailability(root, "", now, func(repo, ref string) (string, error) {
		t.Fatal("ls-remote should not be called when stamp is fresh")
		return "", nil
	})
	if err != nil || !res.Skipped {
		t.Errorf("res=%#v err=%v want skipped", res, err)
	}
}

func TestCheckAvailability_StaleStampTriggersCheck_NoUpdate(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abc")
	res, err := CheckAvailability(root, "", time.Now(), func(repo, ref string) (string, error) {
		// Same SHA upstream — no update.
		return "abc", nil
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Skipped || res.Message != "" {
		t.Errorf("res=%#v want no-message no-skip", res)
	}
}

func TestCheckAvailability_UpdateAvailable(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abcdef0123456789aaaa")
	now := time.Now()
	res, err := CheckAvailability(root, "", now, func(repo, ref string) (string, error) {
		return "ffffffffffffffff", nil
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Skipped {
		t.Fatal("expected non-skipped result")
	}
	if !strings.HasPrefix(res.Message, "LINT|info|template|") {
		t.Errorf("Message=%q want LINT|info|template|... prefix", res.Message)
	}
	// Short SHA (first 12 chars) must appear in the message.
	if !strings.Contains(res.Message, "abcdef012345") {
		t.Errorf("Message=%q missing 12-char short sha", res.Message)
	}
	// Stamp must be created/refreshed.
	if _, err := os.Stat(filepath.Join(root, ".awiki", "template-cache", "_check-stamp")); err != nil {
		t.Errorf("stamp not created: %v", err)
	}
}

func TestCheckAvailability_LsRemoteFailsSilently(t *testing.T) {
	root := t.TempDir()
	writeAwikiProvenance(t, root, "https://x/y.git", "main", "abc")
	res, err := CheckAvailability(root, "", time.Now(), func(repo, ref string) (string, error) {
		return "", errFake
	})
	if err != nil {
		t.Fatalf("err=%v want nil (silent skip)", err)
	}
	if !res.Skipped {
		t.Errorf("res=%#v want Skipped=true", res)
	}
}

type fakeErr struct{ msg string }

func (e fakeErr) Error() string { return e.msg }

var errFake = fakeErr{msg: "boom"}
