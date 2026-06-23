package singleinstance

import "errors"

// ErrAlreadyRunning возвращается, когда другой экземпляр клиента уже запущен.
var ErrAlreadyRunning = errors.New("another instance is already running")
