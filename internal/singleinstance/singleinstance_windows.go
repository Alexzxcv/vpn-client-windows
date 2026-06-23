//go:build windows

// Package singleinstance гарантирует, что одновременно работает только один
// экземпляр клиента (иначе — конфликт портов прокси/control-API и трея).
// На Windows используется именованный мьютекс ядра: он автоматически
// освобождается ОС при завершении процесса (в т.ч. при краше).
package singleinstance

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Lock — удерживаемый дескриптор именованного мьютекса.
type Lock struct {
	handle windows.Handle
}

// Acquire пытается захватить именованный мьютекс name. Если мьютекс уже
// существует (запущен другой экземпляр), возвращает ErrAlreadyRunning.
func Acquire(name string) (*Lock, error) {
	if name == "" {
		return nil, fmt.Errorf("singleinstance: empty name")
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("singleinstance: encode name: %w", err)
	}

	h, err := windows.CreateMutex(nil, false, namePtr)
	// CreateMutex возвращает валидный handle даже если объект уже существует;
	// в этом случае last-error == ERROR_ALREADY_EXISTS.
	if h == 0 {
		return nil, fmt.Errorf("singleinstance: create mutex: %w", err)
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(h)
		return nil, ErrAlreadyRunning
	}
	return &Lock{handle: h}, nil
}

// Release освобождает мьютекс. Безопасно вызывать повторно/из defer.
func (l *Lock) Release() {
	if l == nil || l.handle == 0 {
		return
	}
	_ = windows.CloseHandle(l.handle)
	l.handle = 0
}
