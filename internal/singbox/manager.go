package singbox

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// FindBinary resolves the sing-box executable path:
//  1. VPNCLIENT_SINGBOX_BIN env var, if set;
//  2. sing-box.exe next to the running executable (dir/bin/sing-box.exe,
//     dir/sing-box.exe);
//  3. "sing-box"/"sing-box.exe" from PATH.
func FindBinary() (string, error) {
	if env := os.Getenv("VPNCLIENT_SINGBOX_BIN"); env != "" {
		if fileExists(env) {
			return env, nil
		}
		return "", fmt.Errorf("VPNCLIENT_SINGBOX_BIN points to missing file: %s", env)
	}

	name := "sing-box"
	if runtime.GOOS == "windows" {
		name = "sing-box.exe"
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
	return "", fmt.Errorf("sing-box binary not found (set VPNCLIENT_SINGBOX_BIN or run `make singbox`)")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Manager runs a single sing-box subprocess (TUN engine).
type Manager struct {
	log *slog.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	cfgPath string
	// gen увеличивается на каждый Start/Stop. Горутина-наблюдатель сравнивает
	// свой gen с текущим, чтобы отличить плановую остановку от падения и не
	// дёргать onExit при намеренном kill.
	gen int

	// onExit вызывается, когда sing-box завершился НЕ по команде Stop (упал
	// сам). Используется app для авто-переподключения.
	onExit func()
}

// NewManager creates a Manager. If log is nil the default slog logger is used.
func NewManager(log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{log: log}
}

// Running reports whether a sing-box process is currently managed.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil
}

// SetExitHandler registers a callback invoked when the sing-box process exits on
// its own (a crash), but not when it is stopped via Stop. Pass nil to clear.
// Safe to call before Start. The callback runs on its own goroutine.
func (m *Manager) SetExitHandler(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExit = fn
}

// Start writes the config to a temp file and launches `sing-box run -c <file>`,
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
		return fmt.Errorf("start sing-box: %w", err)
	}

	f, err := os.CreateTemp("", "singbox-config-*.json")
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

	// Process lifecycle is controlled via Stop(); ctx only guards the start op.
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
	m.gen++
	myGen := m.gen
	onExit := m.onExit

	go pipeLines(stdout, m.log, "singbox.stdout")
	go pipeLines(stderr, m.log, "singbox.stderr")
	go func() {
		err := cmd.Wait()
		if err != nil {
			m.log.Warn("sing-box process exited", slog.String("err", err.Error()))
		} else {
			m.log.Info("sing-box process exited")
		}
		m.mu.Lock()
		crashed := m.gen == myGen
		m.mu.Unlock()
		if crashed && onExit != nil {
			onExit()
		}
	}()

	_ = ctx
	m.log.Info("sing-box started", slog.String("bin", bin))
	return nil
}

func pipeLines(r io.Reader, log *slog.Logger, tag string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		log.Debug(tag, slog.String("line", sc.Text()))
	}
}

// Stop kills the sing-box process (if any) and removes the temp config.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	m.gen++
	var err error
	if m.cmd != nil && m.cmd.Process != nil {
		if kerr := m.cmd.Process.Kill(); kerr != nil {
			err = fmt.Errorf("kill sing-box: %w", kerr)
		}
		m.log.Info("sing-box stopped")
	}
	m.cmd = nil
	if m.cfgPath != "" {
		_ = os.Remove(m.cfgPath)
		m.cfgPath = ""
	}
	return err
}

// WaitReady confirms the process is alive and gives the TUN interface a short
// moment to come up. sing-box has no local probe port (unlike the proxy SOCKS
// inbound), so readiness is process-liveness + a brief settle delay. ctx bounds
// the wait.
func (m *Manager) WaitReady(ctx context.Context) error {
	// Дать процессу шанс сразу упасть на ошибке конфига/прав.
	select {
	case <-ctx.Done():
		return fmt.Errorf("sing-box wait ready: %w", ctx.Err())
	case <-time.After(1500 * time.Millisecond):
	}

	m.mu.Lock()
	alive := m.cmd != nil && m.cmd.Process != nil
	m.mu.Unlock()
	if !alive {
		return fmt.Errorf("sing-box exited before becoming ready (check admin rights / wintun.dll)")
	}
	return nil
}
