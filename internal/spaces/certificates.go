// Copyright 2025 Upbound Inc.
// All rights reserved

package spaces

import (
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// mergeCACertificates combines custom CA bundle with existing CA data.
func mergeCACertificates(customCAPath string, existingCAData []byte) ([]byte, error) {
	customCAData, err := loadCustomCABundle(customCAPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load custom CA bundle")
	}
	customCAData = append(customCAData, existingCAData...)

	return customCAData, nil
}

// loadCustomCABundle loads and validates custom CA certificates from file.
func loadCustomCABundle(caPath string) ([]byte, error) {
	caData, err := os.ReadFile(caPath) // #nosec G304 -- caPath is validated/controlled input
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read CA bundle file %s", caPath)
	}
	if err := validateCACertificates(caData); err != nil {
		return nil, errors.Wrap(err, "invalid CA certificates in bundle")
	}

	return caData, nil
}

// validateCACertificates ensures the provided data contains valid PEM-encoded certificates.
func validateCACertificates(caData []byte) error {
	certificateCount := 0

	for block, rest := pem.Decode(caData); block != nil; block, rest = pem.Decode(rest) {
		if block.Type == "CERTIFICATE" {
			_, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return errors.Wrap(err, "invalid certificate found")
			}
			certificateCount++
		}
	}

	if certificateCount == 0 {
		return errors.New("no valid certificates found in CA bundle")
	}

	return nil
}

func ensureCertificateAuthorityData(tlsCert string) error {
	block, _ := pem.Decode([]byte(tlsCert))
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.New("CA string does not contain PEM certificate data")
	}

	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "CA cannot be parsed to x509 certificate")
	}
	return nil
}
