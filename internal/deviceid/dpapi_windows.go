//go:build windows

package deviceid

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CRYPTPROTECT_UI_FORBIDDEN: no UI prompts; fail instead (we run headless/tray).
const cryptprotectUIForbidden = 0x1

// dataBlob mirrors the Win32 DATA_BLOB struct used by CryptProtectData /
// CryptUnprotectData.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func newBlob(d []byte) dataBlob {
	if len(d) == 0 {
		return dataBlob{}
	}
	return dataBlob{
		cbData: uint32(len(d)),
		pbData: &d[0],
	}
}

func (b dataBlob) bytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

// seal encrypts plaintext with DPAPI scoped to the current user. The result is
// opaque ciphertext that only this Windows user account can decrypt; the raw
// key never touches the disk unencrypted.
func seal(plaintext []byte) ([]byte, error) {
	in := newBlob(plaintext)
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // description
		0, // optional entropy
		0, // reserved
		0, // prompt struct
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

// unseal reverses seal, returning the original plaintext.
func unseal(ciphertext []byte) ([]byte, error) {
	in := newBlob(ciphertext)
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // description out (ignored)
		0, // optional entropy
		0, // reserved
		0, // prompt struct
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
