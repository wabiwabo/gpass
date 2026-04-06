// Package pem provides PEM encoding/decoding utilities for
// certificates and keys. Validates PEM structure, extracts
// certificate metadata, and supports multiple PEM blocks.
package pem

import (
	"encoding/pem"
	"fmt"
	"strings"
)

// Block represents a parsed PEM block with metadata.
type Block struct {
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers,omitempty"`
	Bytes   []byte            `json:"-"`
}

// Parse extracts the first PEM block from data.
func Parse(data []byte) (*Block, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("pem: no PEM data found")
	}
	return &Block{
		Type:    block.Type,
		Headers: block.Headers,
		Bytes:   block.Bytes,
	}, nil
}

// ParseAll extracts all PEM blocks from data.
func ParseAll(data []byte) ([]*Block, error) {
	var blocks []*Block
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		blocks = append(blocks, &Block{
			Type:    block.Type,
			Headers: block.Headers,
			Bytes:   block.Bytes,
		})
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("pem: no PEM data found")
	}
	return blocks, nil
}

// Encode encodes a block to PEM format.
func Encode(blockType string, data []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  blockType,
		Bytes: data,
	})
}

// EncodeWithHeaders encodes with additional headers.
func EncodeWithHeaders(blockType string, data []byte, headers map[string]string) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    blockType,
		Headers: headers,
		Bytes:   data,
	})
}

// IsPEM checks if data looks like PEM-encoded content.
func IsPEM(data []byte) bool {
	return strings.Contains(string(data), "-----BEGIN ")
}

// TypeOf returns the PEM block type without full parsing.
func TypeOf(data []byte) string {
	block, _ := pem.Decode(data)
	if block == nil {
		return ""
	}
	return block.Type
}

// Common PEM block types.
const (
	TypeCertificate   = "CERTIFICATE"
	TypeRSAPrivateKey = "RSA PRIVATE KEY"
	TypeECPrivateKey  = "EC PRIVATE KEY"
	TypePrivateKey    = "PRIVATE KEY"
	TypePublicKey     = "PUBLIC KEY"
	TypeCSR           = "CERTIFICATE REQUEST"
)

// IsCertificate checks if the PEM data is a certificate.
func IsCertificate(data []byte) bool {
	return TypeOf(data) == TypeCertificate
}

// IsPrivateKey checks if the PEM data is any type of private key.
func IsPrivateKey(data []byte) bool {
	t := TypeOf(data)
	return t == TypePrivateKey || t == TypeRSAPrivateKey || t == TypeECPrivateKey
}

// Count returns the number of PEM blocks in the data.
func Count(data []byte) int {
	count := 0
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		count++
	}
	return count
}
