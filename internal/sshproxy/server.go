package sshproxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	cfgpkg "ssh-reverseproxy/internal/config"
	maptypes "ssh-reverseproxy/internal/mapping"
)

const permTargetKey = "proxy.target.json.b64"

// Run starts the proxy server and blocks.
func Run(cfg cfgpkg.ServerConfig, mapping maptypes.Mapping) error {
	hostSigner, err := loadOrGenerateHostKey(cfg.HostKeyPath)
	if err != nil {
		return fmt.Errorf("load host key: %w", err)
	}

	srvConf := &ssh.ServerConfig{
		PublicKeyCallback: func(connMeta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			tgt, ok := findTarget(mapping, key)
			if !ok {
				return nil, errors.New("unauthorized public key")
			}
			b, _ := json.Marshal(tgt)
			return &ssh.Permissions{Extensions: map[string]string{
				permTargetKey: base64.StdEncoding.EncodeToString(b),
			}}, nil
		},
		ServerVersion: cfg.ServerIdent,
	}
	srvConf.AuthLogCallback = func(connMeta ssh.ConnMetadata, method string, err error) {
		if err != nil {
			log.Printf("auth failed: user=%s from=%s method=%s err=%v", connMeta.User(), connMeta.RemoteAddr(), method, err)
		}
	}
	srvConf.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ListenAddr, err)
	}
	log.Printf("ssh-reverseproxy listening on %s", cfg.ListenAddr)

	// Optional: background mapping reload from DB if interval > 0 and DB configured
	if cfg.DBDSN != "" && cfg.DBRefreshInterval > 0 {
		log.Printf("mapping auto-reload enabled (every %s)", cfg.DBRefreshInterval)
		go func() {
			ticker := time.NewTicker(cfg.DBRefreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
				m, err := maptypes.LoadFromDB(ctx, cfg.DBDSN, cfg.DBTable)
				cancel()
				if err != nil {
					log.Printf("mapping reload failed: %v", err)
					continue
				}
				// Replace in-memory mapping atomically by swapping the slice
				mapping = m
				log.Printf("mapping reloaded: %d entries", len(mapping.Entries))
			}
		}()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				log.Printf("accept timeout: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}
		go handleConn(conn, srvConf, cfg)
	}
}

func findTarget(m maptypes.Mapping, key ssh.PublicKey) (maptypes.Target, bool) {
	fp := ssh.FingerprintSHA256(key)
	for _, e := range m.Entries {
		if e.Fingerprint != "" && e.Fingerprint == fp {
			return e.Target, true
		}
	}
	incoming := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	incomingFields := strings.Fields(incoming)
	var incomingCore string
	if len(incomingFields) >= 2 {
		incomingCore = incomingFields[0] + " " + incomingFields[1]
	} else {
		incomingCore = incoming
	}
	for _, e := range m.Entries {
		if e.PublicKey == "" {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(e.PublicKey))
		var core string
		if len(fields) >= 2 {
			core = fields[0] + " " + fields[1]
		} else {
			core = strings.TrimSpace(e.PublicKey)
		}
		if core == incomingCore {
			return e.Target, true
		}
	}
	return maptypes.Target{}, false
}

func handleConn(nc net.Conn, srvConf *ssh.ServerConfig, cfg cfgpkg.ServerConfig) {
	defer nc.Close()
	serverConn, chans, reqs, err := ssh.NewServerConn(nc, srvConf)
	if err != nil {
		if !strings.Contains(err.Error(), "unauthorized public key") {
			log.Printf("handshake error from %s: %s", nc.RemoteAddr(), describeHandshakeError(err))
		}
		return
	}
	defer serverConn.Close()

	ext := serverConn.Permissions
	if ext == nil {
		log.Printf("no permissions for %s", nc.RemoteAddr())
		return
	}
	b64 := ext.Extensions[permTargetKey]
	if b64 == "" {
		log.Printf("no target in permissions for %s", nc.RemoteAddr())
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		log.Printf("decode target: %v", err)
		return
	}
	var target maptypes.Target
	if err := json.Unmarshal(data, &target); err != nil {
		log.Printf("unmarshal target: %v", err)
		return
	}

	upstreamClient, err := dialUpstream(target, cfg)
	if err != nil {
		log.Printf("upstream dial to %s failed: %v", target.Addr, err)
		return
	}
	defer upstreamClient.Close()

	go proxyGlobalRequests(reqs, upstreamClient)

	for newCh := range chans {
		go func(ch ssh.NewChannel) {
			if err := proxyChannel(ch, upstreamClient); err != nil {
				log.Printf("channel proxy error: %v", err)
			}
		}(newCh)
	}
}

func dialUpstream(t maptypes.Target, cfg cfgpkg.ServerConfig) (*ssh.Client, error) {
	var hostKeyCallback ssh.HostKeyCallback
	if cfg.KnownHosts != "" {
		if err := ensureFile(cfg.KnownHosts, 0o644); err != nil {
			return nil, fmt.Errorf("prepare known_hosts: %w", err)
		}
		kh, err := knownhosts.New(cfg.KnownHosts)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts: %w", err)
		}
		hostKeyCallback = kh
	} else {
		log.Printf("WARNING: SSH_KNOWN_HOSTS not set; skipping upstream host key verification")
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	authMethods := []ssh.AuthMethod{}
	switch strings.ToLower(strings.TrimSpace(t.Auth.Method)) {
	case "password":
		authMethods = append(authMethods, ssh.Password(t.Auth.Password))
	case "key":
		keyBytes := []byte(t.Auth.KeyInline)
		if len(keyBytes) == 0 {
			return nil, errors.New("key auth selected but keyInline is empty (DB-only mode)")
		}
		var signer ssh.Signer
		var err error
		if t.Auth.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(t.Auth.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	case "none", "":
		// no auth methods
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", t.Auth.Method)
	}

	clientConf := &ssh.ClientConfig{
		User:            t.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.DialTimeout,
	}

	client, err := ssh.Dial("tcp", t.Addr, clientConf)
	if err == nil {
		return client, nil
	}
	if cfg.KnownHosts != "" && cfg.AcceptUnknownUpstream && strings.Contains(err.Error(), "knownhosts: key is unknown") {
		if ferr := fetchAndAppendHostKey(t.Addr, cfg.KnownHosts, cfg.DialTimeout); ferr != nil {
			return nil, fmt.Errorf("upstream host key unknown and could not fetch: %w", ferr)
		}
		kh, khErr := knownhosts.New(cfg.KnownHosts)
		if khErr != nil {
			return nil, fmt.Errorf("reload known_hosts: %w", khErr)
		}
		clientConf.HostKeyCallback = kh
		return ssh.Dial("tcp", t.Addr, clientConf)
	}
	return nil, err
}

func proxyGlobalRequests(clientReqs <-chan *ssh.Request, upstream *ssh.Client) {
	for req := range clientReqs {
		ok := false
		var resp []byte
		var err error
		if req.WantReply {
			ok, resp, err = upstream.SendRequest(req.Type, true, req.Payload)
		} else {
			_, _, err = upstream.SendRequest(req.Type, false, req.Payload)
		}
		if err != nil {
			log.Printf("global request forward error: %v", err)
		}
		if req.WantReply {
			_ = req.Reply(ok, resp)
		}
	}
}

func proxyChannel(newCh ssh.NewChannel, upstream *ssh.Client) error {
	chType := newCh.ChannelType()
	extra := newCh.ExtraData()

	upstreamCh, upstreamReqs, err := upstream.OpenChannel(chType, extra)
	if err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, fmt.Sprintf("upstream open failed: %v", err))
		return err
	}
	defer upstreamCh.Close()

	localCh, localReqs, err := newCh.Accept()
	if err != nil {
		upstreamCh.Close()
		return err
	}
	defer localCh.Close()

	go forwardRequests(localReqs, upstreamCh)
	go forwardRequests(upstreamReqs, localCh)

	errc := make(chan error, 2)
	go func() {
		_, e := io.Copy(upstreamCh, localCh)
		_ = upstreamCh.CloseWrite()
		errc <- e
	}()
	go func() {
		_, e := io.Copy(localCh, upstreamCh)
		_ = localCh.CloseWrite()
		errc <- e
	}()

	<-errc
	<-errc
	return nil
}

func forwardRequests(in <-chan *ssh.Request, out ssh.Channel) {
	for req := range in {
		ok, err := out.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			log.Printf("channel request forward error: %v", err)
		}
		if req.WantReply {
			_ = req.Reply(ok, nil)
		}
	}
}

func ensureFile(path string, perm os.FileMode) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, perm)
			if err != nil {
				return err
			}
			return f.Close()
		}
		return err
	}
	return nil
}

func describeHandshakeError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "connection reset"):
		return s + " (peer closed the connection during handshake)"
	case strings.Contains(s, "no common algorithms"):
		return s + " (mismatch in key/cipher/MAC algorithms)"
	case strings.Contains(s, "unauthorized public key"):
		return s + " (client key not present in mapping)"
	case strings.Contains(s, "unexpected message type"):
		return s + " (client speaking non-SSH or incompatible SSH version)"
	default:
		return s
	}
}

func fetchAndAppendHostKey(addr string, knownHostsPath string, timeout time.Duration) error {
	var captured ssh.PublicKey
	cfg := &ssh.ClientConfig{
		User: "unknown",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			captured = key
			return nil
		},
		Timeout: timeout,
	}
	_, _ = ssh.Dial("tcp", addr, cfg)
	if captured == nil {
		return fmt.Errorf("could not capture host key from %s", addr)
	}
	hostPattern := normalizeKnownHostsHost(addr)
	line := knownhosts.Line([]string{hostPattern}, captured)
	if err := ensureFile(knownHostsPath, 0o644); err != nil {
		return err
	}
	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	log.Printf("added upstream host key for %s to %s", hostPattern, knownHostsPath)
	return nil
}

func normalizeKnownHostsHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if port == "22" {
		return host
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if dir := filepath.Dir(path); dir != "." && dir != "" {
					if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
						return nil, fmt.Errorf("create host key dir %s: %w", dir, mkErr)
					}
				}
				_, priv, gErr := ed25519.GenerateKey(rand.Reader)
				if gErr != nil {
					return nil, gErr
				}
				der, mErr := x509.MarshalPKCS8PrivateKey(priv)
				if mErr != nil {
					return nil, fmt.Errorf("marshal host key: %w", mErr)
				}
				pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
				pemBytes := pem.EncodeToMemory(pemBlock)
				if wErr := os.WriteFile(path, pemBytes, 0o600); wErr != nil {
					return nil, fmt.Errorf("write host key %s: %w", path, wErr)
				}
				log.Printf("generated new ed25519 host key at %s", path)
				signer, sErr := ssh.NewSignerFromKey(priv)
				if sErr != nil {
					return nil, sErr
				}
				return signer, nil
			}
			return nil, fmt.Errorf("read host key %s: %w", path, err)
		}
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		return signer, nil
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	log.Printf("using ephemeral in-memory host key (set SSH_HOST_KEY_PATH to use a persistent key)")
	return signer, nil
}
