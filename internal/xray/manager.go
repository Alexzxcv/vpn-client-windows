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
	"strings"
	"sync"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/procutil"
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

// assetDir resolves the directory holding geoip.dat/geosite.dat for xray. It
// checks XRAY_LOCATION_ASSET, then a "geo" dir next to the binary/executable,
// then the binary/executable dir itself. Returns "" if none contain geoip.dat.
func assetDir(bin string) string {
	if env := os.Getenv("XRAY_LOCATION_ASSET"); env != "" {
		return env
	}
	var dirs []string
	if d := filepath.Dir(bin); d != "" {
		dirs = append(dirs, filepath.Join(d, "geo"), d)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(d, "geo"), d)
	}
	for _, d := range dirs {
		if fileExists(filepath.Join(d, "geoip.dat")) {
			return d
		}
	}
	return ""
}

// Manager runs a single xray-core subprocess.
type Manager struct {
	log *slog.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	cfgPath string
	// gen увеличивается на каждый Start/Stop. Горутина-наблюдатель за процессом
	// сравнивает свой gen с текущим, чтобы отличить плановую остановку (Stop)
	// от аварийного падения и не дёргать onExit при намеренном kill.
	gen int

	// done закрывается, когда текущий xray-процесс завершился. WaitReady ждёт на
	// нём, чтобы упасть сразу при раннем выходе xray, не выжидая весь таймаут.
	done chan struct{}

	// onExit вызывается, когда xray-процесс завершился НЕ по команде Stop
	// (т.е. упал сам). Используется app для авто-переподключения.
	onExit func()

	// tailMu/tail хранят последние строки stderr xray для диагностики (видны
	// в ошибке connect, даже когда подробный лог xray уходит в Debug).
	tailMu sync.Mutex
	tail   []string
}

// maxTailLines — сколько последних строк stderr xray держим для диагностики.
const maxTailLines = 25

// appendTail добавляет строку stderr в кольцевой буфер диагностики.
func (m *Manager) appendTail(line string) {
	m.tailMu.Lock()
	m.tail = append(m.tail, line)
	if len(m.tail) > maxTailLines {
		m.tail = m.tail[len(m.tail)-maxTailLines:]
	}
	m.tailMu.Unlock()
}

// Tail returns the recent xray stderr lines joined for an error/diagnostic.
func (m *Manager) Tail() string {
	m.tailMu.Lock()
	defer m.tailMu.Unlock()
	return strings.TrimSpace(strings.Join(m.tail, " | "))
}

// NewManager creates a Manager. If log is nil the default slog logger is used.
func NewManager(log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{log: log}
}

// KillOrphans terminates stray xray processes left over from a previous run
// (crash-recovery) so they cannot hold the SOCKS/HTTP ports or the upstream
// connection. It only kills processes whose image is exactly our bundled
// xray binary; an unrelated "xray" elsewhere on the system is untouched. A
// no-op if the binary cannot be resolved. Returns the number terminated.
func (m *Manager) KillOrphans() int {
	bin, err := FindBinary()
	if err != nil {
		return 0
	}
	n, err := procutil.KillOrphansByPath(bin)
	if err != nil {
		m.log.Warn("xray orphan cleanup", slog.String("err", err.Error()))
		return 0
	}
	if n > 0 {
		m.log.Warn("terminated orphaned xray process(es) from a previous run", slog.Int("count", n))
	}
	return n
}

// Running reports whether an xray process is currently managed.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil
}

// SetExitHandler registers a callback invoked when the xray process exits on
// its own (a crash), but not when it is stopped via Stop. Pass nil to clear.
// Safe to call before Start. The callback runs on its own goroutine.
func (m *Manager) SetExitHandler(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExit = fn
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
	procutil.HideConsole(cmd) // no console window pop-up on connect
	// Point xray at the bundled geo databases (geoip.dat/geosite.dat) so the
	// "Russian sites direct" routing (geoip:ru / geosite:ru) resolves. If no
	// asset dir is found this is a no-op and xray uses its own defaults.
	if assetDir := assetDir(bin); assetDir != "" {
		cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+assetDir)
	}
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

	// Fresh diagnostics buffer + exit channel for this process.
	m.tailMu.Lock()
	m.tail = nil
	m.tailMu.Unlock()
	done := make(chan struct{})
	m.done = done

	go pipeLines(stdout, m.log, "xray.stdout", nil)
	go pipeLines(stderr, m.log, "xray.stderr", m.appendTail)
	go func() {
		err := cmd.Wait()
		close(done) // unblock WaitReady on early exit
		// Determine whether this was a planned stop or a crash: if our gen is
		// still the current one, nobody called Stop/Start in the meantime, so
		// the process died on its own.
		m.mu.Lock()
		crashed := m.gen == myGen
		m.mu.Unlock()
		if err != nil {
			// On a crash surface the captured stderr tail at Warn so the reason
			// (e.g. a bad config field) is visible without Debug logging.
			attrs := []any{slog.String("err", err.Error())}
			if crashed {
				if tail := m.Tail(); tail != "" {
					attrs = append(attrs, slog.String("stderr", tail))
				}
			}
			m.log.Warn("xray process exited", attrs...)
		} else {
			m.log.Info("xray process exited")
		}
		if crashed && onExit != nil {
			onExit()
		}
	}()

	_ = ctx // start is synchronous; ctx reserved for future use
	m.log.Info("xray started", slog.String("bin", bin))
	return nil
}

// pipeLines streams a reader line-by-line to the Debug log; if onLine is set,
// each line is also passed to it (used to keep a stderr tail for diagnostics).
func pipeLines(r io.Reader, log *slog.Logger, tag string, onLine func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		log.Debug(tag, slog.String("line", line))
		if onLine != nil {
			onLine(line)
		}
	}
}

// Stop kills the xray process (if any) and removes the temp config.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	// Bump gen so the in-flight Wait goroutine treats this exit as planned.
	m.gen++
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
// connection, the xray process exits, or ctx is done. On failure it includes the
// captured xray stderr tail so the actual reason (e.g. a rejected config field)
// is surfaced instead of a bare timeout.
func (m *Manager) WaitReady(ctx context.Context, socksPort int) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort))
	m.mu.Lock()
	done := m.done
	m.mu.Unlock()
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
		case <-done:
			// xray exited before the SOCKS port came up — report why.
			if tail := m.Tail(); tail != "" {
				return fmt.Errorf("xray exited before socks was ready on %s: %s", addr, tail)
			}
			return fmt.Errorf("xray exited before socks was ready on %s", addr)
		case <-ctx.Done():
			if tail := m.Tail(); tail != "" {
				return fmt.Errorf("xray socks not ready on %s: %w (xray: %s)", addr, ctx.Err(), tail)
			}
			return fmt.Errorf("xray socks not ready on %s: %w", addr, ctx.Err())
		case <-ticker.C:
		}
	}
}
