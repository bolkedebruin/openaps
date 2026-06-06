package recoveryd

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// Key is one parsed authorized_keys entry. Fingerprint is the stable id
// (ssh.FingerprintSHA256) used for dedupe and removal. AddedMs is unset
// when parsed from a file (authorized_keys carries no timestamp) and is
// stamped on AddKey for the response payload. Options carries any leading
// authorized_keys restrictions (command=, from=, no-pty, …) so a
// restricted key survives the file rewrite that every mutation performs;
// dropping them would silently widen a forced-command key to full shell
// access.
type Key struct {
	Pubkey      string
	Comment     string
	AddedMs     int64
	Fingerprint string
	Options     []string
}

// ParseKey validates a single OpenSSH authorized-key line and returns a
// normalised Key. It rejects malformed input. The pubkey is canonicalised
// to "<type> <base64>", but any leading options are preserved verbatim on
// the Key so a restricted entry is re-emitted intact rather than widened.
//
// comment overrides any trailing comment parsed from line when non-empty.
func ParseKey(line, comment string) (Key, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Key{}, fmt.Errorf("empty pubkey")
	}
	pub, parsedComment, options, _, err := ssh.ParseAuthorizedKey([]byte(line))
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
	// Options are re-emitted verbatim ahead of the key, so a control byte
	// there is the same injection risk as in the comment — reject it too.
	for _, opt := range options {
		if i := strings.IndexFunc(opt, func(r rune) bool { return unicode.IsControl(r) }); i >= 0 {
			return Key{}, fmt.Errorf("option must be single line (control character at offset %d)", i)
		}
	}
	// Marshal back to "<type> <base64>" — drops the trailing comment so the
	// stored/rendered form is canonical. Options are kept separately.
	canonical := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	return Key{
		Pubkey:      canonical,
		Comment:     c,
		Fingerprint: ssh.FingerprintSHA256(pub),
		Options:     options,
	}, nil
}

// renderLine returns the authorized_keys line for a key: any options, the
// canonical pubkey, and the comment when present, in OpenSSH order
// ("options type base64 comment").
func renderLine(k Key) string {
	line := k.Pubkey
	if len(k.Options) > 0 {
		line = strings.Join(k.Options, ",") + " " + line
	}
	if c := strings.TrimSpace(k.Comment); c != "" {
		line += " " + c
	}
	return line
}
