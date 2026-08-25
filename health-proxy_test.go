package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExactSemver(t *testing.T) {
	valid := []string{"v0.0.0", "v7.2.141", "v4294967295.1.2"}
	for _, value := range valid {
		if _, err := parseSemver(value); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
	}
	invalid := []string{"", "7.2.1", "v1.2", "v1.2.3-beta", "v01.2.3", "v1.2.3+1", "v1.2.3\n", "v4294967296.1.1"}
	for _, value := range invalid {
		if _, err := parseSemver(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{{"v2.0.0", "v1.99.99", 1}, {"v1.2.3", "v1.2.3", 0}, {"v1.2.2", "v1.2.3", -1}} {
		got, err := compareVersion(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Fatalf("%s %s: %d %v", tc.a, tc.b, got, err)
		}
	}
}

func TestSoakClockSkewAndJitter(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	delay, err := soakDelay(now.Add(-5*time.Hour), now)
	if err != nil || delay != time.Hour {
		t.Fatalf("delay=%s err=%v", delay, err)
	}
	delay, err = soakDelay(now.Add(-7*time.Hour), now)
	if err != nil || delay != 0 {
		t.Fatalf("mature delay=%s err=%v", delay, err)
	}
	if _, err := soakDelay(now.Add(6*time.Minute), now); err == nil {
		t.Fatal("future timestamp accepted")
	}
	a := deterministicJitter("install-a", now)
	if a != deterministicJitter("install-a", now) || a < 0 || a > maxJitter {
		t.Fatalf("bad jitter %s", a)
	}
}

func TestCadenceAndClockClamp(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	u := &updater{now: func() time.Time { return now }, ledger: updateLedger{NextCheck: now.Add(25 * time.Hour)}}
	if got := u.nextWait(); got != 0 {
		t.Fatalf("future-skew wait %s", got)
	}
	u.ledger.NextCheck = now.Add(-time.Second)
	if got := u.nextWait(); got != 0 {
		t.Fatalf("overdue wait %s", got)
	}
	u.ledger.InstallationID = "cadence"
	u.ledger.NextCheck = time.Time{}
	u.root = t.TempDir()
	if err := u.scheduleAfterAttempt(true, 0); err != nil {
		t.Fatal(err)
	}
	if gap := u.ledger.NextCheck.Sub(now); gap < defaultInterval || gap > defaultInterval+maxJitter || gap >= 24*time.Hour {
		t.Fatalf("success gap %s", gap)
	}
}

func tarFixture(t *testing.T, headers []*tar.Header, bodies [][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tw := tar.NewWriter(gz)
	for i, header := range headers {
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(bodies) > i && bodies[i] != nil {
			if _, err := tw.Write(bodies[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestStrictTarExtraction(t *testing.T) {
	good := tarFixture(t,
		[]*tar.Header{
			{Name: "cli-proxy-api", Typeflag: tar.TypeReg, Mode: 0755, Size: 6},
			{Name: "LICENSE", Typeflag: tar.TypeReg, Mode: 0644, Size: 3},
		},
		[][]byte{[]byte("binary"), []byte("mit")},
	)
	got, err := extractBinary(good)
	if err != nil || string(got) != "binary" {
		t.Fatalf("good archive: %q %v", got, err)
	}
	cases := map[string][]byte{
		"traversal": tarFixture(t, []*tar.Header{{Name: "../cli-proxy-api", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}}, [][]byte{[]byte("x")}),
		"symlink":   tarFixture(t, []*tar.Header{{Name: "cli-proxy-api", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"}}, nil),
		"wrong":     tarFixture(t, []*tar.Header{{Name: "CLIProxyAPI", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}}, [][]byte{[]byte("x")}),
		"noexec":    tarFixture(t, []*tar.Header{{Name: "cli-proxy-api", Typeflag: tar.TypeReg, Mode: 0644, Size: 1}}, [][]byte{[]byte("x")}),
		"duplicate": tarFixture(t, []*tar.Header{{Name: "cli-proxy-api", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}, {Name: "cli-proxy-api", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}}, [][]byte{[]byte("x"), []byte("y")}),
	}
	for name, archive := range cases {
		if _, err := extractBinary(archive); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestChecksumsStrict(t *testing.T) {
	sum := strings.Repeat("a", 64)
	got, err := checksumFor([]byte(sum+"  wanted.tar.gz\n"+strings.Repeat("b", 64)+"  other\n"), "wanted.tar.gz")
	if err != nil || got != sum {
		t.Fatalf("%q %v", got, err)
	}
	for _, data := range []string{
		sum + "  wanted.tar.gz\n" + sum + "  wanted.tar.gz\n",
		"bad  wanted.tar.gz\n",
		sum + " wanted.tar.gz extra\n",
		sum + "  wanted.tar.gz\r\n",
	} {
		if _, err := checksumFor([]byte(data), "wanted.tar.gz"); err == nil {
			t.Fatalf("accepted checksum data %q", data)
		}
	}
}

func TestAssetShapeAndArchitecture(t *testing.T) {
	tag := "v7.2.141"
	name := archAsset(tag)
	release := githubRelease{TagName: tag, Assets: []releaseAsset{
		{Name: name, Size: 20 << 20, BrowserDownloadURL: "https://github.com/a"},
		{Name: "checksums.txt", Size: 1094, BrowserDownloadURL: "https://github.com/s"},
	}}
	if _, _, err := findAssets(release); err != nil {
		t.Fatal(err)
	}
	release.Assets = append(release.Assets, release.Assets[0])
	if _, _, err := findAssets(release); err == nil {
		t.Fatal("duplicate architecture asset accepted")
	}
}

func TestAllowlistRedirectAndFixtureBoundary(t *testing.T) {
	u := &updater{}
	for _, raw := range []string{
		"https://api.github.com/x", "https://github.com/x",
		"https://objects.githubusercontent.com/x", "https://release-assets.githubusercontent.com/x",
	} {
		v, _ := url.Parse(raw)
		if !u.allowedURL(v) {
			t.Fatalf("rejected %s", raw)
		}
	}
	for _, raw := range []string{"http://github.com/x", "https://github.com.evil/x", "https://user@github.com/x"} {
		v, _ := url.Parse(raw)
		if u.allowedURL(v) {
			t.Fatalf("accepted %s", raw)
		}
	}
	fixture := &updater{
		allowLocal: true, fixtureHost: "fixture.example.test", fixturePort: "443", fixtureScheme: "https",
	}
	for _, raw := range []string{
		"https://fixture.example.test/releases",
		"https://fixture.example.test:443/assets/candidate.tar.gz",
	} {
		v, _ := url.Parse(raw)
		if !fixture.allowedURL(v) {
			t.Fatalf("exact HTTPS fixture host rejected: %s", raw)
		}
		req := &http.Request{URL: v}
		if err := fixture.checkRedirect(req, []*http.Request{{}}); err != nil {
			t.Fatalf("exact HTTPS fixture redirect rejected: %s: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"http://fixture.example.test/releases",
		"https://Fixture.example.test/releases",
		"https://fixture.example.test.evil/releases",
		"https://user@fixture.example.test/releases",
		"https://fixture.example.test:444/releases",
		"ftp://fixture.example.test/releases",
	} {
		v, _ := url.Parse(raw)
		if fixture.allowedURL(v) {
			t.Fatalf("unsafe HTTPS fixture boundary accepted: %s", raw)
		}
		if err := fixture.checkRedirect(&http.Request{URL: v}, []*http.Request{{}}); err == nil {
			t.Fatalf("unsafe fixture redirect accepted: %s", raw)
		}
	}
	validFixtureURL, _ := url.Parse("https://fixture.example.test/releases")
	if err := fixture.checkRedirect(&http.Request{URL: validFixtureURL}, make([]*http.Request, 6)); err == nil {
		t.Fatal("redirect hop limit bypassed")
	}

	local := &updater{allowLocal: true, fixturePort: "1234", fixtureScheme: "http"}
	for _, raw := range []string{"http://127.0.0.1:1234/x", "http://localhost:1234/x"} {
		v, _ := url.Parse(raw)
		if !local.allowedURL(v) {
			t.Fatalf("loopback HTTP fixture rejected: %s", raw)
		}
	}
	for _, raw := range []string{
		"http://127.0.0.1:1235/x",
		"http://user@127.0.0.1:1234/x",
		"https://127.0.0.1:1234/x",
		"http://localhost.evil:1234/x",
	} {
		v, _ := url.Parse(raw)
		if local.allowedURL(v) {
			t.Fatalf("unsafe loopback fixture boundary accepted: %s", raw)
		}
	}
}

func TestFixtureAPIInitializationBoundary(t *testing.T) {
	t.Setenv("CLIPROXY_UPDATER_FIXTURE", "1")
	t.Setenv("CLIPROXY_UPDATER_FIXTURE_HOST", "fixture.example.test")
	for _, tc := range []struct {
		name, api string
		wantOK    bool
	}{
		{"exact-https", "https://fixture.example.test/releases", true},
		{"loopback-http", "http://127.0.0.1:18419/releases", true},
		{"localhost-http", "http://localhost:18419/releases", true},
		{"case-drift", "https://Fixture.example.test/releases", false},
		{"subdomain-drift", "https://fixture.example.test.evil/releases", false},
		{"userinfo", "https://user@fixture.example.test/releases", false},
		{"remote-http", "http://unconfigured.example.test/releases", false},
		{"wrong-scheme", "ftp://fixture.example.test/releases", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "update")
			embedded := filepath.Join(t.TempDir(), "embedded")
			if err := os.WriteFile(embedded, []byte("fixture-binary"), 0755); err != nil {
				t.Fatal(err)
			}
			_, err := newUpdater(root, embedded, "/tmp/config", "127.0.0.1:8317", tc.api)
			if tc.wantOK && err != nil {
				t.Fatalf("valid fixture API rejected: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("unsafe fixture API accepted")
			}
		})
	}
}

func TestFetchRateLimitStableSelectionAndFreshIdentityMetadata(t *testing.T) {
	now := time.Now().UTC()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.Header.Get("If-None-Match") != "" {
				t.Errorf("conditional metadata request could hide same-tag drift")
			}
			w.Header().Set("Retry-After", "17")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if calls == 2 {
			w.Header().Set("ETag", `"new"`)
			_ = json.NewEncoder(w).Encode([]githubRelease{
				{TagName: "v9.0.0-rc1", Prerelease: true, PublishedAt: now},
				{TagName: "v8.1.0", PublishedAt: now},
				{TagName: "v8.2.0", Draft: true, PublishedAt: now},
			})
			return
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	u := &updater{
		apiURL: server.URL, allowLocal: true,
		fixturePort: effectiveURLPort(serverURL), fixtureScheme: serverURL.Scheme,
		ledger: updateLedger{ETag: `"old"`},
		client: server.Client(),
	}
	_, _, retry, _, err := u.fetchRelease(context.Background())
	if err == nil || retry != 17*time.Second {
		t.Fatalf("rate limit retry=%s err=%v", retry, err)
	}
	release, etag, _, status, err := u.fetchRelease(context.Background())
	if err != nil || release.TagName != "v8.1.0" || etag != `"new"` || status != 200 {
		t.Fatalf("%+v %q %d %v", release, etag, status, err)
	}
	_, _, _, status, err = u.fetchRelease(context.Background())
	if err != nil || status != http.StatusNotModified {
		t.Fatalf("304 status=%d err=%v", status, err)
	}
}

func TestBootstrapLockJournalAndBounds(t *testing.T) {
	t.Setenv("CLIPROXY_UPDATER_FIXTURE", "1")
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	embedded := filepath.Join(t.TempDir(), "embedded")
	if err := os.WriteFile(embedded, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	u, err := newUpdater(root, embedded, "/tmp/config", "127.0.0.1:8317", "http://127.0.0.1/releases")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"embedded", "current"} {
		info, err := os.Stat(filepath.Join(root, "bin", name))
		if err != nil || info.Mode().Perm() != 0755 {
			t.Fatalf("%s mode %v err=%v", name, info.Mode().Perm(), err)
		}
	}
	info, _ := os.Stat(filepath.Join(root, "ledger.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatalf("ledger mode %v", info.Mode().Perm())
	}
	lock, err := u.updaterLock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.updaterLock(); err == nil {
		t.Fatal("concurrent lock accepted")
	}
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	_ = lock.Close()

	if err := os.WriteFile(filepath.Join(root, "bin", "staged"), []byte("candidate"), 0755); err != nil {
		t.Fatal(err)
	}
	u.ledger.Phase = "staged"
	u.ledger.Staged = &versionRecord{Tag: "v7.2.142", SHA256: strings.Repeat("a", 64)}
	if err := u.saveLedger(); err != nil {
		t.Fatal(err)
	}
	u2, err := newUpdater(root, embedded, "/tmp/config", "127.0.0.1:8317", "http://127.0.0.1/releases")
	if err != nil {
		t.Fatal(err)
	}
	if err := u2.recoverInterrupted(); err != nil {
		t.Fatal(err)
	}
	if u2.ledger.Phase != "idle" || u2.ledger.Staged != nil {
		t.Fatalf("journal not recovered: %+v", u2.ledger)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "staged")); !os.IsNotExist(err) {
		t.Fatal("staged residue retained")
	}

	tooLarge := filepath.Join(t.TempDir(), "large")
	f, err := os.Create(tooLarge)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxBinaryBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := atomicCopyFile(tooLarge, filepath.Join(root, "bin", "too-large"), 0755, maxBinaryBytes); err == nil {
		t.Fatal("oversized binary copied")
	}
}

func recoveryFakeBinary(t *testing.T, version string) string {
	t.Helper()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fake-child")
	script := "#!/bin/sh\nCLIPROXY_RECOVERY_HELPER=1 CLIPROXY_RECOVERY_VERSION=" +
		strconv.Quote(version) + " exec " + strconv.Quote(testBinary) +
		" -test.run=^TestRecoveryFakeChild$ -- \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRecoveryFakeChild(t *testing.T) {
	if os.Getenv("CLIPROXY_RECOVERY_HELPER") != "1" {
		return
	}
	version := os.Getenv("CLIPROXY_RECOVERY_VERSION")
	for _, arg := range os.Args {
		if arg == "--version" {
			fmt.Printf("CLIProxyAPI Version: %s, Commit: recovery-fixture, BuiltAt: fixture\n", version)
			return
		}
	}
	var configPath string
	for index, arg := range os.Args {
		if arg == "-config" && index+1 < len(os.Args) {
			configPath = os.Args[index+1]
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		os.Exit(2)
	}
	port := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "port:") {
			port, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "port:")))
		}
	}
	if port == 0 {
		os.Exit(2)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-signals
		_ = server.Close()
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(3)
	}
}

func freeTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func recoveryUpdater(t *testing.T) (*updater, string, string) {
	t.Helper()
	t.Setenv("CLIPROXY_UPDATER_FIXTURE", "1")
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	priorBinary := recoveryFakeBinary(t, "v7.2.141")
	candidateBinary := recoveryFakeBinary(t, "v7.2.142")
	port := freeTestPort(t)
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(config, []byte(fmt.Sprintf("port: %d\n", port)), 0600); err != nil {
		t.Fatal(err)
	}
	u, err := newUpdater(root, priorBinary, config, fmt.Sprintf("127.0.0.1:%d", port), "http://127.0.0.1/releases")
	if err != nil {
		t.Fatal(err)
	}
	return u, priorBinary, candidateBinary
}

func binaryRecord(t *testing.T, path, tag string) versionRecord {
	t.Helper()
	sum, err := fileSHA(path)
	if err != nil {
		t.Fatal(err)
	}
	return versionRecord{Tag: tag, SHA256: sum, Installed: time.Now().UTC()}
}

func assertVerifiedChildStarts(t *testing.T, u *updater, version string) {
	t.Helper()
	output, err := exec.Command(filepath.Join(u.root, "bin", "current"), "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "CLIProxyAPI Version: "+version+",") {
		t.Fatalf("version proof failed: err=%v output=%q", err, output)
	}
	if err := u.startCurrent(); err != nil {
		t.Fatal(err)
	}
	if err := waitHTTP(context.Background(), "http://"+u.upstream+"/healthz", "", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if !u.childHealthy() {
		t.Fatal("last verified child is not healthy")
	}
	if err := u.stopForCutover(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryEveryPersistedPhase(t *testing.T) {
	for _, phase := range []string{"", "idle", "download", "metadata", "staged", "probe", "cutover", "probation", "rollback"} {
		t.Run(phase, func(t *testing.T) {
			u, priorBinary, candidateBinary := recoveryUpdater(t)
			prior := binaryRecord(t, priorBinary, "v7.2.141")
			candidate := binaryRecord(t, candidateBinary, "v7.2.142")
			u.ledger.Current = prior
			u.ledger.Prior, u.ledger.Staged = nil, nil
			u.ledger.Phase, u.ledger.CrashJournal = phase, "interrupted fixture"
			if phase == "staged" || phase == "probe" || phase == "cutover" || phase == "probation" || phase == "rollback" {
				if err := atomicCopyFile(candidateBinary, filepath.Join(u.root, "bin", "staged"), 0755, maxBinaryBytes); err != nil {
					t.Fatal(err)
				}
				u.ledger.Staged = &candidate
			}
			if phase == "cutover" || phase == "probation" || phase == "rollback" {
				if err := atomicCopyFile(priorBinary, filepath.Join(u.root, "bin", "prior"), 0755, maxBinaryBytes); err != nil {
					t.Fatal(err)
				}
				u.ledger.Prior = &prior
			}
			if phase == "probation" || phase == "rollback" {
				if err := atomicCopyFile(candidateBinary, filepath.Join(u.root, "bin", "current"), 0755, maxBinaryBytes); err != nil {
					t.Fatal(err)
				}
				u.ledger.Current = candidate
			}
			if err := u.saveLedger(); err != nil {
				t.Fatal(err)
			}
			if err := u.recoverInterrupted(); err != nil {
				t.Fatal(err)
			}
			if u.ledger.Phase != "idle" || u.ledger.CrashJournal != "" || u.ledger.Prior != nil || u.ledger.Staged != nil {
				t.Fatalf("non-canonical recovery: %+v", u.ledger)
			}
			if u.ledger.Current.Tag != prior.Tag || u.verifyCurrentBinary() != nil {
				t.Fatalf("wrong recovered binary: %+v", u.ledger.Current)
			}
			var persisted updateLedger
			data, err := os.ReadFile(filepath.Join(u.root, "ledger.json"))
			if err != nil || json.Unmarshal(data, &persisted) != nil || persisted.Phase != "idle" ||
				persisted.CrashJournal != "" || persisted.Prior != nil || persisted.Staged != nil {
				t.Fatalf("persisted recovery is incoherent: err=%v ledger=%+v", err, persisted)
			}
			assertVerifiedChildStarts(t, u, "v7.2.141")
		})
	}
	for _, phase := range []string{"CUTOVER", "unknown", "rollback\n"} {
		t.Run("reject-"+strconv.Quote(phase), func(t *testing.T) {
			u, _, _ := recoveryUpdater(t)
			u.ledger.Phase = phase
			if err := u.recoverInterrupted(); err == nil {
				t.Fatal("malformed or unknown phase accepted")
			}
			if u.child != nil {
				t.Fatal("child started after fail-closed recovery")
			}
		})
	}
}

func TestPostStopLedgerSaveFailuresAlwaysRestartPrior(t *testing.T) {
	cases := []struct {
		name       string
		fail       func(updateLedger) bool
		semanticOK bool
	}{
		{"probation-journal", func(l updateLedger) bool {
			return l.Phase == "probation" && l.Current.Tag == "v7.2.142"
		}, true},
		{"acceptance-journal", func(l updateLedger) bool {
			return l.Phase == "idle" && l.Current.Tag == "v7.2.142" && l.Prior != nil
		}, true},
		{"rollback-terminal-journal", func(l updateLedger) bool {
			return l.Phase == "idle" && l.Current.Tag == "v7.2.141" && l.Prior == nil
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, _, candidateBinary := recoveryUpdater(t)
			u.proxyKey, u.managementKey = strings.Repeat("p", 64), strings.Repeat("m", 64)
			u.probation = 10 * time.Millisecond
			u.semanticCheck = func(context.Context, string, string, string) error {
				if tc.semanticOK {
					return nil
				}
				return errors.New("forced semantic rollback")
			}
			candidate := binaryRecord(t, candidateBinary, "v7.2.142")
			if err := atomicCopyFile(candidateBinary, filepath.Join(u.root, "bin", "staged"), 0755, maxBinaryBytes); err != nil {
				t.Fatal(err)
			}
			u.ledger.Staged = &candidate
			u.ledger.ChecksumsByTag[candidate.Tag] = strings.Repeat("a", 64)
			if err := u.saveLedger(); err != nil {
				t.Fatal(err)
			}
			if err := u.startCurrent(); err != nil {
				t.Fatal(err)
			}
			if err := waitHTTP(context.Background(), "http://"+u.upstream+"/healthz", "", 5*time.Second); err != nil {
				t.Fatal(err)
			}
			failed := false
			u.saveLedgerHook = func(ledger updateLedger) error {
				if !failed && tc.fail(ledger) {
					failed = true
					return errors.New("in-process test save failure")
				}
				return nil
			}
			if err := u.cutover(context.Background()); err == nil {
				t.Fatal("injected save failure was not surfaced")
			}
			if !failed {
				t.Fatal("target save route was not exercised")
			}
			u.saveLedgerHook = nil
			if u.ledger.Current.Tag != "v7.2.141" || u.ledger.Prior != nil || u.ledger.Staged != nil ||
				u.ledger.Phase != "idle" || u.ledger.CrashJournal != "" {
				t.Fatalf("rollback ledger is incoherent: %+v", u.ledger)
			}
			if !u.childHealthy() {
				t.Fatal("rollback returned with both children stopped")
			}
			output, err := exec.Command(filepath.Join(u.root, "bin", "current"), "--version").CombinedOutput()
			if err != nil || !strings.Contains(string(output), "CLIProxyAPI Version: v7.2.141,") {
				t.Fatalf("prior version was not restored: err=%v output=%q", err, output)
			}
			if err := u.stopForCutover(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNewerEmbeddedFallbackAdvancesWithoutLosingPrior(t *testing.T) {
	t.Setenv("CLIPROXY_UPDATER_FIXTURE", "1")
	originalVersion := embeddedVersion
	defer func() { embeddedVersion = originalVersion }()
	embeddedVersion = "v7.2.141"
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "embedded")
	if err := os.WriteFile(source, []byte("old-image-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	u, err := newUpdater(root, source, "/tmp/config", "127.0.0.1:8317", "http://127.0.0.1/releases")
	if err != nil {
		t.Fatal(err)
	}
	old := u.ledger.Current
	embeddedVersion = "v7.2.142"
	if err := os.WriteFile(source, []byte("new-image-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := u.refreshEmbeddedFallback(); err != nil {
		t.Fatal(err)
	}
	if u.ledger.Current.Tag != "v7.2.142" || u.ledger.Embedded.Tag != "v7.2.142" ||
		u.ledger.Prior == nil || u.ledger.Prior.Tag != old.Tag {
		t.Fatalf("bad fallback advancement: %+v", u.ledger)
	}
	prior, err := os.ReadFile(filepath.Join(root, "bin", "prior"))
	if err != nil || string(prior) != "old-image-binary" {
		t.Fatalf("prior=%q err=%v", prior, err)
	}
	if err := os.WriteFile(source, []byte("same-tag-drift"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := u.refreshEmbeddedFallback(); err == nil {
		t.Fatal("same embedded tag drift accepted")
	}
}

func TestRetryScheduleNeverExceedsRollingDay(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	u := &updater{
		root: root, now: func() time.Time { return now },
		ledger: updateLedger{
			Schema: ledgerSchema, InstallationID: "id",
			ChecksumsByTag: map[string]string{}, Quarantine: map[string]string{},
		},
	}
	if err := u.scheduleAfterAttempt(false, 72*time.Hour); err != nil {
		t.Fatal(err)
	}
	if gap := u.ledger.NextCheck.Sub(now); gap <= 0 || gap > maxAttemptGap {
		t.Fatalf("retry gap %s", gap)
	}
}

func TestProbeConfigHasSeparatedCredentialsAndPrivateBind(t *testing.T) {
	config := string(probeConfig(probePort))
	for _, required := range []string{
		`host: "127.0.0.1"`, fmt.Sprintf("port: %d", probePort),
		`secret-key: "probe-management-key-`, `- "probe-client-key-`,
		`disable-auto-update-panel: true`, `ws-auth: true`,
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("missing %s", required)
		}
	}
	if strings.Count(config, "probe-management-key") != 1 || strings.Count(config, "probe-client-key") != 1 {
		t.Fatal("probe key duplication")
	}
}

func TestLiveSemanticValidationUsesBothCredentialsAndPinnedUI(t *testing.T) {
	proxyKey := strings.Repeat("p", 32)
	managementKey := strings.Repeat("m", 32)
	ui := []byte("pinned-management-ui")
	uiSum := fmt.Sprintf("%x", sha256.Sum256(ui))
	mode := "good"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch r.URL.Path {
		case "/v1/models":
			if mode == "proxy-auth-bypass" {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			if token != proxyKey {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v0/management/config":
			if token != managementKey {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if mode == "invalid-management-config" {
				_, _ = w.Write([]byte(`not-json`))
				return
			}
			_, _ = w.Write([]byte(`{"config":{"safe":true}}`))
		case "/management.html":
			_, _ = w.Write(ui)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	if err := validateSemanticStateWithUIHash(context.Background(), server.URL, proxyKey, managementKey, uiSum); err != nil {
		t.Fatal(err)
	}
	if err := validateSemanticStateWithUIHash(context.Background(), server.URL, managementKey, proxyKey, uiSum); err == nil {
		t.Fatal("credential-role reversal passed live semantic validation")
	}
	mode = "proxy-auth-bypass"
	if err := validateSemanticStateWithUIHash(context.Background(), server.URL, proxyKey, managementKey, uiSum); err == nil {
		t.Fatal("unauthenticated proxy path passed semantic validation")
	}
	mode = "invalid-management-config"
	if err := validateSemanticStateWithUIHash(context.Background(), server.URL, proxyKey, managementKey, uiSum); err == nil {
		t.Fatal("invalid management config passed semantic validation")
	}
	mode = "good"
	if err := validateSemanticStateWithUIHash(context.Background(), server.URL, proxyKey, managementKey, strings.Repeat("0", 64)); err == nil {
		t.Fatal("management UI drift passed live semantic validation")
	}
}

func TestPrivateReadinessFailureIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if err := waitHTTP(context.Background(), server.URL, "", 10*time.Millisecond); err == nil {
		t.Fatal("unready candidate accepted")
	}
}

func TestFailureClassesQuarantineOnlyExactDeterministicIdentity(t *testing.T) {
	u := &updater{
		root: t.TempDir(),
		ledger: updateLedger{
			Schema: ledgerSchema, InstallationID: "failure-classes",
			ChecksumsByTag: map[string]string{}, Quarantine: map[string]string{}, Phase: "idle",
		},
	}
	sum := strings.Repeat("a", 64)
	u.handleCandidateFailure("v7.2.142", classified(failureTransient, "network retry", context.DeadlineExceeded))
	if len(u.ledger.Quarantine) != 0 || u.ledger.LastFailureClass != string(failureTransient) {
		t.Fatalf("transient failure was quarantined: %+v", u.ledger)
	}
	u.handleCandidateFailure("v7.2.142", deterministic("archive mismatch", sum, errors.New("bad archive")))
	if u.ledger.Quarantine[quarantineKey("v7.2.142", sum)] != "archive mismatch" {
		t.Fatalf("exact deterministic identity not quarantined: %+v", u.ledger.Quarantine)
	}
}

func TestEqualTagAlwaysVerifiesChecksumIdentityWithoutArchiveDownload(t *testing.T) {
	t.Setenv("CLIPROXY_UPDATER_FIXTURE", "1")
	originalVersion := embeddedVersion
	defer func() { embeddedVersion = originalVersion }()
	embeddedVersion = "v7.2.141"
	currentChecksum := strings.Repeat("a", 64)
	checksumCalls, archiveCalls := 0, 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			_ = json.NewEncoder(w).Encode([]githubRelease{{
				TagName: "v7.2.141", PublishedAt: time.Now().Add(-24 * time.Hour),
				Assets: []releaseAsset{
					{Name: archAsset("v7.2.141"), Size: 8, BrowserDownloadURL: server.URL + "/archive"},
					{Name: "checksums.txt", Size: 96, BrowserDownloadURL: server.URL + "/checksums"},
				},
			}})
		case "/checksums":
			checksumCalls++
			_, _ = fmt.Fprintf(w, "%s  %s\n", currentChecksum, archAsset("v7.2.141"))
		case "/archive":
			archiveCalls++
			_, _ = w.Write([]byte("unused"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	embedded := filepath.Join(t.TempDir(), "embedded")
	if err := os.WriteFile(embedded, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	u, err := newUpdater(root, embedded, "/tmp/config", "127.0.0.1:8317", server.URL+"/releases")
	if err != nil {
		t.Fatal(err)
	}
	u.client = server.Client()
	if err := u.checkAndApply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if checksumCalls != 1 || archiveCalls != 0 || u.ledger.ChecksumsByTag["v7.2.141"] != currentChecksum {
		t.Fatalf("identity calls checksums=%d archive=%d ledger=%+v", checksumCalls, archiveCalls, u.ledger)
	}
	currentChecksum = strings.Repeat("b", 64)
	if err := u.checkAndApply(context.Background()); err == nil {
		t.Fatal("same-tag checksum drift accepted")
	}
	if checksumCalls != 2 || archiveCalls != 0 ||
		u.ledger.Quarantine[quarantineKey("v7.2.141", currentChecksum)] == "" {
		t.Fatalf("same-tag drift not isolated: checksums=%d archive=%d quarantine=%+v", checksumCalls, archiveCalls, u.ledger.Quarantine)
	}
}

func TestFreeSpaceGateAndBackwardClockRestartRules(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	u := &updater{
		root: t.TempDir(), now: func() time.Time { return now }, monoNow: func() time.Time { return now },
		freeBytes: func(string) (uint64, error) { return minDiskHeadroom, nil },
		ledger: updateLedger{
			LastAttempt: now.Add(-time.Hour), NextCheck: now.Add(time.Hour),
		},
	}
	if err := u.requireFreeSpace(1); err == nil {
		t.Fatal("disk headroom failure accepted")
	}
	u.freeBytes = func(string) (uint64, error) { return uint64(minDiskHeadroom + maxBinaryBytes), nil }
	if err := u.requireFreeSpace(maxBinaryBytes); err != nil {
		t.Fatal(err)
	}
	for _, skew := range []time.Duration{time.Hour, 12 * time.Hour, 22 * time.Hour} {
		u.ledger.LastAttempt = now.Add(skew)
		u.ledger.NextCheck = now.Add(skew + time.Hour)
		if wait := u.nextWait(); wait != 0 {
			t.Fatalf("backward skew %s delayed restart by %s", skew, wait)
		}
	}
	u.ledger.LastAttempt = time.Time{}
	u.ledger.NextCheck = now.Add(time.Hour)
	if wait := u.nextWait(); wait != 0 {
		t.Fatalf("missing persisted attempt delayed restart by %s", wait)
	}
}

func TestStopForCutoverReapsChild(t *testing.T) {
	script := filepath.Join(t.TempDir(), "child")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntrap 'exit 0' TERM\nwhile :; do sleep 1; done\n"), 0755); err != nil {
		t.Fatal(err)
	}
	u := &updater{configPath: "/tmp/config", childExit: make(chan childResult, 8)}
	if err := u.startChild(script); err != nil {
		t.Fatal(err)
	}
	if err := u.stopForCutover(); err != nil {
		t.Fatal(err)
	}
	u.mu.Lock()
	child, done := u.child, u.childDone
	u.mu.Unlock()
	if child != nil || done != nil {
		t.Fatal("planned child was not cleared after reap")
	}
}
