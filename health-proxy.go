package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultAPI         = "https://api.github.com/repos/router-for-me/CLIProxyAPI/releases?per_page=20"
	defaultInterval    = 6 * time.Hour
	maxJitter          = 30 * time.Minute
	maxAttemptGap      = 23 * time.Hour
	releaseSoak        = 6 * time.Hour
	transientRetry     = 90 * time.Minute
	maxMetadataBytes   = 2 << 20
	maxChecksumsBytes  = 128 << 10
	maxArchiveBytes    = 96 << 20
	maxBinaryBytes     = 80 << 20
	maxArchiveEntries  = 8
	probePort          = 18318
	probeTimeout       = 20 * time.Second
	defaultProbation   = 30 * time.Second
	ledgerSchema       = 1
	updaterUserAgent   = "railway-cliproxyapi-runtime-updater/1"
	expectedBinaryName = "cli-proxy-api"
	managementUIHash   = "e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4"
	minDiskHeadroom    = 8 << 20
)

var embeddedVersion = "v7.2.141"

type semver struct{ major, minor, patch uint64 }

func parseSemver(raw string) (semver, error) {
	if len(raw) < 2 || raw[0] != 'v' || strings.ContainsAny(raw, "+- \t\r\n") {
		return semver{}, errors.New("not an exact stable semantic version")
	}
	parts := strings.Split(raw[1:], ".")
	if len(parts) != 3 {
		return semver{}, errors.New("not an exact stable semantic version")
	}
	var out semver
	dst := []*uint64{&out.major, &out.minor, &out.patch}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semver{}, errors.New("non-canonical semantic version")
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return semver{}, errors.New("not an exact stable semantic version")
			}
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return semver{}, errors.New("semantic version overflow")
		}
		*dst[i] = n
	}
	return out, nil
}

func compareVersion(a, b string) (int, error) {
	av, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	switch {
	case av.major != bv.major:
		if av.major < bv.major {
			return -1, nil
		}
	case av.minor != bv.minor:
		if av.minor < bv.minor {
			return -1, nil
		}
	case av.patch != bv.patch:
		if av.patch < bv.patch {
			return -1, nil
		}
	default:
		return 0, nil
	}
	return 1, nil
}

type releaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string         `json:"tag_name"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []releaseAsset `json:"assets"`
}

type versionRecord struct {
	Tag       string    `json:"tag"`
	SHA256    string    `json:"sha256"`
	Installed time.Time `json:"installed_at"`
}

type updateLedger struct {
	Schema            int               `json:"schema"`
	InstallationID    string            `json:"installation_id"`
	Embedded          versionRecord     `json:"embedded"`
	Current           versionRecord     `json:"current"`
	Prior             *versionRecord    `json:"prior,omitempty"`
	Staged            *versionRecord    `json:"staged,omitempty"`
	LastAttempt       time.Time         `json:"last_attempt,omitempty"`
	LastSuccess       time.Time         `json:"last_success,omitempty"`
	NextCheck         time.Time         `json:"next_check,omitempty"`
	ETag              string            `json:"etag,omitempty"`
	ChecksumsByTag    map[string]string `json:"checksums_by_tag"`
	Quarantine        map[string]string `json:"quarantine"`
	Phase             string            `json:"phase"`
	CrashJournal      string            `json:"crash_journal,omitempty"`
	ConsecutiveFail   int               `json:"consecutive_failures"`
	LastFailureClass  string            `json:"last_failure_class,omitempty"`
	LastFailureReason string            `json:"last_failure_reason,omitempty"`
}

type failureClass string

const (
	failureTransient     failureClass = "transient"
	failureDeterministic failureClass = "deterministic"
	failureSecurity      failureClass = "security"
	failureStorage       failureClass = "storage"
)

type updateFailure struct {
	class    failureClass
	reason   string
	checksum string
	err      error
}

func (f *updateFailure) Error() string { return f.reason }
func (f *updateFailure) Unwrap() error { return f.err }

func classified(class failureClass, reason string, err error) error {
	return &updateFailure{class: class, reason: reason, err: err}
}

func deterministic(reason, checksum string, err error) error {
	return &updateFailure{class: failureDeterministic, reason: reason, checksum: checksum, err: err}
}

type updater struct {
	root            string
	embeddedPath    string
	configPath      string
	upstream        string
	apiURL          string
	allowLocal      bool
	fixtureHost     string
	fixturePort     string
	fixtureScheme   string
	now             func() time.Time
	client          *http.Client
	probation       time.Duration
	proxyKey        string
	managementKey   string
	monoNow         func() time.Time
	attemptMono     time.Time
	nextMono        time.Time
	freeBytes       func(string) (uint64, error)
	semanticCheck   func(context.Context, string, string, string) error
	saveLedgerHook  func(updateLedger) error
	mu              sync.Mutex
	ledger          updateLedger
	child           *exec.Cmd
	childGeneration uint64
	plannedStop     uint64
	childDone       chan error
	childExit       chan childResult
	stopping        bool
}

type childResult struct {
	generation uint64
	err        error
}

func main() {
	listen := flag.String("listen", "0.0.0.0:8080", "public listen address")
	upstream := flag.String("upstream", "127.0.0.1:8317", "private upstream address")
	binary := flag.String("binary", "/CLIProxyAPI/CLIProxyAPI", "embedded upstream binary")
	config := flag.String("config", "/data/state/config.yaml", "upstream config")
	updateRoot := flag.String("update-root", "/data/update", "protected updater state")
	api := flag.String("release-api", defaultAPI, "official GitHub Releases endpoint")
	proxyKeyFIFO := flag.String("proxy-key-fifo", "", "one-use proxy key handoff")
	managementKeyFIFO := flag.String("management-key-fifo", "", "one-use management key handoff")
	flag.Parse()

	proxyKey, err := readSecretFIFO(*proxyKeyFIFO, 10001, 10001)
	if err != nil {
		log.Fatal("proxy credential handoff failed")
	}
	managementKey, err := readSecretFIFO(*managementKeyFIFO, 10001, 10001)
	if err != nil {
		log.Fatal("management credential handoff failed")
	}
	if !validRuntimeKey(proxyKey) || !validRuntimeKey(managementKey) || proxyKey == managementKey {
		log.Fatal("credential handoff validation failed")
	}
	if err := reapInheritedWriters(2, time.Second); err != nil {
		log.Fatal("credential handoff cleanup failed")
	}
	u, err := newUpdater(*updateRoot, *binary, *config, *upstream, *api)
	if err != nil {
		log.Fatalf("updater initialization failed: %v", err)
	}
	u.proxyKey, u.managementKey = proxyKey, managementKey
	if err := u.recoverInterrupted(); err != nil {
		log.Fatalf("updater recovery failed: %v", err)
	}
	if err := u.refreshEmbeddedFallback(); err != nil {
		log.Fatalf("embedded fallback reconciliation failed: %v", err)
	}
	if err := u.startCurrent(); err != nil {
		log.Fatalf("upstream start failed: %v", err)
	}

	target, err := url.Parse("http://" + *upstream)
	if err != nil {
		log.Fatalf("invalid private upstream: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !u.childHealthy() {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/", proxy)

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go u.updateLoop(ctx)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	exitCode := 0
	for {
		select {
		case sig := <-signals:
			u.stopChild(sig)
			goto shutdown
		case err := <-serverErr:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("public listener stopped")
				exitCode = 1
			}
			u.stopChild(syscall.SIGTERM)
			goto shutdown
		case result := <-u.childExit:
			if u.ignoreStaleExit(result) {
				continue
			}
			if err := u.rollbackAfterCrash(result.err); err != nil {
				log.Printf("upstream stopped and rollback was unavailable")
				exitCode = 1
				goto shutdown
			}
		}
	}
shutdown:
	cancel()
	ctxShutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = server.Shutdown(ctxShutdown)
	os.Exit(exitCode)
}

func reapInheritedWriters(expected int, timeout time.Duration) error {
	deadline, reaped := time.Now().Add(timeout), 0
	for time.Now().Before(deadline) {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		switch {
		case pid > 0:
			reaped++
			if reaped == expected {
				return nil
			}
		case err != nil && !errors.Is(err, syscall.ECHILD):
			return err
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	return errors.New("credential writer reap timeout")
}

func newUpdater(root, embeddedPath, configPath, upstream, apiURL string) (*updater, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("update root must be absolute and clean")
	}
	allowLocal := os.Getenv("CLIPROXY_UPDATER_FIXTURE") == "1"
	parsedAPI, err := url.Parse(apiURL)
	if err != nil {
		return nil, errors.New("invalid release API")
	}
	if !allowLocal && (apiURL != defaultAPI || parsedAPI.Scheme != "https" || parsedAPI.Hostname() != "api.github.com") {
		return nil, errors.New("release API outside official allowlist")
	}
	fixtureHost := os.Getenv("CLIPROXY_UPDATER_FIXTURE_HOST")
	if allowLocal {
		httpFixture := parsedAPI.Scheme == "http" &&
			(parsedAPI.Hostname() == "127.0.0.1" || parsedAPI.Hostname() == "localhost" ||
				(fixtureHost != "" && parsedAPI.Hostname() == fixtureHost))
		httpsFixture := parsedAPI.Scheme == "https" && fixtureHost != "" && parsedAPI.Hostname() == fixtureHost
		if parsedAPI.User != nil || (!httpFixture && !httpsFixture) {
			return nil, errors.New("fixture API outside exact configured boundary")
		}
	}
	for _, dir := range []string{root, filepath.Join(root, "bin"), filepath.Join(root, "probe")} {
		if err := secureMkdir(dir); err != nil {
			return nil, err
		}
	}
	u := &updater{
		root: root, embeddedPath: embeddedPath, configPath: configPath,
		upstream: upstream, apiURL: apiURL, allowLocal: allowLocal, fixtureHost: fixtureHost,
		fixturePort: effectiveURLPort(parsedAPI), fixtureScheme: parsedAPI.Scheme,
		now:           func() time.Time { return time.Now().UTC() },
		monoNow:       time.Now,
		freeBytes:     filesystemFreeBytes,
		semanticCheck: validateSemanticState,
		probation:     defaultProbation,
		childExit:     make(chan childResult, 8),
	}
	u.client = &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: u.checkRedirect,
	}
	if allowLocal {
		u.probation = 500 * time.Millisecond
	}
	if err := u.loadOrBootstrap(); err != nil {
		return nil, err
	}
	return u, nil
}

func validRuntimeKey(value string) bool {
	if len(value) < 32 || len(value) > 256 {
		return false
	}
	for _, char := range value {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("._~-", char)) {
			return false
		}
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{
		"your-api-key", "your-secret-key", "change-me", "changeme", "default",
		"example", "test-key", "proxy-key", "management-key",
	} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

func readSecretFIFO(path string, uid, gid int) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 || info.Mode().Perm() != 0600 {
		return "", errors.New("invalid secret handoff")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != uid || int(stat.Gid) != gid {
		return "", errors.New("invalid secret handoff owner")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	if err := os.Remove(path); err != nil {
		return "", err
	}
	deadline := time.Now().Add(5 * time.Second)
	data := make([]byte, 0, 257)
	buffer := make([]byte, 64)
	for time.Now().Before(deadline) {
		count, readErr := file.Read(buffer)
		if count > 0 {
			data = append(data, buffer[:count]...)
			if len(data) > 256 {
				return "", errors.New("secret handoff too large")
			}
			if bytes.Contains(data, []byte{'\n'}) {
				break
			}
		}
		if readErr != nil && !errors.Is(readErr, syscall.EAGAIN) && !errors.Is(readErr, io.EOF) {
			return "", readErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(data) < 2 || data[len(data)-1] != '\n' || bytes.Count(data, []byte{'\n'}) != 1 {
		return "", errors.New("incomplete secret handoff")
	}
	return string(data[:len(data)-1]), nil
}

func secureMkdir(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0700 {
			return fmt.Errorf("unsafe updater directory: %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Mkdir(path, 0700)
}

func filesystemFreeBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

func (u *updater) requireFreeSpace(required int64) error {
	if required <= 0 {
		return classified(failureStorage, "invalid storage reservation", nil)
	}
	free, err := u.freeBytes(u.root)
	if err != nil {
		return classified(failureStorage, "filesystem capacity check failed", err)
	}
	needed := uint64(required) + minDiskHeadroom
	if free < needed {
		return classified(failureStorage, "insufficient filesystem capacity", syscall.ENOSPC)
	}
	return nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > maxBinaryBytes || !info.Mode().IsRegular() {
		return "", errors.New("invalid binary")
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (u *updater) loadOrBootstrap() error {
	ledgerPath := filepath.Join(u.root, "ledger.json")
	if data, err := os.ReadFile(ledgerPath); err == nil {
		if len(data) > 1<<20 || json.Unmarshal(data, &u.ledger) != nil {
			return errors.New("invalid release ledger")
		}
		if u.ledger.Schema != ledgerSchema || u.ledger.InstallationID == "" ||
			u.ledger.ChecksumsByTag == nil || u.ledger.Quarantine == nil {
			return errors.New("unsupported release ledger")
		}
		if _, err := parseSemver(u.ledger.Current.Tag); err != nil {
			return err
		}
		return u.verifyCurrentBinary()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := parseSemver(embeddedVersion); err != nil {
		return errors.New("invalid embedded version")
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	embeddedDst := filepath.Join(u.root, "bin", "embedded")
	if err := atomicCopyFile(u.embeddedPath, embeddedDst, 0755, maxBinaryBytes); err != nil {
		return err
	}
	sum, err := fileSHA(embeddedDst)
	if err != nil {
		return err
	}
	currentDst := filepath.Join(u.root, "bin", "current")
	if err := atomicCopyFile(embeddedDst, currentDst, 0755, maxBinaryBytes); err != nil {
		return err
	}
	now := u.now()
	u.ledger = updateLedger{
		Schema: ledgerSchema, InstallationID: id,
		Embedded:       versionRecord{Tag: embeddedVersion, SHA256: sum, Installed: now},
		Current:        versionRecord{Tag: embeddedVersion, SHA256: sum, Installed: now},
		ChecksumsByTag: map[string]string{},
		Quarantine:     map[string]string{}, Phase: "idle", NextCheck: now,
	}
	return u.saveLedger()
}

func (u *updater) refreshEmbeddedFallback() error {
	sourceSum, err := fileSHA(u.embeddedPath)
	if err != nil {
		return err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	embeddedCmp, err := compareVersion(embeddedVersion, u.ledger.Embedded.Tag)
	if err != nil {
		return err
	}
	if embeddedCmp < 0 {
		return nil
	}
	if embeddedCmp == 0 {
		if sourceSum != u.ledger.Embedded.SHA256 {
			return errors.New("embedded same-tag checksum drift")
		}
		return nil
	}
	if err := atomicCopyFile(u.embeddedPath, filepath.Join(u.root, "bin", "embedded"), 0755, maxBinaryBytes); err != nil {
		return err
	}
	record := versionRecord{Tag: embeddedVersion, SHA256: sourceSum, Installed: u.now()}
	u.ledger.Embedded = record
	currentCmp, err := compareVersion(embeddedVersion, u.ledger.Current.Tag)
	if err != nil {
		return err
	}
	if currentCmp > 0 {
		if err := atomicCopyFile(filepath.Join(u.root, "bin", "current"), filepath.Join(u.root, "bin", "prior"), 0755, maxBinaryBytes); err != nil {
			return err
		}
		if err := atomicCopyFile(u.embeddedPath, filepath.Join(u.root, "bin", "current"), 0755, maxBinaryBytes); err != nil {
			return err
		}
		prior := u.ledger.Current
		u.ledger.Prior = &prior
		u.ledger.Current = record
		u.ledger.CrashJournal = "advanced current to newer build-qualified embedded fallback"
	}
	return u.saveLedger()
}

func (u *updater) verifyCurrentBinary() error {
	sum, err := fileSHA(filepath.Join(u.root, "bin", "current"))
	if err != nil {
		return err
	}
	if sum != u.ledger.Current.SHA256 {
		return errors.New("current binary checksum mismatch")
	}
	return nil
}

func (u *updater) saveLedger() error {
	if u.saveLedgerHook != nil {
		if err := u.saveLedgerHook(u.ledger); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(u.ledger, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(u.root, "ledger.json"), append(data, '\n'), 0600)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmpName := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	_ = os.Remove(tmpName)
	tmp, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	df, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer df.Close()
	if err := df.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func atomicCopyFile(src, dst string, mode os.FileMode, limit int64) error {
	in, err := os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return errors.New("copy source outside bounds")
	}
	dir := filepath.Dir(dst)
	tmpName := filepath.Join(dir, "."+filepath.Base(dst)+".tmp")
	_ = os.Remove(tmpName)
	tmp, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	written, err := io.CopyN(tmp, in, info.Size())
	if err != nil || written != info.Size() {
		return errors.New("copy failed")
	}
	var extra [1]byte
	if n, err := in.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return errors.New("copy source changed")
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	df, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer df.Close()
	if err := df.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (u *updater) recoverInterrupted() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	switch u.ledger.Phase {
	case "", "idle":
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	case "metadata":
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	case "cutover", "probation":
		if u.ledger.Prior == nil {
			return errors.New("interrupted cutover without rollback binary")
		}
		if err := os.Rename(filepath.Join(u.root, "bin", "prior"), filepath.Join(u.root, "bin", "current")); err != nil {
			return err
		}
		u.ledger.Current = *u.ledger.Prior
		u.ledger.Prior, u.ledger.Staged = nil, nil
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
		_ = os.Remove(filepath.Join(u.root, "bin", "staged"))
	case "download", "staged", "probe":
		_ = os.Remove(filepath.Join(u.root, "bin", "staged"))
		u.ledger.Staged = nil
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	case "rollback":
		if u.ledger.Prior != nil {
			if err := os.Rename(filepath.Join(u.root, "bin", "prior"), filepath.Join(u.root, "bin", "current")); err != nil {
				return err
			}
			u.ledger.Current = *u.ledger.Prior
			u.ledger.Prior, u.ledger.Staged = nil, nil
		} else if err := u.verifyCurrentBinary(); err != nil {
			return err
		}
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	default:
		return errors.New("unknown crash-journal phase")
	}
	return u.saveLedger()
}

func (u *updater) startCurrent() error { return u.startChild(filepath.Join(u.root, "bin", "current")) }

func (u *updater) startChild(path string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.child != nil && u.child.ProcessState == nil {
		return errors.New("child already running")
	}
	cmd := exec.Command(path, "-config", u.configPath)
	cmd.Stdout, cmd.Stderr, cmd.Env = os.Stdout, os.Stderr, sanitizedChildEnvironment()
	if err := cmd.Start(); err != nil {
		return err
	}
	u.child, u.childGeneration = cmd, u.childGeneration+1
	generation := u.childGeneration
	done := make(chan error, 1)
	u.childDone = done
	go func() {
		err := cmd.Wait()
		done <- err
		u.childExit <- childResult{generation: generation, err: err}
	}()
	return nil
}

func sanitizedChildEnvironment() []string {
	blocked := map[string]bool{
		"CLIPROXY_PROXY_KEY":      true,
		"CLIPROXY_MANAGEMENT_KEY": true,
	}
	output := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		name := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			name = item[:index]
		}
		if !blocked[name] {
			output = append(output, item)
		}
	}
	return output
}

func (u *updater) stopChild(sig os.Signal) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.stopping = true
	if u.child != nil && u.child.ProcessState == nil {
		_ = u.child.Process.Signal(sig)
	}
}

func (u *updater) stopForCutover() error {
	u.mu.Lock()
	child, done := u.child, u.childDone
	if child == nil || child.ProcessState != nil {
		u.child, u.childDone = nil, nil
		u.mu.Unlock()
		return nil
	}
	u.plannedStop = u.childGeneration
	if err := child.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		u.mu.Unlock()
		return err
	}
	u.mu.Unlock()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return errors.New("child could not be reaped")
		}
	}
	u.mu.Lock()
	if u.child == child {
		u.child, u.childDone = nil, nil
	}
	u.mu.Unlock()
	return nil
}

func (u *updater) ignoreStaleExit(result childResult) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if result.generation == u.plannedStop {
		u.plannedStop = 0
		return true
	}
	return u.stopping || result.generation != u.childGeneration
}

func (u *updater) childHealthy() bool {
	u.mu.Lock()
	child, stopping := u.child, u.stopping
	u.mu.Unlock()
	if stopping || child == nil || child.ProcessState != nil {
		return false
	}
	c, err := net.DialTimeout("tcp", u.upstream, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func (u *updater) rollbackAfterCrash(cause error) error {
	u.mu.Lock()
	if u.ledger.Prior == nil || u.ledger.Phase == "rollback" {
		u.mu.Unlock()
		return errors.New("no prior binary")
	}
	failed := u.ledger.Current.Tag
	checksum := u.ledger.ChecksumsByTag[failed]
	u.mu.Unlock()
	failure := deterministic("candidate crashed after live cutover", checksum, cause)
	rollbackErr := u.rollbackCutover(failure)
	if !u.childHealthy() {
		return rollbackErr
	}
	u.handleCandidateFailure(failed, failure)
	return nil
}

func (u *updater) allowedURL(v *url.URL) bool {
	if u.allowLocal {
		if v.User != nil || v.Scheme != u.fixtureScheme || effectiveURLPort(v) != u.fixturePort {
			return false
		}
		if v.Scheme == "http" {
			return v.Hostname() == "127.0.0.1" || v.Hostname() == "localhost" ||
				(u.fixtureHost != "" && v.Hostname() == u.fixtureHost)
		}
		return v.Scheme == "https" && u.fixtureHost != "" && v.Hostname() == u.fixtureHost
	}
	if v.Scheme != "https" || v.User != nil {
		return false
	}
	switch strings.ToLower(v.Hostname()) {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func effectiveURLPort(v *url.URL) string {
	if port := v.Port(); port != "" {
		return port
	}
	switch v.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (u *updater) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 5 || !u.allowedURL(req.URL) {
		return errors.New("redirect outside release allowlist")
	}
	return nil
}

func (u *updater) updateLoop(ctx context.Context) {
	for {
		timer := time.NewTimer(u.nextWait())
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := u.checkAndApply(ctx); err != nil {
			log.Printf("runtime update attempt failed safely")
		}
	}
}

func (u *updater) nextWait() time.Duration {
	u.mu.Lock()
	defer u.mu.Unlock()
	now, next := u.now(), u.ledger.NextCheck
	if u.ledger.LastAttempt.IsZero() || next.IsZero() || now.Before(u.ledger.LastAttempt) ||
		now.Sub(u.ledger.LastAttempt) >= maxAttemptGap || !next.After(now) ||
		next.After(now.Add(maxAttemptGap)) {
		return 0
	}
	wait := next.Sub(now)
	if !u.nextMono.IsZero() && u.monoNow != nil {
		monoNow := u.monoNow()
		monoWait := u.nextMono.Sub(monoNow)
		if monoWait <= 0 {
			return 0
		}
		if !u.attemptMono.IsZero() {
			remaining := maxAttemptGap - monoNow.Sub(u.attemptMono)
			if remaining <= 0 {
				return 0
			}
			if remaining < monoWait {
				monoWait = remaining
			}
		}
		if monoWait < wait {
			wait = monoWait
		}
	}
	return wait
}

func deterministicJitter(id string, at time.Time) time.Duration {
	sum := sha256.Sum256([]byte(id + ":" + at.UTC().Format("2006-01-02T15")))
	n := uint64(sum[0])<<8 | uint64(sum[1])
	return time.Duration(n%uint64(maxJitter/time.Second+1)) * time.Second
}

func soakDelay(published, now time.Time) (time.Duration, error) {
	if published.After(now.Add(5 * time.Minute)) {
		return 0, errors.New("release timestamp in future")
	}
	ready := published.Add(releaseSoak)
	if now.Before(ready) {
		return ready.Sub(now), nil
	}
	return 0, nil
}

func (u *updater) scheduleAfterAttempt(success bool, retry time.Duration) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
	if u.ledger.LastAttempt.IsZero() {
		u.ledger.LastAttempt = now
	}
	delay := retry
	if success {
		u.ledger.LastSuccess, u.ledger.ConsecutiveFail = now, 0
		u.ledger.LastFailureClass, u.ledger.LastFailureReason = "", ""
		delay = defaultInterval + deterministicJitter(u.ledger.InstallationID, now)
	} else {
		u.ledger.ConsecutiveFail++
		if delay <= 0 || delay > maxAttemptGap {
			delay = transientRetry
		}
	}
	if u.ledger.Phase != "cutover" && u.ledger.Phase != "probation" && u.ledger.Phase != "rollback" {
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	}
	u.ledger.NextCheck = now.Add(delay)
	if u.ledger.NextCheck.After(now.Add(maxAttemptGap)) {
		u.ledger.NextCheck = now.Add(maxAttemptGap)
	}
	if u.monoNow != nil {
		u.nextMono = u.monoNow().Add(u.ledger.NextCheck.Sub(now))
	}
	return u.saveLedger()
}

func (u *updater) recordAttemptStart() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
	u.ledger.LastAttempt = now
	u.ledger.NextCheck = now.Add(transientRetry)
	if u.monoNow != nil {
		u.attemptMono = u.monoNow()
		u.nextMono = u.attemptMono.Add(transientRetry)
	}
	u.ledger.Phase, u.ledger.CrashJournal = "metadata", "release metadata check in progress"
	return u.saveLedger()
}

func (u *updater) updaterLock() (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(u.root, "update.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func (u *updater) checkAndApply(ctx context.Context) error {
	lock, err := u.updaterLock()
	if err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}()
	if err := u.recordAttemptStart(); err != nil {
		return err
	}
	release, etag, retry, status, err := u.fetchRelease(ctx)
	if err != nil {
		u.recordFailure(classified(failureTransient, "release acquisition failed", err))
		_ = u.scheduleAfterAttempt(false, retry)
		return err
	}
	if status == http.StatusNotModified {
		return u.scheduleAfterAttempt(true, 0)
	}
	u.mu.Lock()
	u.ledger.ETag = etag
	current := u.ledger.Current.Tag
	u.mu.Unlock()
	cmp, err := compareVersion(release.TagName, current)
	if err != nil {
		failure := classified(failureDeterministic, "release version is invalid", err)
		u.recordFailure(failure)
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return failure
	}
	if cmp < 0 {
		return u.scheduleAfterAttempt(true, 0)
	}
	archiveAsset, checksum, err := u.releaseIdentity(ctx, release)
	if err != nil {
		u.handleCandidateFailure(release.TagName, err)
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return err
	}
	u.mu.Lock()
	knownIdentity := u.ledger.ChecksumsByTag[release.TagName]
	quarantined := u.ledger.Quarantine[quarantineKey(release.TagName, checksum)] != ""
	u.mu.Unlock()
	if knownIdentity != "" && knownIdentity != checksum {
		failure := &updateFailure{
			class: failureSecurity, reason: "same-tag release checksum identity drift",
			checksum: checksum, err: errors.New("same-tag checksum drift"),
		}
		u.handleCandidateFailure(release.TagName, failure)
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return failure
	}
	if cmp == 0 {
		u.mu.Lock()
		u.ledger.ChecksumsByTag[release.TagName] = checksum
		u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
		saveErr := u.saveLedger()
		u.mu.Unlock()
		if saveErr != nil {
			return saveErr
		}
		return u.scheduleAfterAttempt(true, 0)
	}
	if quarantined {
		u.recordFailure(&updateFailure{class: failureDeterministic, reason: "candidate identity remains quarantined", checksum: checksum})
		return u.scheduleAfterAttempt(false, transientRetry)
	}
	now := u.now()
	delay, err := soakDelay(release.PublishedAt, now)
	if err != nil {
		u.recordFailure(classified(failureTransient, "release timestamp could not be trusted yet", err))
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return err
	}
	if delay > 0 {
		return u.scheduleAfterAttempt(false, delay)
	}
	if err := u.stageReleaseWithIdentity(ctx, release, archiveAsset, checksum); err != nil {
		u.handleCandidateFailure(release.TagName, err)
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return err
	}
	if err := u.probeCandidate(ctx, release.TagName); err != nil {
		failure := deterministic("candidate private probe failed", checksum, err)
		u.handleCandidateFailure(release.TagName, failure)
		_ = os.Remove(filepath.Join(u.root, "bin", "staged"))
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return failure
	}
	if err := u.cutover(ctx); err != nil {
		u.handleCandidateFailure(release.TagName, err)
		_ = u.scheduleAfterAttempt(false, transientRetry)
		return err
	}
	return u.scheduleAfterAttempt(true, 0)
}

func quarantineKey(tag, checksum string) string { return tag + "@" + checksum }

func (u *updater) recordFailure(err error) {
	var failure *updateFailure
	if !errors.As(err, &failure) {
		failure = &updateFailure{class: failureTransient, reason: "unclassified retryable failure", err: err}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ledger.LastFailureClass, u.ledger.LastFailureReason = string(failure.class), failure.reason
	_ = u.saveLedger()
}

func (u *updater) handleCandidateFailure(tag string, err error) {
	var failure *updateFailure
	if !errors.As(err, &failure) {
		failure = &updateFailure{class: failureTransient, reason: "candidate operation will be retried", err: err}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ledger.LastFailureClass, u.ledger.LastFailureReason = string(failure.class), failure.reason
	if (failure.class == failureDeterministic || failure.class == failureSecurity) &&
		len(failure.checksum) == 64 {
		u.ledger.Quarantine[quarantineKey(tag, failure.checksum)] = failure.reason
	}
	if u.ledger.Phase != "probation" && u.ledger.Phase != "cutover" && u.ledger.Phase != "rollback" {
		u.ledger.Phase, u.ledger.Staged = "idle", nil
	}
	_ = u.saveLedger()
}

func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}

func (u *updater) fetchRelease(ctx context.Context) (githubRelease, string, time.Duration, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.apiURL, nil)
	if err != nil {
		return githubRelease{}, "", 0, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", updaterUserAgent)
	resp, err := u.client.Do(req)
	if err != nil {
		return githubRelease{}, "", 0, 0, err
	}
	defer resp.Body.Close()
	if !u.allowedURL(resp.Request.URL) {
		return githubRelease{}, "", 0, resp.StatusCode, errors.New("final metadata host rejected")
	}
	if resp.StatusCode == http.StatusNotModified {
		return githubRelease{}, resp.Header.Get("ETag"), 0, resp.StatusCode, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return githubRelease{}, "", retryAfter(resp), resp.StatusCode, errors.New("release API rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, "", 0, resp.StatusCode, errors.New("release API status rejected")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes+1))
	if err != nil || len(body) > maxMetadataBytes {
		return githubRelease{}, "", 0, resp.StatusCode, errors.New("release metadata outside bounds")
	}
	var releases []githubRelease
	if json.Unmarshal(body, &releases) != nil || len(releases) > 100 {
		return githubRelease{}, "", 0, resp.StatusCode, errors.New("invalid release metadata")
	}
	var selected githubRelease
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		if _, err := parseSemver(release.TagName); err != nil {
			continue
		}
		if selected.TagName == "" {
			selected = release
		} else if cmp, _ := compareVersion(release.TagName, selected.TagName); cmp > 0 {
			selected = release
		}
	}
	if selected.TagName == "" || len(selected.Assets) > 32 {
		return githubRelease{}, "", 0, resp.StatusCode, errors.New("no acceptable stable release")
	}
	return selected, resp.Header.Get("ETag"), 0, resp.StatusCode, nil
}

func archAsset(tag string) string {
	version, arch := strings.TrimPrefix(tag, "v"), runtime.GOARCH
	if arch == "arm64" {
		arch = "aarch64"
	}
	return fmt.Sprintf("CLIProxyAPI_%s_linux_%s.tar.gz", version, arch)
}

func findAssets(release githubRelease) (releaseAsset, releaseAsset, error) {
	want := archAsset(release.TagName)
	var archive, sums releaseAsset
	ac, sc := 0, 0
	for _, asset := range release.Assets {
		switch asset.Name {
		case want:
			archive, ac = asset, ac+1
		case "checksums.txt":
			sums, sc = asset, sc+1
		}
	}
	if ac != 1 || sc != 1 || archive.Size <= 0 || archive.Size > maxArchiveBytes ||
		sums.Size <= 0 || sums.Size > maxChecksumsBytes {
		return releaseAsset{}, releaseAsset{}, errors.New("release asset contract mismatch")
	}
	return archive, sums, nil
}

func (u *updater) download(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || !u.allowedURL(parsed) {
		return nil, classified(failureTransient, "release asset URL temporarily rejected", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	req.Header.Set("User-Agent", updaterUserAgent)
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, classified(failureTransient, "release asset transfer failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !u.allowedURL(resp.Request.URL) {
		return nil, classified(failureTransient, "release asset response rejected", nil)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, classified(failureTransient, "release asset transfer exceeded bound", err)
	}
	return data, nil
}

func checksumFor(data []byte, name string) (string, error) {
	if len(data) == 0 || len(data) > maxChecksumsBytes || bytes.Contains(data, []byte("\r")) {
		return "", errors.New("invalid checksums file")
	}
	found := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return "", errors.New("invalid checksum line")
		}
		sum, asset := strings.ToLower(fields[0]), strings.TrimPrefix(fields[1], "*")
		if len(sum) != 64 {
			return "", errors.New("invalid checksum")
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return "", errors.New("invalid checksum")
		}
		if asset == name {
			if found != "" {
				return "", errors.New("duplicate checksum")
			}
			found = sum
		}
	}
	if found == "" {
		return "", errors.New("archive checksum missing")
	}
	return found, nil
}

func extractBinary(archive []byte) ([]byte, error) {
	if len(archive) == 0 || len(archive) > maxArchiveBytes {
		return nil, errors.New("archive outside bounds")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, errors.New("invalid gzip")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var binary []byte
	seen, entries := map[string]bool{}, 0
	allowed := map[string]bool{"cli-proxy-api": true, "LICENSE": true, "README.md": true, "README_CN.md": true, "config.example.yaml": true}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("invalid tar stream")
		}
		entries++
		name := header.Name
		if entries > maxArchiveEntries || filepath.Clean(name) != name || filepath.IsAbs(name) ||
			strings.Contains(name, "/") || seen[name] || !allowed[name] {
			return nil, errors.New("archive entry contract rejected")
		}
		seen[name] = true
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, errors.New("links and non-regular entries rejected")
		}
		if header.Size < 0 || header.Size > maxBinaryBytes {
			return nil, errors.New("archive entry too large")
		}
		if name == expectedBinaryName {
			if header.Size <= 0 || header.Mode&0111 == 0 || binary != nil {
				return nil, errors.New("binary entry rejected")
			}
			binary, err = io.ReadAll(io.LimitReader(tr, maxBinaryBytes+1))
			if err != nil || int64(len(binary)) != header.Size {
				return nil, errors.New("binary extraction failed")
			}
		}
	}
	if binary == nil {
		return nil, errors.New("exact binary missing")
	}
	return binary, nil
}

func (u *updater) releaseIdentity(ctx context.Context, release githubRelease) (releaseAsset, string, error) {
	archiveAsset, sumsAsset, err := findAssets(release)
	if err != nil {
		return releaseAsset{}, "", classified(failureDeterministic, "release asset contract mismatch", err)
	}
	sums, err := u.download(ctx, sumsAsset.BrowserDownloadURL, maxChecksumsBytes)
	if err != nil {
		return releaseAsset{}, "", err
	}
	expected, err := checksumFor(sums, archiveAsset.Name)
	if err != nil {
		return releaseAsset{}, "", classified(failureDeterministic, "release checksum manifest invalid", err)
	}
	return archiveAsset, expected, nil
}

func (u *updater) stageRelease(ctx context.Context, release githubRelease) error {
	archiveAsset, expected, err := u.releaseIdentity(ctx, release)
	if err != nil {
		return err
	}
	return u.stageReleaseWithIdentity(ctx, release, archiveAsset, expected)
}

func (u *updater) stageReleaseWithIdentity(ctx context.Context, release githubRelease, archiveAsset releaseAsset, expected string) error {
	if err := u.requireFreeSpace(archiveAsset.Size + maxBinaryBytes); err != nil {
		return err
	}
	u.mu.Lock()
	u.ledger.Phase, u.ledger.CrashJournal = "download", "candidate download in progress"
	if err := u.saveLedger(); err != nil {
		u.mu.Unlock()
		return err
	}
	u.mu.Unlock()
	u.mu.Lock()
	if old := u.ledger.ChecksumsByTag[release.TagName]; old != "" && old != expected {
		u.mu.Unlock()
		return &updateFailure{
			class: failureSecurity, reason: "same-tag release checksum identity drift",
			checksum: expected, err: errors.New("same-tag checksum drift"),
		}
	}
	u.mu.Unlock()
	archive, err := u.download(ctx, archiveAsset.BrowserDownloadURL, maxArchiveBytes)
	if err != nil {
		return err
	}
	actual := sha256.Sum256(archive)
	if hex.EncodeToString(actual[:]) != expected {
		return deterministic("release archive checksum mismatch", expected, errors.New("archive checksum mismatch"))
	}
	binary, err := extractBinary(archive)
	if err != nil {
		return deterministic("release archive contract mismatch", expected, err)
	}
	binarySum := sha256.Sum256(binary)
	record := versionRecord{Tag: release.TagName, SHA256: hex.EncodeToString(binarySum[:]), Installed: u.now()}
	if err := atomicWrite(filepath.Join(u.root, "bin", "staged"), binary, 0755); err != nil {
		return classified(failureStorage, "candidate staging write failed", err)
	}
	u.mu.Lock()
	u.ledger.ChecksumsByTag[release.TagName], u.ledger.Staged = expected, &record
	u.ledger.Phase, u.ledger.CrashJournal = "staged", "verified candidate staged"
	err = u.saveLedger()
	u.mu.Unlock()
	if err != nil {
		return classified(failureStorage, "candidate staging journal failed", err)
	}
	return nil
}

func probeConfig(port int) []byte {
	return []byte(fmt.Sprintf(`host: "127.0.0.1"
port: %d
tls:
  enable: false
remote-management:
  allow-remote: true
  secret-key: "probe-management-key-000000000000000000000"
  disable-control-panel: false
  disable-auto-update-panel: true
auth-dir: "/data/update/probe/auth"
api-keys:
  - "probe-client-key-000000000000000000000000"
debug: false
logging-to-file: false
usage-statistics-enabled: false
ws-auth: true
`, port))
}

func (u *updater) probeCandidate(ctx context.Context, tag string) error {
	u.mu.Lock()
	u.ledger.Phase, u.ledger.CrashJournal = "probe", "isolated candidate probe"
	if err := u.saveLedger(); err != nil {
		u.mu.Unlock()
		return err
	}
	u.mu.Unlock()
	probeRoot := filepath.Join(u.root, "probe")
	for _, dir := range []string{filepath.Join(probeRoot, "auth"), filepath.Join(probeRoot, "home")} {
		if err := secureMkdir(dir); err != nil {
			return err
		}
	}
	cfg := filepath.Join(probeRoot, "config.yaml")
	if err := atomicWrite(cfg, probeConfig(probePort), 0600); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(cfg)
		_ = os.RemoveAll(filepath.Join(probeRoot, "auth"))
		_ = os.RemoveAll(filepath.Join(probeRoot, "home"))
	}()
	binary := filepath.Join(u.root, "bin", "staged")
	versionCtx, cancelVersion := context.WithTimeout(ctx, 5*time.Second)
	versionCmd := exec.CommandContext(versionCtx, binary, "--version")
	versionCmd.Env = sanitizedChildEnvironment()
	versionOutput, _ := versionCmd.CombinedOutput()
	cancelVersion()
	if !strings.Contains(string(versionOutput), "CLIProxyAPI Version: "+tag+",") {
		return errors.New("candidate version mismatch")
	}
	probeCtx, cancelProbe := context.WithCancel(ctx)
	defer cancelProbe()
	cmd := exec.CommandContext(probeCtx, binary, "-config", cfg)
	cmd.Env = append(sanitizedChildEnvironment(), "HOME="+filepath.Join(probeRoot, "home"))
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	}()
	base := fmt.Sprintf("http://127.0.0.1:%d", probePort)
	if err := waitHTTP(ctx, base+"/v1/models", "probe-client-key-000000000000000000000000", probeTimeout); err != nil {
		return err
	}
	checks := []struct {
		path, token string
		want        int
	}{
		{"/v1/models", "", 401},
		{"/v1/models", "probe-management-key-000000000000000000000", 401},
		{"/v1/models", "probe-client-key-000000000000000000000000", 200},
		{"/v0/management/config", "", 401},
		{"/v0/management/config", "probe-client-key-000000000000000000000000", 401},
		{"/v0/management/config", "probe-management-key-000000000000000000000", 200},
		{"/management.html", "", 200},
	}
	for _, check := range checks {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+check.path, nil)
		if check.token != "" {
			req.Header.Set("Authorization", "Bearer "+check.token)
		}
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
		if err != nil || resp.StatusCode != check.want {
			if resp != nil {
				_ = resp.Body.Close()
			}
			return errors.New("candidate auth probe failed")
		}
		if check.path == "/management.html" {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			if readErr != nil || fmt.Sprintf("%x", sha256.Sum256(body)) != managementUIHash {
				_ = resp.Body.Close()
				return errors.New("candidate management UI integrity probe failed")
			}
		}
		_ = resp.Body.Close()
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-done:
		if err != nil {
			return errors.New("candidate did not exit cleanly")
		}
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("candidate did not exit cleanly")
	}
}

func waitHTTP(ctx context.Context, endpoint, token string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := (&http.Client{Timeout: time.Second}).Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("candidate readiness timeout")
}

func waitTCP(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("readiness timeout")
}

func semanticRequest(ctx context.Context, client *http.Client, endpoint, token string, want int, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(body)) > limit || resp.StatusCode != want {
		return nil, errors.New("semantic response contract failed")
	}
	return body, nil
}

func validateSemanticState(ctx context.Context, base, proxyKey, managementKey string) error {
	return validateSemanticStateWithUIHash(ctx, base, proxyKey, managementKey, managementUIHash)
}

func validateSemanticStateWithUIHash(ctx context.Context, base, proxyKey, managementKey, expectedUIHash string) error {
	if len(proxyKey) < 32 || len(managementKey) < 32 || proxyKey == managementKey {
		return errors.New("live credentials unavailable")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	checks := []struct {
		path, token string
		want        int
	}{
		{"/v1/models", "", http.StatusUnauthorized},
		{"/v1/models", managementKey, http.StatusUnauthorized},
		{"/v1/models", proxyKey, http.StatusOK},
		{"/v0/management/config", "", http.StatusUnauthorized},
		{"/v0/management/config", proxyKey, http.StatusUnauthorized},
	}
	for _, check := range checks {
		if _, err := semanticRequest(ctx, client, base+check.path, check.token, check.want, 2<<20); err != nil {
			return err
		}
	}
	configBody, err := semanticRequest(ctx, client, base+"/v0/management/config", managementKey, http.StatusOK, 2<<20)
	if err != nil || len(configBody) == 0 || !json.Valid(configBody) {
		return errors.New("live management config unreadable")
	}
	ui, err := semanticRequest(ctx, client, base+"/management.html", "", http.StatusOK, 8<<20)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(ui)) != expectedUIHash {
		return errors.New("live management UI integrity failed")
	}
	return nil
}

func fsyncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (u *updater) writeRollbackSnapshot(current versionRecord) error {
	snapshot := u.ledger
	snapshot.Current, snapshot.Prior, snapshot.Staged = current, nil, nil
	snapshot.Phase = "idle"
	snapshot.CrashJournal = ""
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(u.root, "rollback-ledger.json"), append(data, '\n'), 0600)
}

func (u *updater) cutover(ctx context.Context) error {
	u.mu.Lock()
	if u.ledger.Staged == nil {
		u.mu.Unlock()
		return errors.New("no staged candidate")
	}
	candidate, current := *u.ledger.Staged, u.ledger.Current
	currentInfo, err := os.Stat(filepath.Join(u.root, "bin", "current"))
	if err != nil {
		u.mu.Unlock()
		return classified(failureStorage, "current binary stat failed", err)
	}
	if err := u.requireFreeSpace(currentInfo.Size() + minDiskHeadroom); err != nil {
		u.mu.Unlock()
		return err
	}
	if err := atomicCopyFile(filepath.Join(u.root, "bin", "current"), filepath.Join(u.root, "bin", "prior"), 0755, maxBinaryBytes); err != nil {
		u.mu.Unlock()
		return classified(failureStorage, "rollback binary preparation failed", err)
	}
	u.ledger.Prior = &current
	u.ledger.Phase, u.ledger.CrashJournal = "cutover", "candidate cutover prepared"
	if err := u.saveLedger(); err != nil {
		u.mu.Unlock()
		return classified(failureStorage, "cutover journal preparation failed", err)
	}
	if err := u.writeRollbackSnapshot(current); err != nil {
		u.mu.Unlock()
		return classified(failureStorage, "rollback ledger preparation failed", err)
	}
	u.mu.Unlock()
	if err := u.stopForCutover(); err != nil {
		return u.rollbackCutover(classified(failureTransient, "current child stop failed", err))
	}
	if err := atomicCopyFile(filepath.Join(u.root, "bin", "staged"), filepath.Join(u.root, "bin", "current"), 0755, maxBinaryBytes); err != nil {
		return u.rollbackCutover(classified(failureStorage, "candidate activation failed", err))
	}
	u.mu.Lock()
	u.stopping = false
	u.ledger.Prior, u.ledger.Current = &current, candidate
	u.ledger.Phase, u.ledger.CrashJournal = "probation", "candidate running in bounded probation"
	if err := u.saveLedger(); err != nil {
		u.mu.Unlock()
		return u.rollbackCutover(classified(failureStorage, "candidate probation journal failed", err))
	}
	u.mu.Unlock()
	if err := u.startCurrent(); err != nil {
		return u.rollbackCutover(deterministic("candidate live start failed", u.ledger.ChecksumsByTag[candidate.Tag], err))
	}
	if err := waitTCP(u.upstream, probeTimeout); err != nil {
		return u.rollbackCutover(deterministic("candidate live readiness failed", u.ledger.ChecksumsByTag[candidate.Tag], err))
	}
	base := "http://" + u.upstream
	if err := u.semanticCheck(ctx, base, u.proxyKey, u.managementKey); err != nil {
		return u.rollbackCutover(deterministic("candidate live semantic validation failed", u.ledger.ChecksumsByTag[candidate.Tag], err))
	}
	timer := time.NewTimer(u.probation)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return u.rollbackCutover(classified(failureTransient, "candidate probation interrupted", ctx.Err()))
	case <-timer.C:
	}
	if !u.childHealthy() {
		return u.rollbackCutover(deterministic("candidate failed live probation", u.ledger.ChecksumsByTag[candidate.Tag], nil))
	}
	if err := u.semanticCheck(ctx, base, u.proxyKey, u.managementKey); err != nil {
		return u.rollbackCutover(deterministic("candidate post-probation semantic validation failed", u.ledger.ChecksumsByTag[candidate.Tag], err))
	}
	u.mu.Lock()
	u.ledger.Staged, u.ledger.Phase, u.ledger.CrashJournal = nil, "idle", ""
	err = u.saveLedger()
	u.mu.Unlock()
	if err != nil {
		return u.rollbackCutover(classified(failureStorage, "candidate acceptance journal failed", err))
	}
	_ = os.Remove(filepath.Join(u.root, "bin", "staged"))
	_ = os.Remove(filepath.Join(u.root, "rollback-ledger.json"))
	return nil
}

func (u *updater) rollbackCutover(cause error) error {
	stopErr := u.stopForCutover()
	u.mu.Lock()
	if u.ledger.Prior == nil {
		u.mu.Unlock()
		if stopErr != nil {
			return errors.Join(cause, stopErr)
		}
		return cause
	}
	prior := *u.ledger.Prior
	u.ledger.Phase, u.ledger.CrashJournal = "rollback", "candidate failed; binary-only rollback"
	u.mu.Unlock()
	if err := os.Rename(filepath.Join(u.root, "bin", "prior"), filepath.Join(u.root, "bin", "current")); err != nil {
		return err
	}
	if err := fsyncDir(filepath.Join(u.root, "bin")); err != nil {
		return err
	}
	u.mu.Lock()
	u.stopping = false
	u.ledger.Current, u.ledger.Prior, u.ledger.Staged = prior, nil, nil
	u.ledger.Phase, u.ledger.CrashJournal = "idle", ""
	err := u.saveLedger()
	u.mu.Unlock()
	if err != nil {
		snapshot := filepath.Join(u.root, "rollback-ledger.json")
		if renameErr := os.Rename(snapshot, filepath.Join(u.root, "ledger.json")); renameErr != nil {
			err = errors.Join(err, renameErr)
		} else {
			err = fsyncDir(u.root)
		}
	}
	_ = os.Remove(filepath.Join(u.root, "bin", "staged"))
	startErr := u.startCurrent()
	if startErr == nil {
		startErr = waitTCP(u.upstream, probeTimeout)
	}
	if startErr != nil {
		return errors.Join(cause, stopErr, err, startErr)
	}
	return errors.Join(cause, stopErr, err)
}
