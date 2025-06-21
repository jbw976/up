// Copyright 2025 Upbound Inc.
// All rights reserved

package spaces

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMergeCACertificates(t *testing.T) {
	tempDir := t.TempDir()
	validCACert := generateTestCACert(t)
	validCAPath := filepath.Join(tempDir, "valid_ca.pem")

	if err := os.WriteFile(validCAPath, validCACert, 0o644); err != nil {
		t.Fatalf("Failed to write valid CA file: %v", err)
	}

	invalidCAPath := filepath.Join(tempDir, "invalid_ca.pem")
	if err := os.WriteFile(invalidCAPath, []byte("invalid pem data"), 0o644); err != nil {
		t.Fatalf("Failed to write invalid CA file: %v", err)
	}

	nonExistentPath := filepath.Join(tempDir, "nonexistent.pem")

	existingCAData := []byte("existing-ca-data")

	tests := map[string]struct {
		customCAPath   string
		existingCAData []byte
		want           []byte
		wantError      bool
	}{
		"ValidCACertificateMerge": {
			customCAPath:   validCAPath,
			existingCAData: existingCAData,
			want:           append(validCACert, existingCAData...),
			wantError:      false,
		},
		"InvalidCACertificate": {
			customCAPath:   invalidCAPath,
			existingCAData: existingCAData,
			want:           nil,
			wantError:      true,
		},
		"NonExistentCAFile": {
			customCAPath:   nonExistentPath,
			existingCAData: existingCAData,
			want:           nil,
			wantError:      true,
		},
		"EmptyExistingData": {
			customCAPath:   validCAPath,
			existingCAData: []byte{},
			want:           validCACert,
			wantError:      false,
		},
		"NilExistingData": {
			customCAPath:   validCAPath,
			existingCAData: nil,
			want:           validCACert,
			wantError:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := mergeCACertificates(tc.customCAPath, tc.existingCAData)

			if tc.wantError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("mergeCACertificates() returned an error: %v", err)
			}

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("mergeCACertificates() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestLoadCustomCABundle(t *testing.T) {
	tempDir := t.TempDir()

	validCACert := generateTestCACert(t)
	validCAPath := filepath.Join(tempDir, "valid_ca.pem")

	if err := os.WriteFile(validCAPath, validCACert, 0o644); err != nil {
		t.Fatalf("Failed to write valid CA file: %v", err)
	}

	invalidCAPath := filepath.Join(tempDir, "invalid_ca.pem")
	if err := os.WriteFile(invalidCAPath, []byte("invalid pem data"), 0o644); err != nil {
		t.Fatalf("Failed to write invalid CA file: %v", err)
	}

	emptyCAPath := filepath.Join(tempDir, "empty_ca.pem")

	if err := os.WriteFile(emptyCAPath, []byte{}, 0o644); err != nil {
		t.Fatalf("Failed to write empty CA file: %v", err)
	}

	nonExistentPath := filepath.Join(tempDir, "nonexistent.pem")

	tests := map[string]struct {
		caPath    string
		want      []byte
		wantError bool
	}{
		"ValidCACertificate": {
			caPath:    validCAPath,
			want:      validCACert,
			wantError: false,
		},
		"InvalidCACertificate": {
			caPath:    invalidCAPath,
			want:      nil,
			wantError: true,
		},
		"EmptyFile": {
			caPath:    emptyCAPath,
			want:      nil,
			wantError: true,
		},
		"NonExistentFile": {
			caPath:    nonExistentPath,
			want:      nil,
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := loadCustomCABundle(tc.caPath)

			if tc.wantError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("loadCustomCABundle() returned an error: %v", err)
			}

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("loadCustomCABundle() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestValidateCACertificates(t *testing.T) {
	validCACert := generateTestCACert(t)
	multipleCACerts := generateMultipleTestCACerts(t, 2)

	// Create invalid certificate PEM (wrong block type)
	invalidCertPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("invalid-cert-data"),
	}
	invalidCertData := pem.EncodeToMemory(invalidCertPEM)

	// Create malformed certificate data
	malformedCertPEM := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("malformed-cert-data"),
	}
	malformedCertData := pem.EncodeToMemory(malformedCertPEM)

	tests := map[string]struct {
		caData    []byte
		wantError bool
	}{
		"ValidSingleCertificate": {
			caData:    validCACert,
			wantError: false,
		},
		"ValidMultipleCertificates": {
			caData:    multipleCACerts,
			wantError: false,
		},
		"NoPEMData": {
			caData:    []byte("not a pem file"),
			wantError: true,
		},
		"EmptyData": {
			caData:    []byte{},
			wantError: true,
		},
		"NilData": {
			caData:    nil,
			wantError: true,
		},
		"InvalidCertificateType": {
			caData:    invalidCertData,
			wantError: true,
		},
		"MalformedCertificate": {
			caData:    malformedCertData,
			wantError: true,
		},
		"PEMWithoutCertificates": {
			caData: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: []byte("some-key-data"),
			}),
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateCACertificates(tc.caData)

			if tc.wantError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("validateCACertificates() returned an error: %v", err)
			}
		})
	}
}

func generateTestCACert(t *testing.T) []byte {
	t.Helper()

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode as PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM
}

func TestEnsureCertificateAuthorityData(t *testing.T) {
	validCACert := generateTestCACert(t)
	validCACertString := string(validCACert)

	// Create invalid certificate PEM (wrong block type)
	invalidCertPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("invalid-cert-data"),
	}
	invalidCertString := string(pem.EncodeToMemory(invalidCertPEM))

	// Create malformed certificate data
	malformedCertPEM := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("malformed-cert-data"),
	}
	malformedCertString := string(pem.EncodeToMemory(malformedCertPEM))

	tests := map[string]struct {
		tlsCert   string
		wantError bool
	}{
		"ValidCertificate": {
			tlsCert:   validCACertString,
			wantError: false,
		},
		"EmptyString": {
			tlsCert:   "",
			wantError: true,
		},
		"NotPEMData": {
			tlsCert:   "not a pem certificate",
			wantError: true,
		},
		"WrongPEMBlockType": {
			tlsCert:   invalidCertString,
			wantError: true,
		},
		"MalformedCertificate": {
			tlsCert:   malformedCertString,
			wantError: true,
		},
		"OnlyWhitespace": {
			tlsCert:   "   \n\t  ",
			wantError: true,
		},
		"PartialPEMData": {
			tlsCert:   "-----BEGIN CERTIFICATE-----\ninvalid data",
			wantError: true,
		},
		"MultiplePEMBlocks": {
			tlsCert:   validCACertString + invalidCertString,
			wantError: false, // Should validate the first valid certificate
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ensureCertificateAuthorityData(tc.tlsCert)

			if tc.wantError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ensureCertificateAuthorityData() returned an error: %v", err)
			}
		})
	}
}

func generateMultipleTestCACerts(t *testing.T, count int) []byte {
	t.Helper()

	var allCerts []byte
	for range count {
		cert := generateTestCACert(t)
		allCerts = append(allCerts, cert...)
	}
	return allCerts
}
