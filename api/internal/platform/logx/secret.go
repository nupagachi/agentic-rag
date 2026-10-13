// Package logx provides the JSON logger and the primitives that keep secrets
// out of logs, error messages and fmt output.
package logx

import (
	"log/slog"
	"net/url"
	"strings"
)

// Mask is what a secret prints as.
const Mask = "***"

// Secret wraps a sensitive string (password, DSN, token). Every formatting
// path (%v, %+v, %#v, %s, slog, encoding/json, encoding text) yields Mask.
// The raw value is only reachable through Reveal.
type Secret struct {
	v string
}

// NewSecret wraps v.
func NewSecret(v string) Secret { return Secret{v: v} }

// Reveal returns the raw value. Callers must never log or format it.
func (s Secret) Reveal() string { return s.v }

// IsZero reports whether the secret is empty.
func (s Secret) IsZero() bool { return s.v == "" }

// String implements fmt.Stringer.
func (Secret) String() string { return Mask }

// GoString implements fmt.GoStringer (%#v).
func (Secret) GoString() string { return Mask }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(Mask) }

// MarshalText implements encoding.TextMarshaler.
func (Secret) MarshalText() ([]byte, error) { return []byte(Mask), nil }

// MarshalJSON implements json.Marshaler.
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + Mask + `"`), nil }

// sensitiveQueryKeys are URL query parameters whose values are credentials.
var sensitiveQueryKeys = map[string]struct{}{
	"password":    {},
	"sslpassword": {},
	"passfile":    {},
	"sslkey":      {},
	"token":       {},
}

// RedactURL returns raw with the password (and credential-like query values)
// replaced by Mask. Anything that does not parse as a URL becomes Mask
// entirely so that a malformed secret is never echoed.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return Mask
	}
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), Mask)
		}
	}
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if _, ok := sensitiveQueryKeys[strings.ToLower(k)]; ok {
				q.Set(k, Mask)
			}
		}
		u.RawQuery = q.Encode()
	}
	// net/url percent-encodes "*"; keep the mask readable.
	return strings.ReplaceAll(u.String(), "%2A%2A%2A", Mask)
}

// needles returns the strings that must be scrubbed from free text when s is
// a secret: the whole value plus, for URLs, the password in raw and decoded form.
func (s Secret) needles() []string {
	if s.v == "" {
		return nil
	}
	out := []string{s.v}
	u, err := url.Parse(s.v)
	if err != nil {
		return out
	}
	if u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			out = append(out, pw, url.QueryEscape(pw), url.PathEscape(pw))
		}
	}
	for k, vals := range u.Query() {
		if _, ok := sensitiveQueryKeys[strings.ToLower(k)]; !ok {
			continue
		}
		for _, v := range vals {
			if v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}
