package xray

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// FindBinary resolves the xray executable path:
//  1. VPNCLIENT_XRAY_BIN env var, if set;
//  2. xray.exe next to the running executable (dir/bin/xray.exe, dir/xray.exe);
//  3. "xray"/"xray.exe" from PATH.
func FindBinary() (string, error) {
	if env := os.Getenv("VPNCLIENT_XRAY_BIN"); env != "" {
		if fileExists(env) {
			return env, nil
		}
		return "", fmt.Errorf("VPNCLIENT_XRAY_BIN points to missing file: %s", env)
	}

	name := "xray"
	if runtime.GOOS == "windows" {
		name = "xray.exe"
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, "bin", name),
			filepath.Join(dir, name),
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c, nil
			}
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("xray binary not found (set VPNCLIENT_XRAY_BIN or run `make xray`)")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Manager runs a single xray-core subprocess.
type Manager struct {
	log *slog.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	cfgPath string
}

// NewManager creates a Manager. If log is nil the default slog logger is used.
func NewManager(log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{log: log}
}

// Running reports whether an xray process is currently managed.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil
}

// Start writes the config to a temp file and launches `xray run -c <file>`,
// piping stdout/stderr to the logger. If a process is already running it is
// stopped first.
func (m *Manager) Start(ctx context.Context, configJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		m.stopLocked()
	}

	bin, err := FindBinary()
	if err != nil {
		return fmt.Errorf("start xray: %w", err)
	}

	f, err := os.CreateTemp("", "xray-config-*.json")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	if _, err := f.Write(configJSON); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return fmt.Errorf("close temp config: %w", err)
	}
	cfgPath := f.Name()

	// Use a background context for the process itself; lifecycle is controlled
	// via Stop(). ctx only guards the start operation.
	cmd := exec.Command(bin, "run", "-c", cfgPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.Remove(cfgPath)
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = os.Remove(cfgPath)
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(cfgPath)
		return fmt.Errorf("start process: %w", err)
	}

	m.cmd = cmd
	m.cfgPath = cfgPath

	go pipeLines(stdout, m.log, "xray.stdout")
	go pipeLines(stderr, m.log, "xray.stderr")
	go func() {
		err := cmd.Wait()
		if err != nil {
			m.log.Warn("xray process exited", slog.String("err", err.Error()))
		} else {
			m.log.Info("xray process exited")
		}
	}()

	_ = ctx // start is synchronous; ctx reserved for future use
	m.log.Info("xray started", slog.String("bin", bin))
	return nil
}

func pipeLines(r io.Reader, log *slog.Logger, tag string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		log.Debug(tag, slog.String("line", sc.Text()))
	}
}

// Stop kills the xray process (if any) and removes the temp config.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	var err error
	if m.cmd != nil && m.cmd.Process != nil {
		if kerr := m.cmd.Process.Kill(); kerr != nil {
			err = fmt.Errorf("kill xray: %w", kerr)
		}
		m.log.Info("xray stopped")
	}
	m.cmd = nil
	if m.cfgPath != "" {
		_ = os.Remove(m.cfgPath)
		m.cfgPath = ""
	}
	return err
}

// WaitReady polls a TCP connection to the SOCKS port until it accepts a
// connection or ctx is done.
func (m *Manager) WaitReady(ctx context.Context, socksPort int) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort))
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		d := net.Dialer{Timeout: 500 * time.Millisecond}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("xray socks not ready on %s: %w", addr, ctx.Err())
		case <-ticker.C:
		}
	}
}
