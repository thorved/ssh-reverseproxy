# ssh-reverseproxy

Simple SSH reverse proxy (sshpiper-like) in Go. It accepts SSH connections and, based on the client's public key, forwards the entire SSH session to a mapped upstream host.

What it does
- Verifies if the connecting client's SSH public key is in the local mapping file.
- If present, picks the corresponding upstream host/user and completes an SSH client connection to that upstream.
- Proxies all channels and requests transparently (exec, shell, sftp, port-forwarding, etc.).

It does NOT
- Do per-user auth beyond public key presence check (use upstream for real auth).
- Manage agent forwarding keys itself; it just relays requests.

## Config

You can configure via environment variables or a local .env file.

Quick start with .env
- Copy .env.example to .env and adjust paths
- Set SSH_HOST_KEY_PATH to a persistent private key file so the server's host key fingerprint remains stable across restarts

Environment variables
- SSH_PORT or SSH_LISTEN_ADDR: listen port/address (default :2222)
- SSH_HOST_KEY_PATH: path to server host key (PEM/OpenSSH). If unset, an ephemeral Ed25519 key is generated in memory — this changes on each restart and will trigger "REMOTE HOST IDENTIFICATION HAS CHANGED!" on clients.
- SSH_MAPPING_FILE: path to JSON key-to-target mapping (default mapping.json)
 SSH_ACCEPT_UNKNOWN_UPSTREAM: if true, when an upstream host key is unknown, the proxy will automatically learn it (append to SSH_KNOWN_HOSTS) and retry once. Default false.

Mapping file structure (see mapping.example.json)
{
	"entries": [
 
 Auto-learning unknown upstream keys
 Set SSH_ACCEPT_UNKNOWN_UPSTREAM=true to enable "accept-new" behavior. When the upstream's host key is unknown, the proxy captures it, appends it to SSH_KNOWN_HOSTS, and retries once. Use with caution and only in trusted networks.
		{ "publicKey": "ssh-ed25519 AAAA... comment", "target": { "addr": "host:22", "user": "ubuntu", "auth": { "method": "none" } } },
		{ "fingerprint": "SHA256:...", "target": { "addr": "host:22", "user": "ec2-user", "auth": { "method": "key", "keyPath": "./id_ed25519", "passphrase": "" } } }
	]
}

Match rules
- If fingerprint is set, it must match ssh.FingerprintSHA256 of the client's key (format like SHA256:abc...)
- Otherwise, publicKey is compared by type+base64 (comment ignored)

Upstream auth
- method: one of none | password | key
- key method uses keyPath (relative paths resolved from CWD) and optional passphrase

## Build and run

1) Ensure Go is installed, then fetch deps and build
2) Create a mapping file (copy mapping.example.json to mapping.json and edit)
3) Run the proxy

Example env
SSH_PORT=2222
SSH_MAPPING_FILE=mapping.json
SSH_KNOWN_HOSTS=~/.ssh/known_hosts

Then connect with an SSH client using a key that exists in mapping.json; the session will be forwarded to the mapped upstream.

### About host key warnings
If you restart the proxy and see:
"WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!"
it's because the server host key changed. By default, this proxy generates a new in-memory host key per run. To avoid this, generate or point to a persistent host key file and set SSH_HOST_KEY_PATH. Example to generate an ed25519 host key:

Optional commands
ssh-keygen -t ed25519 -f hostkey_ed25519 -N ""
echo "SSH_HOST_KEY_PATH=$(pwd)/hostkey_ed25519" >> .env

## License

MIT