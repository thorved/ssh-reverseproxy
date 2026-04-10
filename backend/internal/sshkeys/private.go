package sshkeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func GeneratePrivateKeyPEM() (string, string, string, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", fmt.Errorf("generate private key: %w", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal private key: %w", err)
	}

	passphrase, err := generatePassphrase()
	if err != nil {
		return "", "", "", err
	}

	block, err := x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY", der, []byte(passphrase), x509.PEMCipherAES256)
	if err != nil {
		return "", "", "", fmt.Errorf("encrypt private key: %w", err)
	}

	pemBytes := pem.EncodeToMemory(block)
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("create signer: %w", err)
	}

	return strings.TrimSpace(string(pemBytes)), PublicKeyFromSigner(signer), passphrase, nil
}

func PublicKeyFromPrivateKey(input, passphrase string) (string, error) {
	signer, err := SignerFromPrivateKey(input, passphrase)
	if err != nil {
		return "", err
	}
	return PublicKeyFromSigner(signer), nil
}

func SignerFromPrivateKey(input, passphrase string) (ssh.Signer, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("ssh private key is required")
	}

	decoded, _ := pem.Decode([]byte(trimmed))
	if decoded != nil && x509.IsEncryptedPEMBlock(decoded) {
		if strings.TrimSpace(passphrase) == "" {
			return nil, fmt.Errorf("ssh private key is passphrase protected")
		}
		der, err := x509.DecryptPEMBlock(decoded, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("decrypt ssh private key: %w", err)
		}
		privateKey, err := x509.ParsePKCS8PrivateKey(der)
		if err != nil {
			return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
		}
		signer, err := ssh.NewSignerFromKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("create signer from private key: %w", err)
		}
		return signer, nil
	}

	var (
		signer ssh.Signer
		err    error
	)
	if strings.TrimSpace(passphrase) != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(trimmed), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(trimmed))
	}
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key: %w", err)
	}

	return signer, nil
}

func PublicKeyFromSigner(signer ssh.Signer) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}

func generatePassphrase() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate passphrase: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
