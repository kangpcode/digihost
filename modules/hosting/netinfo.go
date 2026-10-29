// DigiPanel Hosting Network and Log Helpers (modules/hosting/netinfo.go)
// Real host inspection used by account provisioning, telemetry, and the log
// viewer. No shell execution, no fabricated addresses, no fallback constants.

package hosting

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PrimaryIPv4 returns the global unicast IPv4 address bound to the interface
// that carries the default route. An empty string means the host has no
// routable IPv4 address, and callers must surface that instead of inventing one.
func PrimaryIPv4() string {
	if iface := defaultRouteInterface(); iface != "" {
		if addr := firstGlobalIPv4(iface); addr != "" {
			return addr
		}
	}
	return firstGlobalIPv4("")
}

func defaultRouteInterface() string {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		// The gateway column being all zeroes indicates the default route.
		return fields[0]
	}
	return ""
}

func firstGlobalIPv4(ifaceName string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if ifaceName != "" && iface.Name != ifaceName {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

// AccountLogNames lists the log files the panel is allowed to expose for an
// account. Access outside this closed set is rejected.
var AccountLogNames = map[string]bool{
	"access.log": true,
	"error.log":  true,
}

// ReadAccountLog returns at most maxLines lines from the requested account log.
// The second return value reports whether the file exists: a freshly created
// account legitimately has no traffic log yet.
func (m *Manager) ReadAccountLog(username, logName string, maxLines int) ([]string, bool, error) {
	if !AccountLogNames[logName] {
		return nil, false, fmt.Errorf("unsupported log name %q", logName)
	}
	if maxLines <= 0 || maxLines > 2000 {
		maxLines = 200
	}

	account, err := m.GetAccount(username)
	if err != nil {
		return nil, false, err
	}

	logPath := filepath.Join(account.HomeDir, "logs", logName)
	resolved, err := filepath.EvalSymlinks(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, false, nil
		}
		return nil, false, fmt.Errorf("resolve log path: %w", err)
	}
	if !strings.HasPrefix(resolved, filepath.Clean(account.HomeDir)+string(os.PathSeparator)) {
		return nil, false, fmt.Errorf("log path escapes account home")
	}

	file, err := os.Open(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, false, nil
		}
		return nil, false, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	lines, err := tailLines(file, maxLines)
	if err != nil {
		return nil, true, err
	}
	return lines, true, nil
}

func tailLines(reader io.Reader, maxLines int) ([]string, error) {
	ring := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if len(ring) == maxLines {
			copy(ring, ring[1:])
			ring[len(ring)-1] = line
			continue
		}
		ring = append(ring, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ring, nil
}

// measureDirectory reports the real disk bytes and inode count beneath a path.
// Symlinks are counted but never followed, so an account cannot inflate or
// redirect the measurement outside its own tree.
func measureDirectory(root string) (int64, int, error) {
	var bytes int64
	inodes := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		inodes++
		if entry.Type().IsRegular() {
			info, statErr := entry.Info()
			if statErr == nil {
				bytes += info.Size()
			}
		}
		return nil
	})
	return bytes, inodes, err
}

// RefreshUsage recomputes disk and inode usage for an account and stores the
// result. It returns a copy so callers never alias mutable manager state.
func (m *Manager) RefreshUsage(username string) (*HostingAccount, error) {
	account, err := m.GetAccount(username)
	if err != nil {
		return nil, err
	}

	bytes, inodes, err := measureDirectory(account.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("measure account usage: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stored, exists := m.accounts[username]
	if !exists {
		return nil, ErrAccountNotFound
	}
	stored.DiskUsedBytes = bytes
	stored.InodesUsed = inodes
	stored.UpdatedAt = time.Now()

	copied := *stored
	return &copied, nil
}

// FormatAccountUID keeps the POSIX identity rendering consistent across the API
// and the UI.
func FormatAccountUID(uid int) string {
	return strconv.Itoa(uid)
}
