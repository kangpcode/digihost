// DigiPanel Runtime Module Tests (modules/runtimes/runtimes_test.go)
// These tests exercise real OS processes: detected interpreter binaries are
// spawned, inspected through /proc, and terminated. When a runtime binary is
// unavailable on the test host the corresponding scenario is skipped honestly.

package runtimes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func binaryAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writeTestApp(t *testing.T, dir string, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write entry file: %v", err)
	}
}

func freshManager(t *testing.T) *Manager {
	t.Helper()
	base := t.TempDir()
	return NewManager(base)
}

func TestDetectRuntimesMatchesHost(t *testing.T) {
	m := freshManager(t)
	detected := m.DetectedRuntimes()

	nodeOK := binaryAvailable("node")
	pyOK := binaryAvailable("python3") || binaryAvailable("python")
	goOK := binaryAvailable("go")

	if detected[RuntimeNodeJS] != "" && !nodeOK {
		t.Errorf("manager reports nodejs version but node is not on PATH")
	}
	if nodeOK && detected[RuntimeNodeJS] == "" {
		t.Errorf("node is available but manager failed to detect its version")
	}
	if detected[RuntimePython] != "" && !pyOK {
		t.Errorf("manager reports python version but python is not on PATH")
	}
	if detected[RuntimeGolang] != "" && !goOK {
		t.Errorf("manager reports go version but go is not on PATH")
	}

	t.Logf("detected runtimes: nodejs=%q python=%q golang=%q",
		detected[RuntimeNodeJS], detected[RuntimePython], detected[RuntimeGolang])
}

func TestCreateAppValidation(t *testing.T) {
	m := freshManager(t)
	dir := t.TempDir()
	writeTestApp(t, dir, "index.js", "setInterval(() => {}, 1000);\n")

	cases := []struct {
		label string
		req   CreateAppRequest
	}{
		{"invalid name", CreateAppRequest{Name: "x", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 3000}},
		{"invalid runtime", CreateAppRequest{Name: "demo", Runtime: "ruby", Path: dir, EntryFile: "index.js", Port: 3000}},
		{"relative path", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: "relative/path", EntryFile: "index.js", Port: 3000}},
		{"traversal path", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: dir + "/../escape", EntryFile: "index.js", Port: 3000}},
		{"missing entry", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: dir, EntryFile: "server.js", Port: 3000}},
		{"traversal entry", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: dir, EntryFile: "../index.js", Port: 3000}},
		{"low port", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 22}},
		{"high port", CreateAppRequest{Name: "demo", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 70000}},
	}
	for _, tc := range cases {
		if _, err := m.CreateApp(tc.req); err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.label)
		}
	}
}

func TestCreateAppDuplicateNameAndPort(t *testing.T) {
	m := freshManager(t)
	dir1 := t.TempDir()
	writeTestApp(t, dir1, "index.js", "setInterval(() => {}, 1000);\n")
	if _, err := m.CreateApp(CreateAppRequest{Name: "dup-demo", Runtime: "nodejs", Path: dir1, EntryFile: "index.js", Port: 4077}); err != nil {
		t.Skipf("node not detected on this host: %v", err)
	}
	dir2 := t.TempDir()
	writeTestApp(t, dir2, "index.js", "setInterval(() => {}, 1000);\n")
	if _, err := m.CreateApp(CreateAppRequest{Name: "dup-demo", Runtime: "nodejs", Path: dir2, EntryFile: "index.js", Port: 4078}); err != ErrDuplicateName {
		t.Errorf("expected ErrDuplicateName for same name, got %v", err)
	}
	if _, err := m.CreateApp(CreateAppRequest{Name: "other-name", Runtime: "nodejs", Path: dir2, EntryFile: "index.js", Port: 4077}); err != ErrDuplicatePort {
		t.Errorf("expected ErrDuplicatePort for same port, got %v", err)
	}
}

func TestSpawnStopNodeJS(t *testing.T) {
	if !binaryAvailable("node") {
		t.Skip("node binary not available on this host")
	}
	m := freshManager(t)
	dir := filepath.Join(t.TempDir(), "echoapp")
	writeTestApp(t, dir, "index.js", "console.log('digiEcho online');\nsetInterval(() => {}, 1000);\n")

	app, err := m.CreateApp(CreateAppRequest{Name: "digi-echo", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 4091})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if app.Version == "" {
		t.Fatalf("app version should come from detected interpreter")
	}

	snap, err := m.Start(app.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if snap.Status != "running" || snap.PID <= 0 {
		t.Fatalf("expected running with real pid, got status=%q pid=%d", snap.Status, snap.PID)
	}
	if _, err := os.Stat("/proc/" + itoa(snap.PID)); err != nil {
		t.Fatalf("pid %d missing from /proc", snap.PID)
	}

	time.Sleep(200 * time.Millisecond)
	logs, err := m.Logs(app.ID, 50)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "digiEcho online") {
		t.Errorf("captured stdout should contain app banner, got: %q", joined)
	}

	if err := m.Stop(app.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	snap2, err := m.Snapshot(app.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap2.Status != "stopped" || snap2.PID != 0 {
		t.Fatalf("expected stopped after Stop, got status=%q pid=%d", snap2.Status, snap2.PID)
	}
	if _, err := os.Stat("/proc/" + itoa(snap.PID)); !os.IsNotExist(err) {
		t.Fatalf("process %d still alive after Stop", snap.PID)
	}
}

func TestSpawnStopPython(t *testing.T) {
	if !binaryAvailable("python3") && !binaryAvailable("python") {
		t.Skip("python binary not available on this host")
	}
	m := freshManager(t)
	dir := filepath.Join(t.TempDir(), "pyworker")
	writeTestApp(t, dir, "main.py", "import time\nprint('digiPython worker ready', flush=True)\ntime.sleep(3600)\n")

	app, err := m.CreateApp(CreateAppRequest{Name: "digi-py", Runtime: "python", Path: dir, EntryFile: "main.py", Port: 4092, Environment: "staging"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	snap, err := m.Start(app.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if snap.Status != "running" || snap.MemoryMb <= 0 {
		t.Fatalf("expected running python process with real RSS, got status=%q mem=%f", snap.Status, snap.MemoryMb)
	}
	if _, err := os.Stat("/proc/" + itoa(snap.PID)); err != nil {
		t.Fatalf("python pid %d missing from /proc", snap.PID)
	}
	if err := m.Stop(app.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestBuildAndRunGolang(t *testing.T) {
	if !binaryAvailable("go") {
		t.Skip("go toolchain not available on this host")
	}
	m := freshManager(t)
	dir := filepath.Join(t.TempDir(), "gosvc")
	writeTestApp(t, dir, "main.go", "package main\n\nimport \"time\"\n\nfunc main() {\n\ttime.Sleep(time.Hour)\n}\n")

	app, err := m.CreateApp(CreateAppRequest{Name: "digi-go", Runtime: "golang", Path: dir, EntryFile: "main.go", Port: 4093})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	snap, err := m.Start(app.ID)
	if err != nil {
		t.Fatalf("Start (includes compile): %v", err)
	}
	if snap.Status != "running" || snap.MemoryMb <= 0 {
		t.Fatalf("expected running compiled binary, got status=%q mem=%f", snap.Status, snap.MemoryMb)
	}
	restartSnap, err := m.Restart(app.ID)
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if restartSnap.Status != "running" {
		t.Fatalf("expected running after restart, got %q", restartSnap.Status)
	}
	if restartSnap.PID == snap.PID {
		t.Fatalf("restart must spawn a new process, pid unchanged (%d)", restartSnap.PID)
	}
	if err := m.Stop(app.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestStartImmediateExitReportsFailure(t *testing.T) {
	if !binaryAvailable("node") {
		t.Skip("node binary not available on this host")
	}
	m := freshManager(t)
	dir := filepath.Join(t.TempDir(), "crasher")
	writeTestApp(t, dir, "index.js", "process.exit(3);\n")

	app, err := m.CreateApp(CreateAppRequest{Name: "digi-crash", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 4094})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if _, err := m.Start(app.ID); err == nil {
		t.Fatalf("expected immediate-exit start to fail")
	}
	snap, err := m.Snapshot(app.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Status != "stopped" || snap.PID != 0 {
		t.Fatalf("failed start must not leave a tracked process, got status=%q pid=%d", snap.Status, snap.PID)
	}
}

func TestDeleteRequiresStopped(t *testing.T) {
	if !binaryAvailable("node") {
		t.Skip("node binary not available on this host")
	}
	m := freshManager(t)
	dir := filepath.Join(t.TempDir(), "delapp")
	writeTestApp(t, dir, "index.js", "setInterval(() => {}, 1000);\n")

	app, err := m.CreateApp(CreateAppRequest{Name: "digi-del", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 4095})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if _, err := m.Start(app.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Delete(app.ID); err != ErrAlreadyRunning {
		t.Fatalf("expected ErrAlreadyRunning deleting a live app, got %v", err)
	}
	if err := m.Stop(app.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := m.Delete(app.ID); err != nil {
		t.Fatalf("Delete after Stop: %v", err)
	}
	if _, err := m.Snapshot(app.ID); err != ErrAppNotFound {
		t.Fatalf("expected ErrAppNotFound after delete, got %v", err)
	}
}

func TestRegistryPersistsAcrossReload(t *testing.T) {
	if !binaryAvailable("node") {
		t.Skip("node binary not available on this host")
	}
	dir := filepath.Join(t.TempDir(), "persist-app")
	writeTestApp(t, dir, "index.js", "setInterval(() => {}, 1000);\n")

	base := t.TempDir()
	m1 := NewManager(base)
	if _, err := m1.CreateApp(CreateAppRequest{Name: "digi-persist", Runtime: "nodejs", Path: dir, EntryFile: "index.js", Port: 4096}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	m2 := NewManager(base)
	snaps := m2.List()
	found := false
	for _, s := range snaps {
		if s.Name == "digi-persist" {
			found = true
			if s.Status != "stopped" {
				t.Errorf("reloaded app should be stopped, got %q", s.Status)
			}
		}
	}
	if !found {
		t.Fatalf("registry did not persist to state.json")
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
