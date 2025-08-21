package mapping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// LoadFromDB reads mapping entries from Postgres.
// It expects a table with columns described in README (names used below).
func LoadFromDB(ctx context.Context, dsn, table string) (Mapping, error) {
	if strings.TrimSpace(dsn) == "" {
		return Mapping{}, errors.New("SSH_DB_DSN is empty")
	}
	if strings.TrimSpace(table) == "" {
		table = "proxy_mappings"
	}
	// Optional: quick TCP reachability check to fail fast with clearer message
	if host, port := parseHostPortFromDSN(dsn); host != "" && port != "" {
		d := net.Dialer{Timeout: 3 * time.Second}
		if conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port)); err != nil {
			return Mapping{}, fmt.Errorf("db tcp %s:%s unreachable: %w", host, port, err)
		} else {
			_ = conn.Close()
		}
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return Mapping{}, err
	}
	defer conn.Close(ctx)

	// Ensure table exists (idempotent)
	qt, err := quotePGIdent(table)
	if err != nil {
		return Mapping{}, fmt.Errorf("invalid table name %q: %w", table, err)
	}
	createSQL := `CREATE TABLE IF NOT EXISTS ` + qt + ` (
		id bigserial PRIMARY KEY,
		public_key text,
		fingerprint text,
		addr text NOT NULL,
		user_name text NOT NULL,
		auth_method text NOT NULL,
		auth_password text,
		auth_key_inline text,
		auth_passphrase text,
		enabled boolean DEFAULT true
	)`
	if _, err := conn.Exec(ctx, createSQL); err != nil {
		return Mapping{}, fmt.Errorf("create table: %w", err)
	}

	q := fmt.Sprintf("SELECT public_key, fingerprint, addr, user_name, auth_method, auth_password, auth_key_inline, auth_passphrase FROM %s WHERE COALESCE(enabled, true) = true", qt)
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return Mapping{}, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var pk, fp, addr, user, method, password, keyInline, passphrase *string
		if err := rows.Scan(&pk, &fp, &addr, &user, &method, &password, &keyInline, &passphrase); err != nil {
			return Mapping{}, err
		}
		e := Entry{}
		if pk != nil {
			e.PublicKey = strings.TrimSpace(*pk)
		}
		if fp != nil {
			e.Fingerprint = strings.TrimSpace(*fp)
		}
		if addr != nil {
			e.Target.Addr = *addr
		}
		if user != nil {
			e.Target.User = *user
		}
		if method != nil {
			e.Target.Auth.Method = *method
		}
		if password != nil {
			e.Target.Auth.Password = *password
		}
		if keyInline != nil {
			e.Target.Auth.KeyInline = *keyInline
		}
		if passphrase != nil {
			e.Target.Auth.Passphrase = *passphrase
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return Mapping{}, err
	}
	return Mapping{Entries: entries}, nil
}

// parseHostPortFromDSN extracts host and port from a postgres DSN if present.
// Supports formats like postgres://user:pass@host:5432/db?...
func parseHostPortFromDSN(dsn string) (string, string) {
	// Very light parsing to avoid extra deps; handle common scheme.
	// Find '@' then split on '/', then split host:port.
	at := strings.LastIndex(dsn, "@")
	if at == -1 {
		return "", ""
	}
	rest := dsn[at+1:]
	// cut at '/' beginning of path
	slash := strings.IndexByte(rest, '/')
	if slash != -1 {
		rest = rest[:slash]
	}
	hostport := strings.TrimSpace(rest)
	if hostport == "" {
		return "", ""
	}
	// strip brackets for IPv6
	if strings.HasPrefix(hostport, "[") {
		// [::1]:5432
		rb := strings.IndexByte(hostport, ']')
		if rb != -1 {
			host := hostport[1:rb]
			port := ""
			if rb+1 < len(hostport) && hostport[rb+1] == ':' {
				port = hostport[rb+2:]
			}
			return host, port
		}
	}
	// split host:port
	if hp := strings.Split(hostport, ":"); len(hp) == 2 {
		return hp[0], hp[1]
	}
	return hostport, ""
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quotePGIdent safely quotes a possibly schema-qualified table name like schema.table
// allowing only letters, digits and underscore in each part.
func quotePGIdent(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty identifier")
	}
	parts := strings.Split(name, ".")
	if len(parts) > 2 { // allow at most schema.table
		return "", errors.New("invalid identifier format")
	}
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if !identRe.MatchString(p) {
			return "", fmt.Errorf("invalid identifier part %q", p)
		}
		quoted = append(quoted, `"`+p+`"`)
	}
	return strings.Join(quoted, "."), nil
}
