// Package customserver хранит пользовательские VLESS-серверы, добавленные
// вручную (вставкой vless://-ссылки или импортом подписки), и парсит vless-ссылки.
//
// Кастомные серверы — это ЧУЖИЕ ноды, не наши: подключение к ним идёт напрямую
// по сохранённому конфигу, МИНУЯ backend (нет регистрации устройства, нет
// /vpn/config, трафик не учитывается и подписка не ограничивает). Хранятся
// локально в %APPDATA%/sapn-vpn/custom_servers.json.
//
// Никогда не логируем uuid/reality-ключи и содержимое конфигов.
package customserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
)

// IDPrefix помечает location-id кастомного сервера в общем списке локаций,
// чтобы connect-флоу отличал их от backend-нод (см. app.CustomPrefix).
const IDPrefix = "custom:"

// Server — пользовательский VLESS-сервер. Поля плоские (а не вложенный
// VLESSConfig) ради стабильного формата файла. ExpiresAt у кастомных нет.
type Server struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid"`
	Security    string `json:"security"`
	Flow        string `json:"flow"`
	PublicKey   string `json:"public_key"`
	ShortID     string `json:"short_id"`
	SNI         string `json:"sni"`
	Fingerprint string `json:"fingerprint"`
}

// VLESSConfig преобразует кастомный сервер в backend.VLESSConfig, который умеют
// принимать генераторы конфигов xray/sing-box. ExpiresAt остаётся нулевым
// (рефреш для кастомных не нужен).
func (s Server) VLESSConfig() backend.VLESSConfig {
	return backend.VLESSConfig{
		Host:        s.Host,
		Port:        s.Port,
		UUID:        s.UUID,
		Security:    s.Security,
		Flow:        s.Flow,
		PublicKey:   s.PublicKey,
		ShortID:     s.ShortID,
		SNI:         s.SNI,
		Fingerprint: s.Fingerprint,
	}
}

// Link реконструирует vless://-ссылку сервера (обратная операция к ParseVLESS):
//
//	vless://<uuid>@<host>:<port>?type=tcp&encryption=none&security=..&sni=..&fp=..&pbk=..&sid=..&flow=..#name
//
// Пустые reality-поля опускаются. Подходит для копирования пользователем и
// для повторного импорта в любой VLESS-клиент.
func (s Server) Link() string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("encryption", "none")
	security := s.Security
	if security == "" {
		security = "reality"
	}
	q.Set("security", security)
	if s.SNI != "" {
		q.Set("sni", s.SNI)
	}
	fp := s.Fingerprint
	if fp == "" {
		fp = "chrome"
	}
	q.Set("fp", fp)
	if s.PublicKey != "" {
		q.Set("pbk", s.PublicKey)
	}
	if s.ShortID != "" {
		q.Set("sid", s.ShortID)
	}
	if s.Flow != "" {
		q.Set("flow", s.Flow)
	}
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(s.UUID),
		Host:     net.JoinHostPort(s.Host, strconv.Itoa(s.Port)),
		RawQuery: q.Encode(),
		Fragment: s.Name,
	}
	return u.String()
}

// ParseVLESS разбирает одну vless://-ссылку в Server. Формат:
//
//	vless://<uuid>@<host>:<port>?security=reality&pbk=..&sid=..&sni=..&fp=..&flow=..#name
//
// ID генерируется случайно. Имя берётся из фрагмента (#...), иначе host.
func ParseVLESS(raw string) (Server, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "vless://") {
		return Server{}, fmt.Errorf("ссылка должна начинаться с vless://")
	}
	u, err := url.Parse(s)
	if err != nil {
		return Server{}, fmt.Errorf("неверная vless-ссылка: %w", err)
	}
	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	if uuid == "" {
		return Server{}, fmt.Errorf("в ссылке нет UUID")
	}
	host := u.Hostname()
	if host == "" {
		return Server{}, fmt.Errorf("в ссылке нет хоста")
	}
	port := 0
	if p := u.Port(); p != "" {
		if _, e := fmt.Sscanf(p, "%d", &port); e != nil {
			port = 0
		}
	}
	if port <= 0 {
		return Server{}, fmt.Errorf("в ссылке нет порта")
	}

	q := u.Query()
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(q.Get(k)); v != "" {
				return v
			}
		}
		return ""
	}

	name := host
	if frag := strings.TrimSpace(u.Fragment); frag != "" {
		if dec, e := url.QueryUnescape(frag); e == nil && dec != "" {
			name = dec
		} else {
			name = frag
		}
	}

	security := get("security")
	if security == "" {
		security = "reality"
	}
	sni := get("sni", "peer")
	if sni == "" {
		sni = host
	}
	fingerprint := get("fp")
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	id, err := newID()
	if err != nil {
		return Server{}, err
	}
	return Server{
		ID:          id,
		Name:        name,
		Host:        host,
		Port:        port,
		UUID:        uuid,
		Security:    security,
		Flow:        get("flow"),
		PublicKey:   get("pbk"),
		ShortID:     get("sid"),
		SNI:         sni,
		Fingerprint: fingerprint,
	}, nil
}

// ParseSubscription разбирает тело подписки (base64 или plain-текст) в список
// серверов: декодирует, берёт строки vless:// и парсит каждую. Нечитаемые
// строки пропускаются. Сетевой запрос делает вызывающий — пакет чистый.
func ParseSubscription(body string) []Server {
	text := decodeSubscription(body)
	var out []Server
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "vless://") {
			continue
		}
		if srv, err := ParseVLESS(line); err == nil {
			out = append(out, srv)
		}
	}
	return out
}

// decodeSubscription декодирует base64-подписку; если это уже plain-текст со
// ссылками vless:// — возвращает как есть.
func decodeSubscription(body string) string {
	t := strings.TrimSpace(body)
	if strings.HasPrefix(t, "vless://") {
		return t
	}
	// Подписки часто в std или url-safe base64, с паддингом или без.
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if dec, err := enc.DecodeString(strings.ReplaceAll(t, "\n", "")); err == nil {
			if s := string(dec); strings.Contains(s, "vless://") {
				return s
			}
		}
	}
	return t
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("custom server: gen id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("custom servers: user config dir: %w", err)
	}
	return filepath.Join(dir, "sapn-vpn", "custom_servers.json"), nil
}

// Store — потокобезопасное файловое хранилище кастомных серверов.
type Store struct {
	mu   sync.RWMutex
	list []Server
	path string
}

// Load читает custom_servers.json (пустой список, если файла нет/он битый).
func Load() *Store {
	p, _ := filePath()
	st := &Store{path: p}
	if p == "" {
		return st
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return st
	}
	var l []Server
	if json.Unmarshal(data, &l) == nil {
		st.list = l
	}
	return st
}

// List возвращает копию списка серверов.
func (st *Store) List() []Server {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]Server, len(st.list))
	copy(out, st.list)
	return out
}

// Get возвращает сервер по id.
func (st *Store) Get(id string) (Server, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	for _, s := range st.list {
		if s.ID == id {
			return s, true
		}
	}
	return Server{}, false
}

// Add добавляет сервер и сохраняет файл.
func (st *Store) Add(s Server) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.list = append(st.list, s)
	return st.persist()
}

// AddAll добавляет несколько серверов одной транзакцией (для импорта подписки).
func (st *Store) AddAll(servers []Server) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.list = append(st.list, servers...)
	return st.persist()
}

// Remove удаляет сервер по id и сохраняет файл.
func (st *Store) Remove(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	filtered := st.list[:0:0]
	for _, s := range st.list {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	st.list = filtered
	return st.persist()
}

// persist пишет текущий список на диск (вызывать под удержанным mu).
func (st *Store) persist() error {
	if st.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st.list, "", "  ")
	if err != nil {
		return fmt.Errorf("custom servers: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o700); err != nil {
		return fmt.Errorf("custom servers: mkdir: %w", err)
	}
	if err := os.WriteFile(st.path, data, 0o600); err != nil {
		return fmt.Errorf("custom servers: write: %w", err)
	}
	return nil
}
