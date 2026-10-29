// DigiPanel Native Runtime & Application Manager (modules/runtimes/runtimes.go)
// Supervises user applications as real OS processes: detection of installed
// runtimes, process spawn with separate argument arrays (never shell strings),
// group termination, /proc metric collection, and registry persistence.
// Zero emojis, zero mocks: every listed process is a live OS process.

package runtimes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrInvalidName            = errors.New("application name invalid: 3-48 characters, letters digits dashes")
	ErrInvalidRuntime         = errors.New("runtime kind invalid: must be nodejs, python, or golang")
	ErrInvalidPath            = errors.New("application path invalid: absolute directory path required")
	ErrInvalidEntry           = errors.New("entry file invalid: bare filename without traversal")
	ErrEntryFileMissing       = errors.New("entry file does not exist in application path")
	ErrInvalidPort            = errors.New("port invalid: must be between 1024 and 65535")
	ErrDuplicateName          = errors.New("application with the same name already exists")
	ErrDuplicatePort          = errors.New("listen port already assigned to another application")
	ErrAppNotFound            = errors.New("application not found in runtime registry")
	ErrAlreadyRunning         = errors.New("application is already running")
	ErrNotRunning             = errors.New("application is not running")
	ErrRuntimeUnavailable     = errors.New("runtime binary not detected on this host")
	ErrStartupFailed          = errors.New("application process exited immediately after start")
)

type RuntimeKind string

const (
	RuntimeNodeJS RuntimeKind = "nodejs"
	RuntimePython RuntimeKind = "python"
	RuntimeGolang RuntimeKind = "golang"
)

// ValidKinds enumerates the runtime kinds this module supervises.
func ValidKinds() []RuntimeKind {
	return []RuntimeKind{RuntimeNodeJS, RuntimePython, RuntimeGolang}
}

func IsValidKind(k RuntimeKind) bool {
	switch k {
	case RuntimeNodeJS, RuntimePython, RuntimeGolang:
		return true
	}
	return false
}

var safeNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{2,48}$`)

// App is a registry entry. Status, PID and metrics are derived at read time
// from the live process, so this struct never stores stale fabricated values.
type App struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Runtime      RuntimeKind `json:"runtime"`
	Version      string      `json:"version"`
	Path         string      `json:"path"`
	EntryFile    string      `json:"entryFile"`
	Port         int         `json:"port"`
	Unit         string      `json:"unit"` // "web", "worker", "cron"
	Environment  string      `json:"environment"` // production, staging, development
	CreatedAt    time.Time   `json:"createdAt"`
}

type CreateAppRequest struct {
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
	Path        string `json:"path"`
	EntryFile   string `json:"entryFile"`
	Port        int    `json:"port"`
	Unit        string `json:"unit,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// Snapshot is the read model served to the API: derived live from /proc.
type Snapshot struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Runtime     RuntimeKind `json:"runtime"`
	Version     string      `json:"version"`
	Path        string      `json:"path"`
	EntryFile   string      `json:"entryFile"`
	Port        int         `json:"port"`
	Unit        string      `json:"unit"`
	Environment string      `json:"environment"`
	CreatedAt   time.Time   `json:"createdAt"`
	Status      string      `json:"status"` // running, stopped, error
	Failure     string      `json:"failure,omitempty"` // last failure message
	PID         int         `json:"pid"`
	MemoryMb    float64     `json:"memoryMb"`
	CPUPercent  float64     `json:"cpuPercent"`
	Uptime      string      `json:"uptime"`
}

type Manager struct {
	mu       sync.Mutex
	baseDir  string
	logDir   string
	buildDir string
	apps     map[string]*App
	pidFile  map[string]*processRecord // app.ID -> live process bookkeeping
	binaries map[RuntimeKind]string
	versions map[RuntimeKind]string
	nextID   int
}

type processRecord struct {
	PID       int       `json:"pid"`
	StartTicks uint64   `json:"startTicks"` // start_time from /proc, guards PID reuse
	StartedAt time.Time `json:"startedAt"`
}

const clockTicksPerSecond = 100 // HZ on every standard Linux build

func NewManager(baseDir string) *Manager {
	logDir := filepath.Join(baseDir, "logs")
	buildDir := filepath.Join(baseDir, "builds")
	_ = os.MkdirAll(logDir, 0755)
	_ = os.MkdirAll(buildDir, 0755)

	m := &Manager{
		baseDir:  baseDir,
		logDir:   logDir,
		buildDir: buildDir,
		apps:     make(map[string]*App),
		pidFile:  make(map[string]*processRecord),
		binaries: make(map[RuntimeKind]string),
		versions: make(map[RuntimeKind]string),
	}
	m.loadState()
	m.DetectRuntimes()
	return m
}

// DetectRuntimes locates interpreter binaries on PATH and reads their real
// version strings. The host decides which runtimes exist; nothing is invented.
func (m *Manager) DetectRuntimes() {
	candidates := map[RuntimeKind][]string{
		RuntimeNodeJS: {"node", "nodejs"},
		RuntimePython: {"python3", "python"},
		RuntimeGolang: {"go"},
	}
	for kind, names := range candidates {
		var binPath string
		for _, n := range names {
			if p, err := exec.LookPath(n); err == nil {
				binPath = p
				break
			}
		}
		if binPath == "" {
			continue
		}
		args := []string{"--version"}
		if kind == RuntimeGolang {
			args = []string{"version"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, err := exec.CommandContext(ctx, binPath, args...).CombinedOutput()
		cancel()
		if err != nil {
			continue
		}
		line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		version := extractVersionToken(line)
		if version == "" {
			continue
		}
		m.binaries[kind] = binPath
		m.versions[kind] = version
	}
}

// extractVersionToken pulls the version number (or version-prefixed word)
// out of a binary version banner, for example "go version go1.27.0 linux/amd64".
func extractVersionToken(line string) string {
	for _, field := range strings.Fields(line) {
		clean := strings.TrimPrefix(field, "v")
		clean = strings.TrimPrefix(clean, "go")
		v := strings.Split(clean, "-")[0]
		if v == "" {
			continue
		}
		digitSeen := false
		dotSeen := false
		valid := true
		for _, ch := range v {
			if ch >= '0' && ch <= '9' {
				digitSeen = true
			} else if ch == '.' {
				dotSeen = true
			} else {
				valid = false
				break
			}
		}
		if valid && digitSeen && dotSeen {
			return clean
		}
	}
	return ""
}

// DetectedRuntimes returns the binaries and versions found on this host.
func (m *Manager) DetectedRuntimes() map[RuntimeKind]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[RuntimeKind]string)
	for kind, version := range m.versions {
		out[kind] = version
	}
	return out
}

// CreateApp registers a new supervised application. The runtime version is
// resolved from the detected interpreter; caller-supplied versions are ignored
// so the registry cannot drift from the host.
func (m *Manager) CreateApp(req CreateAppRequest) (*App, error) {
	kind := RuntimeKind(strings.ToLower(strings.TrimSpace(req.Runtime)))
	name := strings.TrimSpace(req.Name)
	apath := filepath.Clean(req.Path)
	entry := strings.TrimSpace(req.EntryFile)

	if !safeNameRegex.MatchString(name) {
		return nil, ErrInvalidName
	}
	if !IsValidKind(kind) {
		return nil, ErrInvalidRuntime
	}
	if !filepath.IsAbs(apath) || apath == "/" || strings.Contains(req.Path, "..") {
		return nil, ErrInvalidPath
	}
	if entry == "" || strings.ContainsAny(entry, `/\`) || entry == "." || entry == ".." || strings.Contains(entry, "..") {
		return nil, ErrInvalidEntry
	}
	if stat, err := os.Stat(apath); err != nil || !stat.IsDir() {
		return nil, ErrInvalidPath
	}
	if stat, err := os.Stat(filepath.Join(apath, entry)); err != nil || stat.IsDir() {
		return nil, ErrEntryFileMissing
	}
	if req.Port < 1024 || req.Port > 65535 {
		return nil, ErrInvalidPort
	}

	env := strings.ToLower(strings.TrimSpace(req.Environment))
	if env == "" {
		env = "production"
	}
	unit := strings.ToLower(strings.TrimSpace(req.Unit))
	if unit == "" {
		unit = "web"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	version, ok := m.versions[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, kind)
	}
	for _, a := range m.apps {
		if a.Name == name {
			return nil, ErrDuplicateName
		}
		if a.Port == req.Port {
			return nil, ErrDuplicatePort
		}
	}

	m.nextID++
	app := &App{
		ID:          fmt.Sprintf("app-%d", m.nextID),
		Name:        name,
		Runtime:     kind,
		Version:     version,
		Path:        apath,
		EntryFile:   entry,
		Port:        req.Port,
		Unit:        unit,
		Environment: env,
		CreatedAt:   time.Now(),
	}
	m.apps[app.ID] = app

	if err := m.persistLocked(); err != nil {
		delete(m.apps, app.ID)
		return nil, fmt.Errorf("failed persisting runtime registry: %w", err)
	}
	return app, nil
}

// Start spawns the application as a real OS process in its own process group,
// with stdout and stderr streamed into the per-app log file. For Go programs
// the sources are compiled first with the detected Go toolchain.
func (m *Manager) Start(id string) (*Snapshot, error) {
	m.mu.Lock()
	app, record, err := m.startLocked(id)
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	time.Sleep(1200 * time.Millisecond) // observe for immediate-exit failure

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.procAliveLocked(id) {
		lastErr := tailLastLine(m.logFilePath(app.ID), 480)
		delete(m.pidFile, app.ID)
		_ = m.persistLocked()
		msg := ErrStartupFailed.Error()
		if lastErr != "" {
			msg = msg + ": " + lastErr
		}
		return nil, errors.New(msg)
	}
	return m.buildSnapshotLocked(app, record)
}

func (m *Manager) startLocked(id string) (*App, *processRecord, error) {
	app, exists := m.apps[id]
	if !exists {
		return nil, nil, ErrAppNotFound
	}
	if rec, running := m.pidFile[id]; running {
		if procExists(rec.PID, rec.StartTicks) {
			return nil, nil, ErrAlreadyRunning
		}
		delete(m.pidFile, id)
	}
	bin := m.binaries[app.Runtime]
	if bin == "" {
		return nil, nil, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, app.Runtime)
	}

	executable := ""
	var argv []string
	if app.Runtime == RuntimeGolang {
		outBinary := filepath.Join(m.buildDir, app.ID)
		build := exec.Command(bin, "build", "-o", outBinary, app.EntryFile)
		build.Dir = app.Path
		build.Env = append(os.Environ(), "GO111MODULE=auto")
		if out, err := build.CombinedOutput(); err != nil {
			return nil, nil, fmt.Errorf("go build failed: %s", strings.TrimSpace(tailLastLineFrom(out, 300)))
		}
		executable = outBinary
		argv = []string{}
	} else {
		executable = bin
		argv = []string{app.EntryFile}
	}

	logFile, err := os.OpenFile(m.logFilePath(app.ID), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed opening app log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(executable, argv...)
	cmd.Dir = app.Path
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed spawning application process: %w", err)
	}
	startTicks := readProcStartTicks(cmd.Process.Pid)
	record := &processRecord{PID: cmd.Process.Pid, StartTicks: startTicks, StartedAt: time.Now()}
	m.pidFile[id] = record
	if err := m.persistLocked(); err != nil {
		_ = cmd.Process.Kill()
		delete(m.pidFile, id)
		return nil, nil, fmt.Errorf("failed persisting runtime registry: %w", err)
	}

	// Reap the supervisor exec handle: the process keeps running detached
	// in its own group; when it eventually exits it is reaped here instead
	// of accumulating as a zombie in the panel process table.
	go func(p *os.Process) {
		_, _ = p.Wait()
	}(cmd.Process)

	return app, record, nil
}

// procAliveLocked reports whether the tracked process record still points
// at the live process that was originally spawned.
func (m *Manager) procAliveLocked(id string) bool {
	rec, running := m.pidFile[id]
	if !running {
		return false
	}
	return procExists(rec.PID, rec.StartTicks)
}

// Stop terminates the whole process group: SIGTERM, escalate to SIGKILL.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.apps[id]
	if !exists {
		return ErrAppNotFound
	}
	rec, running := m.pidFile[id]
	if !running {
		return ErrNotRunning
	}
	pgid, err := syscall.Getpgid(rec.PID)
	if err != nil {
		pgid = rec.PID
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if !procExists(rec.PID, rec.StartTicks) {
			break
		}
		time.Sleep(80 * time.Millisecond)
	}
	if procExists(rec.PID, rec.StartTicks) {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	delete(m.pidFile, id)
	return m.persistLocked()
}

// Restart stops a running instance then starts it again.
func (m *Manager) Restart(id string) (*Snapshot, error) {
	err := m.Stop(id)
	if err != nil && !errors.Is(err, ErrNotRunning) {
		return nil, err
	}
	return m.Start(id)
}

// Delete removes the registry entry only for applications that are stopped.
// Application source code on disk is never touched by this operation.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.apps[id]
	if !exists {
		return ErrAppNotFound
	}
	if _, running := m.pidFile[id]; running {
		return ErrAlreadyRunning
	}
	delete(m.apps, id)
	return m.persistLocked()
}

// Snapshot returns the live derived state of one application.
func (m *Manager) Snapshot(id string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	app, exists := m.apps[id]
	if !exists {
		return nil, ErrAppNotFound
	}
	return m.buildSnapshotLocked(app, m.pidFile[id])
}

// List returns snapshots for all registered applications.
func (m *Manager) List() []*Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	// sweep stale records for dead processes
	for id, rec := range m.pidFile {
		if !procExists(rec.PID, rec.StartTicks) {
			delete(m.pidFile, id)
			_ = m.persistLocked()
		}
	}
	out := make([]*Snapshot, 0, len(m.apps))
	for _, app := range m.apps {
		if snap, err := m.buildSnapshotLocked(app, m.pidFile[app.ID]); err == nil {
			out = append(out, snap)
		}
	}
	return out
}

// Logs returns the tail of the application's captured stdout and stderr.
func (m *Manager) Logs(id string, maxLines int) ([]string, error) {
	m.mu.Lock()
	_, exists := m.apps[id]
	m.mu.Unlock()
	if !exists {
		return nil, ErrAppNotFound
	}
	return tailLines(m.logFilePath(id), maxLines), nil
}

func (m *Manager) logFilePath(id string) string {
	return filepath.Join(m.logDir, id+".log")
}

// buildSnapshotLocked derives status and live metrics from /proc for one app.
func (m *Manager) buildSnapshotLocked(app *App, rec *processRecord) (*Snapshot, error) {
	snap := &Snapshot{
		ID:          app.ID,
		Name:        app.Name,
		Runtime:     app.Runtime,
		Version:     app.Version,
		Path:        app.Path,
		EntryFile:   app.EntryFile,
		Port:        app.Port,
		Unit:        app.Unit,
		Environment: app.Environment,
		CreatedAt:   app.CreatedAt,
		Status:      "stopped",
	}
	if rec == nil || !procExists(rec.PID, rec.StartTicks) {
		return snap, nil
	}
	mem, cpu, elapsedSeconds, ok := readProcMetrics(rec.PID)
	if !ok {
		return snap, nil
	}
	snap.Status = "running"
	snap.PID = rec.PID
	snap.MemoryMb = math.Round(mem*10) / 10
	snap.CPUPercent = math.Round(cpu*100) / 100
	snap.Uptime = formatUptime(elapsedSeconds)
	return snap, nil
}

// procExists confirms the PID is alive AND belongs to the process we started,
// via the start_ticks recorded at spawn (guards against PID reuse).
func procExists(pid int, startTicks uint64) bool {
	if pid <= 0 {
		return false
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err != nil {
		return false
	}
	current := readProcStartTicks(pid)
	if startTicks != 0 && current != 0 && current != startTicks {
		return false
	}
	return true
}

func readProcStartTicks(pid int) uint64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	fields := fieldsAfterComm(data)
	// field 22 (1-indexed) is starttime; after stripping pid+comm it is index 19
	if len(fields) <= 19 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[19], 10, 64)
	return v
}

// readProcMetrics reads RSS bytes, cumulative CPU usage and wall time since
// start for a running process straight from /proc.
func readProcMetrics(pid int) (float64, float64, float64, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, 0, false
	}
	fields := fieldsAfterComm(data)
	if len(fields) <= 21 {
		return 0, 0, 0, false
	}
	utime, _ := strconv.ParseFloat(fields[11], 64) // field 14
	stime, _ := strconv.ParseFloat(fields[12], 64) // field 15
	startTicks, _ := strconv.ParseFloat(fields[19], 64)
	rssPages, _ := strconv.ParseFloat(fields[21], 64) // field 24

	pageSize := float64(os.Getpagesize())
	cpuSeconds := (utime + stime) / clockTicksPerSecond
	elapsed := startTicks / clockTicksPerSecond
	nowSeconds := currentBootSeconds()
	uptimeSeconds := nowSeconds - elapsed
	if uptimeSeconds <= 0 {
		uptimeSeconds = 0.001
	}
	cpuPercent := cpuSeconds / uptimeSeconds * 100
	memMb := rssPages * pageSize / (1024 * 1024)
	return memMb, cpuPercent, uptimeSeconds, true
}

// fieldsAfterComm splits /proc/<pid>/stat safely: the comm field may contain
// spaces and parentheses, so fields resume after the final closing bracket.
func fieldsAfterComm(data []byte) []string {
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 || idx+2 > len(s) {
		return nil
	}
	return strings.Fields(s[idx+2:])
}

func currentBootSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func tailLines(path string, maxLines int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	return lines[len(lines)-maxLines:]
}

func tailLastLineFrom(data []byte, maxLen int) string {
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	last := ""
	if len(lines) > 0 {
		last = lines[len(lines)-1]
	}
	if len(last) > maxLen {
		last = last[len(last)-maxLen:]
	}
	return last
}

func tailLastLine(path string, maxLen int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return tailLastLineFrom(data, maxLen)
}

func formatUptime(seconds float64) string {
	total := int64(seconds)
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

// persistLocked writes the registry and process bookkeeping to disk so the
// supervisor survives panel restarts.
func (m *Manager) persistLocked() error {
	type persistedState struct {
		Apps       map[string]*App           `json:"apps"`
		Processes  map[string]*processRecord `json:"processes"`
		NextID     int                       `json:"nextId"`
	}
	state := persistedState{Apps: m.apps, Processes: m.pidFile, NextID: m.nextID}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.baseDir, "state.json"), data, 0600)
}

func (m *Manager) loadState() {
	data, err := os.ReadFile(filepath.Join(m.baseDir, "state.json"))
	if err != nil {
		return
	}
	var state struct {
		Apps      map[string]*App           `json:"apps"`
		Processes map[string]*processRecord `json:"processes"`
		NextID    int                       `json:"nextId"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	if state.Apps != nil {
		m.apps = state.Apps
	}
	if state.Processes != nil {
		for id, rec := range stateProcesses(state.Processes) {
			if procExists(rec.PID, rec.StartTicks) {
				m.pidFile[id] = rec
			}
		}
	}
	m.nextID = state.NextID
}

func stateProcesses(src map[string]*processRecord) map[string]*processRecord {
	return src
}
