package bundle

import (
	"bytes"
	"fmt"
)

// SupportedLicenses lists the SPDX identifiers this repository can bundle.
// Redistribution of course content is only allowed for permissive licenses
// that require preserving the license text and copyright notice.
var SupportedLicenses = map[string]bool{
	"MIT": true,
}

var (
	mitGrant  = []byte("Permission is hereby granted, free of charge")
	mitNotice = []byte("The above copyright notice and this permission notice")
)

// ValidateLicense checks that the pinned license is supported and that the
// upstream LICENSE file actually contains the expected license text.
func ValidateLicense(pin Pin, text []byte) error {
	if !SupportedLicenses[pin.License] {
		return fmt.Errorf("%s: unsupported license %q", pin.Slug, pin.License)
	}
	if !bytes.Contains(text, mitGrant) || !bytes.Contains(text, mitNotice) {
		return fmt.Errorf("%s: LICENSE does not look like the MIT license", pin.Slug)
	}
	return nil
}
