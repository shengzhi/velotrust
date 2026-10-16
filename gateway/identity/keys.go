package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type PublicKey struct {
	WireGuard string `json:"wireguard_public_key"`
}

type keyFile struct {
	name string
	data []byte
	mode os.FileMode
}

// Generate creates the node's WireGuard key pair in a private directory. The
// WireGuard public key is also the node's network identity. Existing key files
// are never overwritten.
func Generate(directory string) (PublicKey, error) {
	if strings.TrimSpace(directory) == "" {
		return PublicKey{}, errors.New("key directory is required")
	}
	absDirectory, err := filepath.Abs(directory)
	if err != nil {
		return PublicKey{}, fmt.Errorf("resolve key directory: %w", err)
	}
	if err := os.MkdirAll(absDirectory, 0o700); err != nil {
		return PublicKey{}, fmt.Errorf("create key directory: %w", err)
	}
	info, err := os.Lstat(absDirectory)
	if err != nil {
		return PublicKey{}, fmt.Errorf("inspect key directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return PublicKey{}, errors.New("key directory must be a regular directory, not a symlink")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return PublicKey{}, errors.New("key directory permissions must not grant group or other access")
	}
	if info.Mode().Perm()&0o700 != 0o700 {
		return PublicKey{}, errors.New("key directory must grant its owner read, write, and execute access")
	}

	wireGuardPrivate, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return PublicKey{}, fmt.Errorf("generate WireGuard key: %w", err)
	}
	public := PublicKey{
		WireGuard: wireGuardPrivate.PublicKey().String(),
	}
	files := []keyFile{
		{name: "wireguard.key", data: []byte(wireGuardPrivate.String() + "\n"), mode: 0o600},
		{name: "wireguard.pub", data: []byte(public.WireGuard + "\n"), mode: 0o644},
	}
	created := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(absDirectory, file.name)
		if err := writeExclusive(path, file.data, file.mode); err != nil {
			for _, createdPath := range created {
				_ = os.Remove(createdPath)
			}
			return PublicKey{}, fmt.Errorf("write %s: %w", file.name, err)
		}
		created = append(created, path)
	}
	return public, nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
