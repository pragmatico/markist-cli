package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShouldCheck(t *testing.T) {
	tests := []struct {
		name          string
		lastCheckedAt time.Time
		want          bool
	}{
		{"never checked", time.Time{}, true},
		{"checked 23h ago", time.Now().Add(-23 * time.Hour), false},
		{"checked 25h ago", time.Now().Add(-25 * time.Hour), true},
		{"checked just now", time.Now(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldCheck(tt.lastCheckedAt); got != tt.want {
				t.Errorf("ShouldCheck(%v) = %v, want %v", tt.lastCheckedAt, got, tt.want)
			}
		})
	}
}

func TestSkip(t *testing.T) {
	noEnv := func(string) string { return "" }

	tests := []struct {
		name        string
		jsonFlag    bool
		interactive bool
		getenv      func(string) string
		want        bool
	}{
		{"json flag set", true, true, noEnv, true},
		{"non-interactive", false, false, noEnv, true},
		{"MARKIST_NO_UPDATE_CHECK set", false, true, func(k string) string {
			if k == "MARKIST_NO_UPDATE_CHECK" {
				return "1"
			}
			return ""
		}, true},
		{"CI set", false, true, func(k string) string {
			if k == "CI" {
				return "true"
			}
			return ""
		}, true},
		{"interactive, no json, no env", false, true, noEnv, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Skip(tt.jsonFlag, tt.interactive, tt.getenv); got != tt.want {
				t.Errorf("Skip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func releaseListServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/pragmatico/markist-cli/releases" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func tagsBody(tags ...string) string {
	type rel struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	rels := make([]rel, len(tags))
	for i, tag := range tags {
		rels[i] = rel{TagName: tag}
	}
	data, _ := json.Marshal(rels)
	return string(data)
}

func TestCheckFindsNewerVersion(t *testing.T) {
	server := releaseListServer(t, tagsBody("v0.1.0", "v0.3.0", "v0.2.0", "nightly-9.9.9", "other/v5.0.0"))
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "0.2.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.3.0" {
		t.Errorf("LatestVersion = %q, want 0.3.0", result.LatestVersion)
	}
	if !result.HasUpdate {
		t.Errorf("HasUpdate = false, want true")
	}
	if result.CurrentVersion != "0.2.0" {
		t.Errorf("CurrentVersion = %q, want 0.2.0", result.CurrentVersion)
	}
}

func TestCheckUpToDate(t *testing.T) {
	server := releaseListServer(t, tagsBody("v0.3.0", "v0.2.0"))
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "0.3.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.HasUpdate {
		t.Errorf("HasUpdate = true, want false")
	}
}

func TestCheckAheadOfLatest(t *testing.T) {
	// A dev build off a future tag, or a pre-release install, shouldn't
	// claim an "update" is available when the running version is newer
	// than anything published.
	server := releaseListServer(t, tagsBody("v0.3.0"))
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "0.4.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.HasUpdate {
		t.Errorf("HasUpdate = true, want false")
	}
}

func TestCheckIgnoresDraftAndPrerelease(t *testing.T) {
	body := `[
		{"tag_name": "v0.5.0", "draft": true},
		{"tag_name": "v0.4.0", "prerelease": true},
		{"tag_name": "v0.2.0"}
	]`
	server := releaseListServer(t, body)
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.2.0" {
		t.Errorf("LatestVersion = %q, want 0.2.0 (draft/prerelease should be ignored)", result.LatestVersion)
	}
}

func TestCheckNoCLIReleases(t *testing.T) {
	server := releaseListServer(t, tagsBody("nightly", "web/v2.0.0"))
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.HasUpdate {
		t.Errorf("HasUpdate = true, want false when no vX.Y.Z release exists")
	}
}

func TestCheckUnparsableCurrentVersion(t *testing.T) {
	// "dev" (the default when built without -ldflags) can't be compared,
	// so Check should report the latest without claiming an update.
	server := releaseListServer(t, tagsBody("v0.3.0"))
	defer server.Close()

	result, err := Check(context.Background(), server.Client(), server.URL, "dev")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.HasUpdate {
		t.Errorf("HasUpdate = true, want false for an unparsable current version")
	}
	if result.LatestVersion != "0.3.0" {
		t.Errorf("LatestVersion = %q, want 0.3.0", result.LatestVersion)
	}
}

func TestCheckHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := Check(context.Background(), server.Client(), server.URL, "0.1.0"); err == nil {
		t.Fatal("Check() error = nil, want an error on HTTP 500")
	}
}

// slowRoundTripper blocks until the request's context is done, simulating a
// hung network call -- used to prove Check's internal timeout fires without
// the test itself sleeping for the real 2s CheckTimeout.
type slowRoundTripper struct{}

func (slowRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestCheckRespectsTimeout(t *testing.T) {
	client := &http.Client{Transport: slowRoundTripper{}}

	start := time.Now()
	_, err := checkWithTimeout(context.Background(), client, "http://example.invalid", "0.1.0", 20*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Check() error = nil, want a timeout error")
	}
	if elapsed > time.Second {
		t.Fatalf("Check() took %v, want it bounded by the injected timeout", elapsed)
	}
}

func TestCheckContextCancelled(t *testing.T) {
	server := releaseListServer(t, tagsBody("v0.3.0"))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Check(ctx, server.Client(), server.URL, "0.1.0"); err == nil {
		t.Fatal("Check() error = nil, want an error for an already-canceled context")
	}
}
