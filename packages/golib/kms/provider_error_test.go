package kms

import (
	"errors"
	"testing"
)

// failingProvider lets tests inject specific failures into the
// EnvelopeEncryptor without standing up a fake KMS.
type failingProvider struct {
	genErr error
	decErr error
}

func (f *failingProvider) GenerateDEK() ([]byte, []byte, error) {
	if f.genErr != nil {
		return nil, nil, f.genErr
	}
	// Return a valid 32-byte key + a tiny "encrypted" placeholder so the
	// envelope packing succeeds.
	return make([]byte, 32), []byte("enc"), nil
}

func (f *failingProvider) DecryptDEK(_ []byte) ([]byte, error) {
	if f.decErr != nil {
		return nil, f.decErr
	}
	return make([]byte, 32), nil
}

func (f *failingProvider) KeyID() string { return "fail-kid" }

// TestEncrypt_PropagatesProviderError pins the GenerateDEK error branch
// in EnvelopeEncryptor.Encrypt — failures from the KMS provider must
// surface to the caller, not be swallowed.
func TestEncrypt_PropagatesProviderError(t *testing.T) {
	want := errors.New("kms-down")
	e := NewEnvelopeEncryptor(&failingProvider{genErr: want})
	if _, err := e.Encrypt([]byte("x")); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestDecrypt_PropagatesProviderError pins the DecryptDEK error branch
// in EnvelopeEncryptor.Decrypt. We construct a real envelope with a real
// LocalProvider, then swap to a failing provider for decryption to force
// the cache miss → DecryptDEK path.
func TestDecrypt_PropagatesProviderError(t *testing.T) {
	// Build a valid envelope with a working provider.
	p := newProvider(t)
	enc := NewEnvelopeEncryptor(p)
	ct, err := enc.EncryptString("hello")
	if err != nil {
		t.Fatal(err)
	}

	// Now decrypt with a fresh encryptor whose provider always errors —
	// the dek cache is empty so it must call DecryptDEK and surface the error.
	want := errors.New("hsm-unreachable")
	bad := NewEnvelopeEncryptor(&failingProvider{decErr: want})
	if _, err := bad.Decrypt(ct); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
