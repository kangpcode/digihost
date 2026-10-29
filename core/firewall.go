// DigiPanel Firewall API (core/firewall.go)
// Managed nftables blocklist. The panel stores the rules, the privileged agent
// renders and applies them, and a failed apply is reported instead of being
// hidden behind a success response.
// Zero emojis, strictly production grade.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"digipanel/modules/firewall"
)

const maxBlockHours = 24 * 365

func firewallStatusFor(err error) int {
	switch {
	case errors.Is(err, firewall.ErrRunnerMissing), errors.Is(err, ErrAgentUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, firewall.ErrInvalidTarget), errors.Is(err, firewall.ErrInvalidReason),
		errors.Is(err, firewall.ErrInvalidTTL), errors.Is(err, firewall.ErrTooManyRules):
		return http.StatusBadRequest
	case errors.Is(err, firewall.ErrRuleExists):
		return http.StatusConflict
	case errors.Is(err, firewall.ErrRuleNotFound):
		return http.StatusNotFound
	default:
		return http.StatusBadGateway
	}
}

func (state *ServerState) firewallManager() *firewall.Manager {
	return state.Firewall
}

func (state *ServerState) handleFirewallRules(w http.ResponseWriter, r *http.Request) {
	manager := state.firewallManager()
	if manager == nil {
		writeAPIError(w, firewall.ErrRunnerMissing.Error(), http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rules, err := manager.Rules()
		if err != nil {
			writeAPIError(w, err.Error(), firewallStatusFor(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)

	case http.MethodPost:
		var body struct {
			Target        string `json:"target"`
			Reason        string `json:"reason"`
			DurationHours int    `json:"durationHours"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeAPIError(w, "Invalid firewall payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.DurationHours < 0 || body.DurationHours > maxBlockHours {
			writeAPIError(w, fmt.Sprintf("durationHours must be between 1 and %d, or 0 for permanent", maxBlockHours), http.StatusBadRequest)
			return
		}
		ttl := time.Duration(body.DurationHours) * time.Hour
		rule, err := manager.Block(r.Context(), body.Target, body.Reason, ttl)
		if err != nil {
			// A rule persisted before a failed apply is still reported so the
			// operator can retry the apply; the error text says so.
			writeAPIError(w, err.Error(), firewallStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "firewall.block", "firewall_rule", rule.ID,
			fmt.Sprintf(`{"target":%q,"hours":%d}`, rule.Target, body.DurationHours), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(rule)

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeAPIError(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := manager.Unblock(r.Context(), id); err != nil {
			writeAPIError(w, err.Error(), firewallStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "firewall.unblock", "firewall_rule", id, `{}`,
			clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})

	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleFirewallApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := state.firewallManager()
	if manager == nil {
		writeAPIError(w, firewall.ErrRunnerMissing.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := manager.Apply(r.Context()); err != nil {
		writeAPIError(w, err.Error(), firewallStatusFor(err))
		return
	}
	state.AuditLogger.Append(state.actorFor(r), "firewall.apply", "firewall_table", "inet digipanel", `{}`,
		clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"applied": true})
}

func (state *ServerState) handleFirewallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := state.firewallManager()
	if manager == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(firewall.Status{
			Table:     "inet digipanel",
			LastError: firewall.ErrRunnerMissing.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manager.Status(r.Context()))
}
