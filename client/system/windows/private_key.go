//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const maxPrivateKeyFileSize = 128
const privateFileAllAccess windows.ACCESS_MASK = 0x001F01FF

// GenerateWireGuardKey writes a new private key to a protected file and returns
// only its public key. The destination directory is restricted to the current
// user and LocalSystem before any key material is written.
func GenerateWireGuardKey(directory string) (string, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve key directory: %w", err)
	}
	if filepath.Dir(directory) == directory {
		return "", errors.New("key directory cannot be a filesystem root")
	}
	_, statErr := os.Lstat(directory)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := os.MkdirAll(directory, 0700); err != nil {
			return "", fmt.Errorf("create key directory: %w", err)
		}
		if err := setPrivateACL(directory, true); err != nil {
			return "", fmt.Errorf("protect key directory: %w", err)
		}
	} else if statErr != nil {
		return "", fmt.Errorf("inspect key directory: %w", statErr)
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("key directory must be a regular non-symlink directory")
	}
	if err := verifyPrivateACL(directory, true); err != nil {
		return "", fmt.Errorf("key directory permissions are unsafe: %w", err)
	}
	keyPath := filepath.Join(directory, "client.key")
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create private key file: %w", err)
	}
	keepFile := false
	defer func() {
		_ = file.Close()
		if !keepFile {
			_ = os.Remove(keyPath)
		}
	}()
	if err := setPrivateACL(keyPath, false); err != nil {
		return "", fmt.Errorf("protect private key file: %w", err)
	}
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", fmt.Errorf("generate WireGuard key: %w", err)
	}
	defer clear(privateKey[:])
	secret := []byte(privateKey.String() + "\n")
	defer clear(secret)
	if _, err := file.Write(secret); err != nil {
		return "", fmt.Errorf("write private key file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("flush private key file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close private key file: %w", err)
	}
	keepFile = true
	return privateKey.PublicKey().String(), nil
}

// LoadWireGuardPrivateKey accepts only a regular file whose protected DACL
// grants access to the current user and LocalSystem.
func LoadWireGuardPrivateKey(path string) (wgtypes.Key, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("inspect WireGuard private key: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxPrivateKeyFileSize {
		return wgtypes.Key{}, errors.New("WireGuard private key must be a small regular non-symlink file")
	}
	if err := verifyPrivateACL(path, false); err != nil {
		return wgtypes.Key{}, fmt.Errorf("WireGuard private key permissions are unsafe: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("open WireGuard private key: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return wgtypes.Key{}, errors.New("WireGuard private key changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPrivateKeyFileSize+1))
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("read WireGuard private key: %w", err)
	}
	if len(data) > maxPrivateKeyFileSize {
		return wgtypes.Key{}, errors.New("WireGuard private key file is too large")
	}
	value := bytes.TrimSpace(data)
	if bytes.ContainsAny(value, "\r\n\t ") {
		return wgtypes.Key{}, errors.New("WireGuard private key file must contain one base64 key")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(value))
	if err != nil || len(decoded) != wgtypes.KeyLen || base64.StdEncoding.EncodeToString(decoded) != string(value) {
		return wgtypes.Key{}, errors.New("WireGuard private key file contains an invalid key")
	}
	return wgtypes.ParseKey(string(value))
}

func setPrivateACL(path string, directory bool) error {
	userSID, err := currentUserSID()
	if err != nil {
		return err
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	inheritance := uint32(0)
	if directory {
		inheritance = windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	}
	entries := []windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: privateFileAllAccess,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(userSID),
			},
		},
		{
			AccessPermissions: privateFileAllAccess,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(systemSID),
			},
		},
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}

func verifyPrivateACL(path string, directory bool) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	if !isCurrentUserSID(owner) {
		return errors.New("private key must be owned by the current user")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("private key DACL must be protected from inheritance")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 2 {
		return errors.New("private key DACL must contain only the current user and LocalSystem")
	}
	userSID, err := currentUserSID()
	if err != nil {
		return err
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	seenUser, seenSystem := false, false
	expectedFlags := uint8(0)
	if directory {
		expectedFlags = windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	}
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != expectedFlags || ace.Mask != privateFileAllAccess {
			return errors.New("private key DACL contains an unsupported access rule")
		}
		sid := aceSID(ace)
		switch {
		case sid.Equals(userSID):
			seenUser = true
		case sid.Equals(systemSID):
			seenSystem = true
		default:
			return errors.New("private key DACL grants access to another principal")
		}
	}
	if !seenUser || !seenSystem {
		return errors.New("private key DACL is missing a required principal")
	}
	return nil
}

func currentUserSID() (*windows.SID, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid, nil
}

func isCurrentUserSID(sid *windows.SID) bool {
	user, err := currentUserSID()
	return err == nil && sid != nil && sid.Equals(user)
}

func aceSID(ace *windows.ACCESS_ALLOWED_ACE) *windows.SID {
	return (*windows.SID)(unsafe.Pointer(&ace.SidStart))
}
