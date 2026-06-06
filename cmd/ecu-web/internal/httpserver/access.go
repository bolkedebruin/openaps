package httpserver

import (
	"net/http"

	"github.com/bolkedebruin/openaps/wire"
)

// sshKeyDTO is one authorized key projected for the web UI. Fingerprint is
// the stable id the UI passes back to delete a key.
type sshKeyDTO struct {
	Pubkey      string `json:"pubkey"`
	Comment     string `json:"comment,omitempty"`
	AddedMs     int64  `json:"added_ms"`
	Fingerprint string `json:"fingerprint"`
}

// accessDTO shapes the /api/access/ssh-keys GET/POST/DELETE response: the
// provider state plus the full key list. error is set when recoveryd
// rejected the request.
type accessDTO struct {
	Provider string      `json:"provider"`
	HostUser string      `json:"host_user,omitempty"`
	Keys     []sshKeyDTO `json:"keys"`
	Error    string      `json:"error,omitempty"`
}

func accessToDTO(resp *wire.AccessResponse) accessDTO {
	out := accessDTO{
		Provider: resp.GetProvider(),
		HostUser: resp.GetHostUser(),
		Keys:     make([]sshKeyDTO, 0, len(resp.GetKeys())),
	}
	for _, k := range resp.GetKeys() {
		out.Keys = append(out.Keys, sshKeyDTO{
			Pubkey:      k.GetPubkey(),
			Comment:     k.GetComment(),
			AddedMs:     k.GetAddedMs(),
			Fingerprint: k.GetFingerprint(),
		})
	}
	return out
}

// handleListSSHKeys returns the current authorized-key list from recoveryd.
// A recoveryd fetch failure degrades to an empty list + error, never a 5xx
// (mirrors handleEvents) — the page stays usable so the operator can still
// add a key once recoveryd recovers.
func (s *Server) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SSHKeysList == nil {
		http.Error(w, "ssh access plane unavailable", http.StatusServiceUnavailable)
		return
	}
	resp, err := s.cfg.SSHKeysList(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, accessDTO{Keys: []sshKeyDTO{}, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, accessToDTO(resp))
}

// handleAddSSHKey validates and appends an authorized key via recoveryd.
// recoveryd does the real validation (parse + fingerprint + dedupe); a
// rejected key returns HTTP 400 with recoveryd's message.
func (s *Server) handleAddSSHKey(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SSHKeyAdd == nil {
		http.Error(w, "ssh access plane unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Pubkey  string `json:"pubkey"`
		Comment string `json:"comment"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Pubkey == "" {
		http.Error(w, "pubkey required", http.StatusBadRequest)
		return
	}
	resp, err := s.cfg.SSHKeyAdd(r.Context(), body.Pubkey, body.Comment)
	if err != nil {
		msg := err.Error()
		if resp != nil && resp.GetError() != "" {
			msg = resp.GetError()
		}
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, accessToDTO(resp))
}

// handleRemoveSSHKey removes an authorized key by fingerprint via
// recoveryd. Removing a key is high-impact (it can lock an operator out of
// shell access), so it is gated by the same single-use step-up as the
// sensitive settings writes: the operator must POST /api/auth/verify within
// stepUpTTL first, or this returns 403.
func (s *Server) handleRemoveSSHKey(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SSHKeyRemove == nil {
		http.Error(w, "ssh access plane unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Fingerprint string `json:"fingerprint"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Fingerprint == "" {
		http.Error(w, "fingerprint required", http.StatusBadRequest)
		return
	}
	if !s.cfg.Auth.StepUpValid(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "step-up required"})
		return
	}
	resp, err := s.cfg.SSHKeyRemove(r.Context(), body.Fingerprint)
	if err != nil {
		msg := err.Error()
		if resp != nil && resp.GetError() != "" {
			msg = resp.GetError()
		}
		// Do NOT consume step-up on failure: the operator may retry the
		// same removal without re-typing the password.
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// Single-use step-up: a successful removal consumes the flag.
	s.cfg.Auth.ConsumeStepUp(r)
	writeJSON(w, http.StatusOK, accessToDTO(resp))
}
