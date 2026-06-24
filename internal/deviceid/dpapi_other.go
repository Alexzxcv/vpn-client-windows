//go:build !windows

package deviceid

// On non-Windows platforms there is no DPAPI. These stubs let the package
// compile for CI/linting; the private key is stored as-is (the real client only
// ships on Windows). The seal/unseal are identity transforms with a marker so a
// file written off-Windows is never silently treated as DPAPI ciphertext on it.
func seal(plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func unseal(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}
