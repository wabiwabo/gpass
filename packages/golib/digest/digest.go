package digest

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// SHA256 computes SHA-256 hash of data and returns hex-encoded string.
func SHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA384 computes SHA-384 hash of data.
func SHA384(data []byte) string {
	h := sha512.Sum384(data)
	return hex.EncodeToString(h[:])
}

// SHA512Hash computes SHA-512 hash of data.
func SHA512Hash(data []byte) string {
	h := sha512.Sum512(data)
	return hex.EncodeToString(h[:])
}

// SHA256String computes SHA-256 hash of a string.
func SHA256String(s string) string {
	return SHA256([]byte(s))
}

// SHA256Reader computes SHA-256 hash from an io.Reader.
func SHA256Reader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("digest: read error: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SHA256File computes SHA-256 hash of a file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("digest: open file: %w", err)
	}
	defer f.Close()
	return SHA256Reader(f)
}

// CRC32 computes CRC-32 checksum using IEEE polynomial.
func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// CRC32Hex computes CRC-32 as hex string.
func CRC32Hex(data []byte) string {
	return fmt.Sprintf("%08x", CRC32(data))
}

// Verify checks if data matches the expected SHA-256 hash.
func Verify(data []byte, expectedSHA256 string) bool {
	return SHA256(data) == expectedSHA256
}

// VerifyFile checks if a file matches the expected SHA-256 hash.
func VerifyFile(path, expectedSHA256 string) (bool, error) {
	actual, err := SHA256File(path)
	if err != nil {
		return false, err
	}
	return actual == expectedSHA256, nil
}

// MultiHash computes multiple hash algorithms at once for efficiency.
type MultiHash struct {
	SHA256 string `json:"sha256"`
	SHA384 string `json:"sha384"`
	SHA512 string `json:"sha512"`
	CRC32  string `json:"crc32"`
	Size   int64  `json:"size"`
}

// ComputeMulti computes all supported hashes of data in one pass.
func ComputeMulti(data []byte) MultiHash {
	return MultiHash{
		SHA256: SHA256(data),
		SHA384: SHA384(data),
		SHA512: SHA512Hash(data),
		CRC32:  CRC32Hex(data),
		Size:   int64(len(data)),
	}
}

// ComputeMultiReader computes all hashes from a reader in one pass.
func ComputeMultiReader(r io.Reader) (MultiHash, error) {
	h256 := sha256.New()
	h384 := sha512.New384()
	h512 := sha512.New()
	hcrc := crc32.NewIEEE()

	w := io.MultiWriter(h256, h384, h512, hcrc)
	n, err := io.Copy(w, r)
	if err != nil {
		return MultiHash{}, fmt.Errorf("digest: read error: %w", err)
	}

	return MultiHash{
		SHA256: hex.EncodeToString(h256.Sum(nil)),
		SHA384: hex.EncodeToString(h384.Sum(nil)),
		SHA512: hex.EncodeToString(h512.Sum(nil)),
		CRC32:  fmt.Sprintf("%08x", hcrc.Sum32()),
		Size:   n,
	}, nil
}
