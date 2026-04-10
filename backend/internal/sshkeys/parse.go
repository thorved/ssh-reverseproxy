package sshkeys

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type ParsedKey struct {
	PublicKey   string
	Fingerprint string
	Algorithm   string
	Comment     string
}

func ParseAuthorizedKey(input string) (*ParsedKey, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("ssh public key is required")
	}

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parse ssh public key: %w", err)
	}

	return &ParsedKey{
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))),
		Fingerprint: ssh.FingerprintSHA256(publicKey),
		Algorithm:   publicKey.Type(),
		Comment:     strings.TrimSpace(comment),
	}, nil
}
