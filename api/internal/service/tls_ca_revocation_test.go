package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var tlsCAEvaluationTime = time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC)

// Exercise crypto/x509 itself: upgrading the Go image must reject the revoked
// eMudhra keys even when an application explicitly trusts their issuer.
func TestEmudhraCompromisedCARejected(t *testing.T) {
	for _, name := range []string{
		"ov-g3c-emudhra.we-have-a-problem.com.pem",
		"ov-g3c-aaa-emudhra.we-have-a-problem.com.pem",
	} {
		t.Run(name, func(t *testing.T) {
			chain := readTLSCAChain(t, filepath.Join("testdata", "emudhra", name))
			err := verifyTLSCAChain(chain, strings.TrimSuffix(name, ".pem"))
			if err == nil {
				t.Fatal("crypto/x509 accepted a chain containing a compromised eMudhra CA key")
			}
			var invalid x509.CertificateInvalidError
			if !errors.As(err, &invalid) {
				t.Fatalf("expected CertificateInvalidError for the distrusted chain, got %T: %v", err, err)
			}
			// Go reports the SPKI block as NoValidChains without extra details.
			// Checking its message keeps this test compilable on the old toolchain
			// so the RED run demonstrates acceptance, not a missing identifier.
			if invalid.Error() != "x509: no valid chains built" || invalid.Detail != "" || !invalid.Cert.Equal(chain[0]) {
				t.Fatalf("expected NoValidChains from the SPKI block, got reason %v: %v", invalid.Reason, err)
			}
		})
	}
}

func TestTLSCAValidLocalChainAccepted(t *testing.T) {
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Sifto TLS regression CA"},
		NotBefore:             tlsCAEvaluationTime.Add(-time.Hour),
		NotAfter:              tlsCAEvaluationTime.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		DNSNames:     []string{"valid.sifto.test"},
		NotBefore:    ca.NotBefore,
		NotAfter:     ca.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, leafPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err = x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyTLSCAChain([]*x509.Certificate{leaf, ca}, "valid.sifto.test"); err != nil {
		t.Fatalf("valid local CA and server certificate should remain trusted: %v", err)
	}
}

func readTLSCAChain(t *testing.T, path string) []*x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var chain []*x509.Certificate
	for len(strings.TrimSpace(string(data))) != 0 {
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			t.Fatalf("expected only PEM certificates in %s", path)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse certificate in %s: %v", path, err)
		}
		chain = append(chain, cert)
		data = rest
	}
	if len(chain) < 2 {
		t.Fatalf("expected leaf and issuer certificates in %s", path)
	}
	return chain
}

func verifyTLSCAChain(chain []*x509.Certificate, hostname string) error {
	roots := x509.NewCertPool()
	// The public fixtures end with an issuer, which need not be self-signed.
	// Explicit trust avoids depending on host root stores or network fetching.
	roots.AddCert(chain[len(chain)-1])
	intermediates := x509.NewCertPool()
	for _, cert := range chain[1 : len(chain)-1] {
		intermediates.AddCert(cert)
	}
	_, err := chain[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       hostname,
		CurrentTime:   tlsCAEvaluationTime,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	return err
}
