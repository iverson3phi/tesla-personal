package keys

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

func TestGenerateProducesLoadableP256Pair(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private-key.pem")
	pub := filepath.Join(dir, "public-key.pem")

	if err := Generate(priv, pub); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Private key must be loadable by the SDK (proves curve + format).
	if _, err := protocol.LoadPrivateKey(priv); err != nil {
		t.Fatalf("SDK LoadPrivateKey: %v", err)
	}
	if info, _ := os.Stat(priv); info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 600", info.Mode().Perm())
	}

	// Public key must be a PKIX "PUBLIC KEY" PEM.
	b, err := os.ReadFile(pub)
	if err != nil {
		t.Fatalf("read pub: %v", err)
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatalf("public PEM type = %v, want PUBLIC KEY", block)
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		t.Fatalf("ParsePKIXPublicKey: %v", err)
	}
	if info, _ := os.Stat(pub); info.Mode().Perm() != 0o644 {
		t.Fatalf("public key mode = %o, want 644", info.Mode().Perm())
	}
}
