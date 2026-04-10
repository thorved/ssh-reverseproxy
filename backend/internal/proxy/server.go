package proxy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"github.com/thorved/ssh-reverseproxy/backend/internal/sshkeys"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gorm.io/gorm"
)

const userIDPermission = "proxy.user_id"

type Server struct {
	cfg      config.Config
	db       *gorm.DB
	listener atomic.Pointer[net.Listener]
	signer   ssh.Signer
}

type upstreamTarget struct {
	Addr       string
	User       string
	AuthMethod string
	Password   string
	KeyInline  string
	Passphrase string
}

func NewServer(cfg config.Config, db *gorm.DB) (*Server, error) {
	signer, err := loadOrGenerateHostKey(cfg.SSHHostKeyPath)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, db: db, signer: signer}, nil
}

func (s *Server) Run() error {
	serverConfig := &ssh.ServerConfig{
		ServerVersion: s.cfg.SSHServerIdent,
		PublicKeyCallback: func(connMeta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			userID, err := s.lookupUserIDByKey(key)
			if err != nil {
				return nil, err
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					userIDPermission: strconv.FormatUint(uint64(userID), 10),
				},
			}, nil
		},
	}
	serverConfig.AddHostKey(s.signer)

	ln, err := net.Listen("tcp", s.cfg.SSHListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.SSHListenAddr, err)
	}
	s.listener.Store(&ln)
	log.Printf("ssh proxy listening on %s", s.cfg.SSHListenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handleConn(conn, serverConfig)
	}
}

func (s *Server) Shutdown() error {
	ln := s.listener.Load()
	if ln == nil {
		return nil
	}
	return (*ln).Close()
}

func (s *Server) lookupUserIDByKey(key ssh.PublicKey) (uint, error) {
	fingerprint := ssh.FingerprintSHA256(key)

	var record models.SSHKey
	err := s.db.
		Joins("JOIN users ON users.id = ssh_keys.user_id").
		Where("ssh_keys.fingerprint = ? AND ssh_keys.is_active = ? AND users.is_active = ?", fingerprint, true, true).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("unauthorized public key")
		}
		return 0, err
	}
	return record.UserID, nil
}

func (s *Server) handleConn(nc net.Conn, serverConfig *ssh.ServerConfig) {
	defer nc.Close()

	serverConn, chans, reqs, err := ssh.NewServerConn(nc, serverConfig)
	if err != nil {
		if !strings.Contains(err.Error(), "unauthorized public key") {
			log.Printf("ssh handshake error from %s: %v", nc.RemoteAddr(), err)
		}
		return
	}
	defer serverConn.Close()

	userID, err := userIDFromConn(serverConn)
	if err != nil {
		log.Printf("ssh permission error: %v", err)
		return
	}

	instanceSlug := strings.TrimSpace(serverConn.User())
	if instanceSlug == "" {
		log.Printf("ssh connection missing instance slug from %s", nc.RemoteAddr())
		return
	}

	target, err := s.lookupTarget(userID, instanceSlug)
	if err != nil {
		log.Printf("ssh target lookup failed for user=%d slug=%s: %v", userID, instanceSlug, err)
		return
	}

	upstreamClient, err := s.dialUpstream(target)
	if err != nil {
		log.Printf("upstream dial failed for %s: %v", target.Addr, err)
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

func userIDFromConn(conn *ssh.ServerConn) (uint, error) {
	value := conn.Permissions.Extensions[userIDPermission]
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid user permission")
	}
	return uint(id), nil
}

func (s *Server) lookupTarget(userID uint, slug string) (*upstreamTarget, error) {
	var instance models.Instance
	err := s.db.
		Joins("JOIN instance_assignments ON instance_assignments.instance_id = instances.id").
		Where("instance_assignments.user_id = ? AND instances.slug = ? AND instances.enabled = ?", userID, slug, true).
		First(&instance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("instance not assigned")
		}
		return nil, err
	}

	return &upstreamTarget{
		Addr:       net.JoinHostPort(instance.UpstreamHost, strconv.Itoa(instance.UpstreamPort)),
		User:       instance.UpstreamUser,
		AuthMethod: instance.AuthMethod,
		Password:   instance.AuthPassword,
		KeyInline:  instance.AuthKeyInline,
		Passphrase: instance.AuthPassphrase,
	}, nil
}

func (s *Server) dialUpstream(target *upstreamTarget) (*ssh.Client, error) {
	callback, err := s.hostKeyCallback()
	if err != nil {
		return nil, err
	}

	authMethods := make([]ssh.AuthMethod, 0, 1)
	switch strings.ToLower(strings.TrimSpace(target.AuthMethod)) {
	case "none", "":
	case "password":
		authMethods = append(authMethods, ssh.Password(target.Password))
	case "key":
		if strings.TrimSpace(target.KeyInline) == "" {
			return nil, errors.New("key auth selected but instance auth_key_inline is empty")
		}
		signer, err := sshkeys.SignerFromPrivateKey(target.KeyInline, target.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("parse upstream private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unsupported auth method %q", target.AuthMethod)
	}

	clientConfig := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods,
		HostKeyCallback: callback,
		Timeout:         s.cfg.SSHDialTimeout,
	}

	client, err := ssh.Dial("tcp", target.Addr, clientConfig)
	if err == nil {
		return client, nil
	}
	if s.cfg.SSHKnownHostsPath != "" && s.cfg.SSHAcceptUnknownHost && strings.Contains(err.Error(), "knownhosts: key is unknown") {
		if appendErr := fetchAndAppendHostKey(target.Addr, s.cfg.SSHKnownHostsPath, s.cfg.SSHDialTimeout); appendErr != nil {
			return nil, appendErr
		}
		callback, callbackErr := knownhosts.New(s.cfg.SSHKnownHostsPath)
		if callbackErr != nil {
			return nil, callbackErr
		}
		clientConfig.HostKeyCallback = callback
		return ssh.Dial("tcp", target.Addr, clientConfig)
	}
	return nil, err
}

func (s *Server) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if s.cfg.SSHKnownHostsPath == "" {
		log.Printf("warning: SSH_KNOWN_HOSTS not set; upstream host verification is disabled")
		return ssh.InsecureIgnoreHostKey(), nil
	}
	if err := ensureFile(s.cfg.SSHKnownHostsPath, 0o644); err != nil {
		return nil, err
	}
	return knownhosts.New(s.cfg.SSHKnownHostsPath)
}

func proxyGlobalRequests(clientReqs <-chan *ssh.Request, upstream *ssh.Client) {
	for req := range clientReqs {
		ok := false
		var payload []byte
		var err error
		if req.WantReply {
			ok, payload, err = upstream.SendRequest(req.Type, true, req.Payload)
		} else {
			_, _, err = upstream.SendRequest(req.Type, false, req.Payload)
		}
		if err != nil {
			log.Printf("global request forward error: %v", err)
		}
		if req.WantReply {
			_ = req.Reply(ok, payload)
		}
	}
}

func proxyChannel(newCh ssh.NewChannel, upstream *ssh.Client) error {
	upstreamCh, upstreamReqs, err := upstream.OpenChannel(newCh.ChannelType(), newCh.ExtraData())
	if err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
		return err
	}
	defer upstreamCh.Close()

	localCh, localReqs, err := newCh.Accept()
	if err != nil {
		return err
	}
	defer localCh.Close()

	go forwardRequests(localReqs, upstreamCh)
	go forwardRequests(upstreamReqs, localCh)

	done := make(chan error, 2)
	go func() {
		_, copyErr := io.Copy(upstreamCh, localCh)
		_ = upstreamCh.CloseWrite()
		done <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(localCh, upstreamCh)
		_ = localCh.CloseWrite()
		done <- copyErr
	}()

	<-done
	<-done
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
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, perm)
		if createErr != nil {
			return createErr
		}
		return file.Close()
	} else if err != nil {
		return err
	}
	return nil
}

func fetchAndAppendHostKey(addr, knownHostsPath string, timeout time.Duration) error {
	var captured ssh.PublicKey
	clientConfig := &ssh.ClientConfig{
		User: "discover",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			captured = key
			return nil
		},
		Timeout: timeout,
	}
	_, _ = ssh.Dial("tcp", addr, clientConfig)
	if captured == nil {
		return fmt.Errorf("could not capture upstream host key for %s", addr)
	}

	line := knownhosts.Line([]string{normalizeKnownHostsHost(addr)}, captured)
	if err := ensureFile(knownHostsPath, 0o644); err != nil {
		return err
	}
	file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line + "\n")
	return err
}

func normalizeKnownHostsHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if port == "22" {
		return host
	}
	if !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if strings.TrimSpace(path) == "" {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		return ssh.NewSignerFromKey(privateKey)
	}

	keyBytes, err := os.ReadFile(path)
	if err == nil {
		return ssh.ParsePrivateKey(keyBytes)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(privateKey)
}
