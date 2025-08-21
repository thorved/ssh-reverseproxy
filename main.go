package main

import (
	"bufio"
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

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Mapping holds a list of entries mapping client public keys to upstream targets
// Example file structure is in mapping.example.json
// {"entries": [{"publicKey":"ssh-ed25519 AAAA... comment","target":{...}}]}
// You can also match by fingerprint: {"fingerprint":"SHA256:..."}
// Fingerprint uses ssh.FingerprintSHA256 format.

type Mapping struct {
	Entries []MappingEntry `json:"entries"`
}

type MappingEntry struct {
	PublicKey   string         `json:"publicKey,omitempty"`
	Fingerprint string         `json:"fingerprint,omitempty"`
	Target      UpstreamTarget `json:"target"`
}

type UpstreamTarget struct {
	Addr string `json:"addr"` // host:port
	User string `json:"user"`
	Auth struct {
		Method     string `json:"method"` // none | password | key
		Password   string `json:"password,omitempty"`
		KeyPath    string `json:"keyPath,omitempty"`
		Passphrase string `json:"passphrase,omitempty"`
	} `json:"auth"`
}

const permTargetKey = "proxy.target.json.b64"

func main() {
	// Load environment variables from a local .env file if present
	// (ignored if the file does not exist)
	_ = godotenv.Load()

	cfg := mustLoadConfig()

	mapping, err := loadMapping(cfg.mappingPath)
	if err != nil {
		log.Fatalf("failed to load mapping: %v", err)
	}

	hostSigner, err := loadOrGenerateHostKey(cfg.hostKeyPath)
	if err != nil {
		log.Fatalf("failed to load host key: %v", err)
	}

	srvConf := &ssh.ServerConfig{
		PublicKeyCallback: func(connMeta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			// Map key to target
			tgt, ok := findTarget(mapping, key)
			if !ok {
				return nil, errors.New("unauthorized public key")
			}
			b, _ := json.Marshal(tgt)
			return &ssh.Permissions{Extensions: map[string]string{
				permTargetKey: base64.StdEncoding.EncodeToString(b),
			}}, nil
		},
		ServerVersion: cfg.serverIdent,
	}
	srvConf.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", cfg.listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.listenAddr, err)
	}
	log.Printf("ssh-reverseproxy listening on %s", cfg.listenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("accept temp err: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log.Printf("accept error: %v", err)
			return
		}
		go handleConn(conn, srvConf, cfg)
	}
}

type serverConfig struct {
	listenAddr            string
	hostKeyPath           string
	mappingPath           string
	knownHosts            string
	serverIdent           string
	dialTimeout           time.Duration
	acceptUnknownUpstream bool
}

func mustLoadConfig() serverConfig {
	addr := getenvDefault("SSH_LISTEN_ADDR", "")
	port := getenvDefault("SSH_PORT", "")
	if addr == "" {
		if port == "" {
			addr = ":2222"
		} else {
			addr = ":" + port
		}
	}

	return serverConfig{
		listenAddr:            addr,
		hostKeyPath:           os.Getenv("SSH_HOST_KEY_PATH"),
		mappingPath:           getenvDefault("SSH_MAPPING_FILE", "mapping.json"),
		knownHosts:            os.Getenv("SSH_KNOWN_HOSTS"),
		serverIdent:           getenvDefault("SSH_SERVER_IDENT", "SSH-2.0-ssh-reverseproxy"),
		dialTimeout:           15 * time.Second,
		acceptUnknownUpstream: getenvBool("SSH_ACCEPT_UNKNOWN_UPSTREAM", false),
	}
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func loadMapping(path string) (Mapping, error) {
	f, err := os.Open(path)
	if err != nil {
		return Mapping{}, err
	}
	defer f.Close()
	var m Mapping
	dec := json.NewDecoder(bufio.NewReader(f))
	if err := dec.Decode(&m); err != nil {
		return Mapping{}, err
	}
	for i := range m.Entries {
		m.Entries[i].PublicKey = strings.TrimSpace(m.Entries[i].PublicKey)
		m.Entries[i].Fingerprint = strings.TrimSpace(m.Entries[i].Fingerprint)
	}
	return m, nil
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Ensure directory exists
				if dir := filepath.Dir(path); dir != "." && dir != "" {
					if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
						return nil, fmt.Errorf("create host key dir %s: %w", dir, mkErr)
					}
				}
				// Generate a new ed25519 key and persist it in PKCS#8 PEM format
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
	// generate ephemeral ed25519 key
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

func findTarget(m Mapping, key ssh.PublicKey) (UpstreamTarget, bool) {
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
	return UpstreamTarget{}, false
}

func handleConn(nc net.Conn, srvConf *ssh.ServerConfig, cfg serverConfig) {
	defer nc.Close()
	serverConn, chans, reqs, err := ssh.NewServerConn(nc, srvConf)
	if err != nil {
		if !strings.Contains(err.Error(), "unauthorized public key") {
			log.Printf("handshake error from %s: %v", nc.RemoteAddr(), err)
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
	var target UpstreamTarget
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

func dialUpstream(t UpstreamTarget, cfg serverConfig) (*ssh.Client, error) {
	var hostKeyCallback ssh.HostKeyCallback
	if cfg.knownHosts != "" {
		// Ensure known_hosts file exists if a path is provided
		if err := ensureFile(cfg.knownHosts, 0o644); err != nil {
			return nil, fmt.Errorf("prepare known_hosts: %w", err)
		}
		kh, err := knownhosts.New(cfg.knownHosts)
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
		keyPath := t.Auth.KeyPath
		if keyPath == "" {
			return nil, errors.New("key auth selected but keyPath empty")
		}
		abs := keyPath
		if !filepath.IsAbs(abs) {
			if wd, err := os.Getwd(); err == nil {
				abs = filepath.Join(wd, keyPath)
			}
		}
		keyBytes, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("read key %s: %w", abs, err)
		}
		var signer ssh.Signer
		if t.Auth.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(t.Auth.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("parse key %s: %w", abs, err)
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
		Timeout:         cfg.dialTimeout,
	}

	client, err := ssh.Dial("tcp", t.Addr, clientConf)
	if err == nil {
		return client, nil
	}
	// If host key is unknown and policy allows, learn it and retry once
	if cfg.knownHosts != "" && cfg.acceptUnknownUpstream && strings.Contains(err.Error(), "knownhosts: key is unknown") {
		if ferr := fetchAndAppendHostKey(t.Addr, cfg.knownHosts, cfg.dialTimeout); ferr != nil {
			return nil, fmt.Errorf("upstream host key unknown and could not fetch: %w", ferr)
		}
		// Rebuild the callback after updating the file
		kh, khErr := knownhosts.New(cfg.knownHosts)
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

	// Wait for both directions to complete to avoid half-closed hangs
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

// ensureFile creates the file and its parent directory if they do not exist.
// If the file exists, it's left untouched.
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

// fetchAndAppendHostKey dials the target with a permissive callback to capture
// its host key, appends it to known_hosts, and returns.
func fetchAndAppendHostKey(addr string, knownHostsPath string, timeout time.Duration) error {
	var captured ssh.PublicKey
	cfg := &ssh.ClientConfig{
		User: "unknown", // not used; auth will fail after handshake which is fine
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			captured = key
			return nil // accept for the purpose of capture
		},
		Timeout: timeout,
	}
	// Intentionally no auth methods
	// Perform a dial to trigger key exchange and callback
	_, _ = ssh.Dial("tcp", addr, cfg) // error is expected due to auth, ignore
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

// normalizeKnownHostsHost converts host:port to known_hosts pattern format.
// For non-22 port it returns "[host]:port", otherwise just host.
func normalizeKnownHostsHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr // already normalized or missing port
	}
	if port == "22" {
		return host
	}
	// IPv6 already contains ':', known_hosts requires [host]:port
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}
