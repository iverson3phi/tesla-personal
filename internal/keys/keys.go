// Package keys generates the P-256 command-signing key pair.
package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Generate writes a P-256 private key (SEC1 PEM, 0600) to privPath and the
// matching PKIX public key (PUBLIC KEY PEM, 0644) to pubPath.
func Generate(privPath, pubPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal public: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	return nil
}
