package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	vaultMagic   = "SSHELFV1"
	vaultVersion = byte(1)
	saltLen      = 16
	nonceLen     = 12
	kdfRounds    = 100_000
)

// BackupKeys creates an AES-256-GCM encrypted archive of all key files in keysDir
// and writes it to destPath. Returns the number of key files included.
func BackupKeys(keysDir, destPath, passphrase string) (int, error) {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return 0, fmt.Errorf("read keys dir: %w", err)
	}

	// Build tar.gz in memory.
	var tarBuf bytes.Buffer
	gz := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gz)

	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip backup files and hidden files; include private keys and .pub files.
		if strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := filepath.Join(keysDir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return 0, fmt.Errorf("read key %s: %w", name, err)
		}
		info, _ := e.Info()
		hdr := &tar.Header{
			Name:    name,
			Size:    int64(len(data)),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return 0, fmt.Errorf("tar header %s: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return 0, fmt.Errorf("tar write %s: %w", name, err)
		}
		count++
	}
	if err := tw.Close(); err != nil {
		return 0, fmt.Errorf("close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return 0, fmt.Errorf("close gzip: %w", err)
	}
	if count == 0 {
		return 0, fmt.Errorf("no key files found in %s", keysDir)
	}

	// Encrypt with AES-256-GCM.
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return 0, fmt.Errorf("generate salt: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return 0, fmt.Errorf("generate nonce: %w", err)
	}

	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("create GCM: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, tarBuf.Bytes(), nil)

	// Write vault file: magic + version + salt + nonce + ciphertext.
	var out bytes.Buffer
	out.WriteString(vaultMagic)
	out.WriteByte(vaultVersion)
	out.Write(salt)
	out.Write(nonce)
	// 8-byte big-endian ciphertext length so restore can sanity-check.
	if err := binary.Write(&out, binary.BigEndian, uint64(len(ciphertext))); err != nil {
		return 0, fmt.Errorf("write length: %w", err)
	}
	out.Write(ciphertext)

	if err := os.WriteFile(destPath, out.Bytes(), 0600); err != nil {
		return 0, fmt.Errorf("write vault file: %w", err)
	}
	return count, nil
}

// RestoreKeys decrypts a vault file created by BackupKeys and extracts the key
// files into destDir. Existing files are skipped unless force is true.
// Returns the number of files restored.
func RestoreKeys(vaultPath, destDir, passphrase string, force bool) (int, error) {
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		return 0, fmt.Errorf("read vault: %w", err)
	}

	// Parse header.
	headerLen := len(vaultMagic) + 1 + saltLen + nonceLen + 8
	if len(data) < headerLen {
		return 0, fmt.Errorf("vault file too small — may be corrupt")
	}

	pos := 0
	magic := string(data[pos : pos+len(vaultMagic)])
	pos += len(vaultMagic)
	if magic != vaultMagic {
		return 0, fmt.Errorf("not a valid sshelf vault file")
	}

	version := data[pos]
	pos++
	if version != vaultVersion {
		return 0, fmt.Errorf("unsupported vault version %d", version)
	}

	salt := data[pos : pos+saltLen]
	pos += saltLen
	nonce := data[pos : pos+nonceLen]
	pos += nonceLen
	ctLen := binary.BigEndian.Uint64(data[pos : pos+8])
	pos += 8

	if uint64(len(data)-pos) != ctLen {
		return 0, fmt.Errorf("vault file is corrupt (length mismatch)")
	}
	ciphertext := data[pos:]

	// Decrypt.
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("create GCM: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// GCM authentication failed — almost certainly wrong passphrase.
		return 0, fmt.Errorf("decryption failed — wrong passphrase?")
	}

	// Extract tar.gz.
	gr, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return 0, fmt.Errorf("decompress vault: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("read archive: %w", err)
		}

		destFile := filepath.Join(destDir, hdr.Name)
		if !force {
			if _, err := os.Stat(destFile); err == nil {
				// File exists and force not set — skip.
				continue
			}
		}

		fileData, err := io.ReadAll(tr)
		if err != nil {
			return count, fmt.Errorf("read %s from archive: %w", hdr.Name, err)
		}
		perm := os.FileMode(hdr.Mode)
		if perm == 0 {
			perm = 0600
		}
		if err := os.WriteFile(destFile, fileData, perm); err != nil {
			return count, fmt.Errorf("write %s: %w", hdr.Name, err)
		}
		count++
	}
	return count, nil
}

// DefaultVaultPath returns a timestamped vault filename in the sshelf config dir.
func DefaultVaultPath(configDir string) string {
	stamp := time.Now().Format("2006-01-02")
	return filepath.Join(configDir, fmt.Sprintf("keys-backup-%s.vault", stamp))
}

// deriveKey produces a 32-byte AES key from passphrase + salt using iterated
// SHA-256. Not memory-hard, but sufficient for local encrypted backups without
// requiring external dependencies.
func deriveKey(passphrase string, salt []byte) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(passphrase))
	key := h.Sum(nil)
	for i := 0; i < kdfRounds-1; i++ {
		h.Reset()
		h.Write(salt)
		h.Write(key)
		key = h.Sum(nil)
	}
	return key
}
