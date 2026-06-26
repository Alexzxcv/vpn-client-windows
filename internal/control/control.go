// Package control hosts the local control HTTP server consumed by the React UI.
//
// It listens only on 127.0.0.1 on a free port, protects /api/* with a one-shot
// session token plus Origin/Host checks, and serves the built UI as static
// files (with a placeholder when the UI is not built).
package control

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Alexzxcv/vpn-client-windows/internal/app"
	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/elevation"
	"github.com/Alexzxcv/vpn-client-windows/internal/settings"
)

// WindowController lets the control server drive the native window (frameless
// title-bar buttons). Implemented by the cmd/vpnclient window manager; nil in
// headless/test builds. Methods are safe to call from the HTTP goroutine.
type WindowController interface {
	Minimize()
	ToggleMaximize()
	Close()
}

// Server is the local control server.
type Server struct {
	log   *slog.Logger
	app   *app.App
	token string
	uiDir string
	win   WindowController

	httpSrv  *http.Server
	listener net.Listener
}

// SetWindowController wires the native window controller used by the
// /api/window/* endpoints. Optional; when unset those endpoints are no-ops.
func (s *Server) SetWindowController(w WindowController) { s.win = w }

// New creates a control Server. log may be nil. uiDir is the directory of the
// built UI; if empty it is resolved at Start time.
func New(log *slog.Logger, application *app.App, uiDir string) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	tok, err := genToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}
	return &Server{
		log:   log,
		app:   application,
		token: tok,
		uiDir: uiDir,
	}, nil
}

// Token returns the one-shot session token.
func (s *Server) Token() string { return s.token }

// Addr returns the actual listen address (host:port) after Start.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Port returns the actual TCP port after Start.
func (s *Server) Port() int {
	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

// URL returns the base URL the UI window should open.
func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", s.Port())
}

func genToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Start binds to 127.0.0.1 on a free port and begins serving. It returns once
// the listener is open; serving continues in a background goroutine.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen 127.0.0.1: %w", err)
	}
	s.listener = ln

	if s.uiDir == "" {
		s.uiDir = resolveUIDir()
	}

	s.httpSrv = &http.Server{
		Handler:           s.router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("control server error", slog.String("err", err.Error()))
		}
	}()

	s.log.Info("control server listening", slog.String("addr", s.Addr()))
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) router() http.Handler {
	r := chi.NewRouter()

	r.Route("/api", func(api chi.Router) {
		api.Use(s.localOnly)
		// bootstrap is the only public endpoint.
		api.Get("/bootstrap", s.handleBootstrap)

		api.Group(func(authed chi.Router) {
			authed.Use(s.requireToken)
			authed.Get("/status", s.handleStatus)
			authed.Post("/auth/login", s.handleLogin)
			authed.Post("/auth/logout", s.handleLogout)
			authed.Get("/me", s.handleMe)
			authed.Get("/locations", s.handleLocations)
			// Кастомные (пользовательские) серверы: список/добавление/удаление.
			authed.Get("/custom-servers", s.handleListCustomServers)
			authed.Post("/custom-servers", s.handleAddCustomServer)
			authed.Delete("/custom-servers/{id}", s.handleRemoveCustomServer)
			authed.Get("/usage", s.handleUsage)
			authed.Post("/connect", s.handleConnect)
			authed.Post("/disconnect", s.handleDisconnect)
			authed.Get("/proxy", s.handleProxy)
			authed.Get("/settings", s.handleGetSettings)
			authed.Put("/settings", s.handlePutSettings)
			authed.Get("/update/check", s.handleUpdateCheck)
			authed.Post("/update/apply", s.handleUpdateApply)
			// Открыть URL во внешнем (системном) браузере — для кнопки
			// «Создать аккаунт», ведущей на веб-дашборд.
			authed.Post("/open-external", s.handleOpenExternal)
			// Native window controls for the custom (frameless) title bar.
			authed.Post("/window/minimize", s.handleWindowMinimize)
			authed.Post("/window/maximize", s.handleWindowMaximize)
			authed.Post("/window/close", s.handleWindowClose)
		})
	})

	// Static UI + SPA fallback for everything else.
	r.NotFound(s.serveUI)
	r.Get("/*", s.serveUI)

	return r
}

// ----- middleware -----

// localOnly rejects requests with a non-local Origin or Host.
func (s *Server) localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			s.log.Warn("rejected non-local Host", slog.String("host", r.Host))
			writeErr(w, http.StatusForbidden, "non-local host")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !isLocalOrigin(origin) {
			s.log.Warn("rejected non-local Origin", slog.String("origin", origin))
			writeErr(w, http.StatusForbidden, "non-local origin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireToken enforces the Bearer session token.
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			writeErr(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		got := strings.TrimPrefix(auth, prefix)
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalHost(host string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	switch h {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return false
}

func isLocalOrigin(origin string) bool {
	// origin is like http://127.0.0.1:NNN
	o := origin
	for _, p := range []string{"http://", "https://"} {
		o = strings.TrimPrefix(o, p)
	}
	return isLocalHost(o)
}

// ----- handlers -----

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"session_token": s.token,
		"api_base":      s.app.APIBase(),
		"version":       s.app.Version(),
		// elevated сообщает UI, доступен ли TUN-режим без перезапуска от админа.
		"elevated": elevation.IsElevated(),
		// dashboard_url — адрес веб-дашборда для кнопки «Создать аккаунт».
		"dashboard_url": s.app.DashboardURL(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Status())
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login"` // email или username
		Email    string `json:"email"` // обратная совместимость
		Password string `json:"password"`
		OTP      string `json:"otp"` // TOTP-код, если у аккаунта включён 2FA
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	login := body.Login
	if login == "" {
		login = body.Email
	}
	if err := s.app.Login(r.Context(), login, body.Password, strings.TrimSpace(body.OTP)); err != nil {
		switch {
		case errors.Is(err, backend.ErrMFARequired):
			// Аккаунт с 2FA: просим у UI код. Тот же код "mfa_required", что и в
			// веб-панели — UI показывает поле для TOTP и повторяет логин с otp.
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "mfa code required",
				"code":  "mfa_required",
			})
		case errors.Is(err, backend.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "invalid login or password",
				"code":  "invalid_credentials",
			})
		default:
			writeErr(w, http.StatusUnauthorized, "login failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.app.Logout(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.app.Me(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "me failed")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLocations(w http.ResponseWriter, r *http.Request) {
	locs, err := s.app.Locations(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "locations failed")
		return
	}
	if locs == nil {
		locs = []app.LocationView{}
	}
	writeJSON(w, http.StatusOK, locs)
}

// handleListCustomServers returns the user's manually-added custom servers.
func (s *Server) handleListCustomServers(w http.ResponseWriter, r *http.Request) {
	list := s.app.ListCustomServers()
	if list == nil {
		list = []app.CustomServerView{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleAddCustomServer adds a custom server from a vless:// link or imports a
// subscription URL (http(s)). Returns how many were added.
func (s *Server) handleAddCustomServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	n, err := s.app.AddCustomServer(r.Context(), body.Input)
	if err != nil {
		// Parse/import errors are user-facing (e.g. "в ссылке нет UUID").
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": n})
}

// handleRemoveCustomServer deletes a custom server by id.
func (s *Server) handleRemoveCustomServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing id")
		return
	}
	if err := s.app.RemoveCustomServer(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "remove failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleUsage returns the current traffic totals, the optional free-daily
// allowance and the windowed traffic samples for the connect-page sparkline.
// The window defaults to 24h; override with ?hours=N.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	usage, err := s.app.Usage(r.Context(), hours)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "usage failed")
		return
	}
	if usage.Samples == nil {
		usage.Samples = []backend.UsageSample{}
	}
	writeJSON(w, http.StatusOK, usage)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerID *string `json:"server_id"`
		Mode     string  `json:"mode"`
	}
	// Body is optional; mode defaults to proxy.
	_ = json.NewDecoder(r.Body).Decode(&body)

	state, err := s.app.Connect(r.Context(), body.ServerID, body.Mode)
	if err != nil {
		// Surface the actual reason (e.g. "TUN requires administrator") so the UI
		// can show it. No secrets are present in connect errors.
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"state": string(state),
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": string(state)})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Disconnect(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"state": string(app.StateError),
			"error": "disconnect failed",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": string(app.StateDisconnected)})
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"socks": fmt.Sprintf("127.0.0.1:%d", s.app.SocksPort()),
		"http":  fmt.Sprintf("127.0.0.1:%d", s.app.HTTPPort()),
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Settings())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	saved, err := s.app.SaveSettings(body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save settings failed")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// handleUpdateCheck queries GitHub Releases for a newer build. It returns the
// cached result if a background check already found one and the network is
// unavailable, so the UI still shows a pending update.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.CheckUpdate(r.Context())
	if err != nil {
		if cached := s.app.LastUpdate(); cached != nil {
			writeJSON(w, http.StatusOK, cached)
			return
		}
		writeErr(w, http.StatusBadGateway, "update check failed")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleUpdateApply downloads and launches the cached release installer. This is
// only reached on an explicit user action in the UI (the "Обновить" button); the
// client never auto-applies an update.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if err := s.app.ApplyUpdate(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleOpenExternal opens an http(s) URL in the user's default system browser.
// Used by the "create account" button to send the user to the web dashboard.
func (s *Server) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.app.OpenExternal(body.URL); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ----- window controls -----

func (s *Server) handleWindowMinimize(w http.ResponseWriter, r *http.Request) {
	if s.win != nil {
		s.win.Minimize()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWindowMaximize(w http.ResponseWriter, r *http.Request) {
	if s.win != nil {
		s.win.ToggleMaximize()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWindowClose(w http.ResponseWriter, r *http.Request) {
	if s.win != nil {
		s.win.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- static UI -----

const placeholderHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>VPN Client</title>
<style>body{font-family:system-ui,sans-serif;background:#0f1115;color:#e6e6e6;
display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{text-align:center}code{background:#1c2030;padding:2px 6px;border-radius:4px}</style>
</head><body><div class="box"><h2>UI not built</h2>
<p>Run <code>make ui</code> to build the React interface.</p></div></body></html>`

func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	// Never serve /api/* from the static handler.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	if s.uiDir == "" || !dirExists(s.uiDir) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(placeholderHTML))
		return
	}

	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean == "." || clean == "" {
		clean = "index.html"
	}
	full := filepath.Join(s.uiDir, clean)

	// Prevent path traversal outside uiDir.
	if rel, err := filepath.Rel(s.uiDir, full); err != nil || strings.HasPrefix(rel, "..") {
		writeErr(w, http.StatusBadRequest, "bad path")
		return
	}

	if fileExists(full) {
		http.ServeFile(w, r, full)
		return
	}

	// SPA fallback: serve index.html for unknown non-asset routes.
	index := filepath.Join(s.uiDir, "index.html")
	if fileExists(index) {
		http.ServeFile(w, r, index)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(placeholderHTML))
}

// resolveUIDir picks the UI directory: VPNCLIENT_UI_DIR, else frontend/dist next
// to the executable, else frontend/dist relative to cwd.
func resolveUIDir() string {
	if env := os.Getenv("VPNCLIENT_UI_DIR"); env != "" {
		return env
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{
			filepath.Join(dir, "ui"),                     // release layout: ui/ рядом с exe
			filepath.Join(dir, "frontend", "dist"),       // dist рядом с exe
			filepath.Join(dir, "..", "frontend", "dist"), // dev: bin\..\frontend\dist
		} {
			if dirExists(cand) {
				return cand
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		cand := filepath.Join(wd, "frontend", "dist")
		if dirExists(cand) {
			return cand
		}
	}
	return ""
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ----- helpers -----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}
