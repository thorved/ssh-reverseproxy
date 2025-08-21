package mapping

// Mapping holds a list of entries mapping client public keys to upstream targets.
type Mapping struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	PublicKey   string `json:"publicKey,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Target      Target `json:"target"`
}

type Target struct {
	Addr string `json:"addr"` // host:port
	User string `json:"user"`
	Auth Auth   `json:"auth"`
}

type Auth struct {
	Method     string `json:"method"` // none | password | key
	Password   string `json:"password,omitempty"`
	KeyPath    string `json:"keyPath,omitempty"`   // unused in DB-only mode but kept for compatibility
	KeyInline  string `json:"keyInline,omitempty"` // preferred source when using DB
	Passphrase string `json:"passphrase,omitempty"`
}
