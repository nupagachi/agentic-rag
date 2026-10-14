// Package logx provides the JSON logger and the primitives that keep secrets
// out of logs, error messages and fmt output.
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
)

// Mask is what a secret prints as.
const Mask = "***"

// Secret wraps a sensitive string (password, DSN, token). It implements
// fmt.Formatter, so every fmt verb (including unusual ones such as %d, %x or
// %08s) prints Mask, and slog.LogValuer and encoding.TextMarshaler, so slog and
// encoding/json print Mask too. The raw value is only reachable through Reveal.
//
// The value lives behind a pointer on purpose: fmt cannot call methods on a
// Secret stored in an unexported struct field, so it prints the field's
// underlying kind. For a pointer that is an address, never the string.
type Secret struct {
	p *string
}

// NewSecret wraps v.
func NewSecret(v string) Secret { return Secret{p: &v} }

// Reveal returns the raw value. Callers must never log or format it.
func (s Secret) Reveal() string {
	if s.p == nil {
		return ""
	}
	return *s.p
}

// Format implements fmt.Formatter. It ignores the verb and flags so that no
// fmt path, not even the bad-verb branch, can reach the raw value.
func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, Mask) }

// String implements fmt.Stringer.
func (Secret) String() string { return Mask }

// GoString implements fmt.GoStringer (%#v).
func (Secret) GoString() string { return Mask }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(Mask) }

// MarshalText implements encoding.TextMarshaler.
func (Secret) MarshalText() ([]byte, error) { return []byte(Mask), nil }

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
	if err != nil || u.Scheme == "" || u.Opaque != "" {
		return Mask
	}
	// A fragment is never part of a valid DSN, but it is where an unescaped
	// "#" in a password ends up; never print it.
	u.Fragment, u.RawFragment = "", ""
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
// a secret: the whole value plus, for URLs, the password exactly as written in
// the URL and in its decoded and re-encoded forms.
func (s Secret) needles() []string {
	v := s.Reveal()
	if v == "" {
		return nil
	}
	out := []string{v}
	if raw := rawUserinfoPassword(v); raw != "" {
		out = append(out, raw)
	}
	u, err := url.Parse(v)
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

// rawUserinfoPassword returns the password as literally written in a URL:
// the text between the first ":" of the userinfo and the last "@" of the
// authority. It does not depend on url.Parse succeeding, and keeps the
// original percent-encoding (for example "%2f" in lowercase), which the
// re-encoded forms would not match.
func rawUserinfoPassword(raw string) string {
	_, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return ""
	}
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest = rest[:end]
	}
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return ""
	}
	_, pw, ok := strings.Cut(rest[:at], ":")
	if !ok {
		return ""
	}
	return pw
}
