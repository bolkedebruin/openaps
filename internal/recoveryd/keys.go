package recoveryd

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// ParseKey validates a single OpenSSH authorized-key line and returns a
// normalised Key. It rejects malformed input. The normalised pubkey is
// the canonical "<type> <base64>" marshalled form (no host options, no
// trailing comment) so rendering and dedupe are deterministic.
//
// comment overrides any trailing comment parsed from line when non-empty.
func ParseKey(line, comment string) (Key, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Key{}, fmt.Errorf("empty pubkey")
	}
	pub, parsedComment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return Key{}, fmt.Errorf("malformed pubkey: %w", err)
	}
	c := strings.TrimSpace(comment)
	if c == "" {
		c = strings.TrimSpace(parsedComment)
	}
	// The comment is appended verbatim to the rendered authorized_keys
	// line, so a control character (notably \n or \r) would inject a
	// second authorized_keys entry / key restriction. Reject any control
	// byte so the rendered file is exactly the validated key set.
	if i := strings.IndexFunc(c, func(r rune) bool { return unicode.IsControl(r) }); i >= 0 {
		return Key{}, fmt.Errorf("comment must be single line (control character at offset %d)", i)
	}
	// Marshal back to "<type> <base64>" — drops options and comment so
	// the stored/rendered form is canonical.
	canonical := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	return Key{
		Pubkey:      canonical,
		Comment:     c,
		Fingerprint: ssh.FingerprintSHA256(pub),
	}, nil
}

// renderLine returns the authorized_keys line for a key: the canonical
// pubkey with the comment appended when present.
func renderLine(k Key) string {
	if c := strings.TrimSpace(k.Comment); c != "" {
		return k.Pubkey + " " + c
	}
	return k.Pubkey
}
