package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type OpsServer struct {
	peer       *Peer
	adminToken string
}

type EndpointDescription struct {
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Summary        string   `json:"summary"`
	Authentication string   `json:"authentication"`
	ContentType    string   `json:"contentType"`
	ResponseFields []string `json:"responseFields,omitempty"`
	QueryParams    []string `json:"queryParams,omitempty"`
	UseCases       []string `json:"useCases,omitempty"`
}

func StartOpsServer(addr string, peer *Peer, adminToken string) error {
	srv := &OpsServer{peer: peer, adminToken: adminToken}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/ready", srv.ready)
	mux.HandleFunc("/chain/status", srv.chainStatus)
	mux.HandleFunc("/metrics", srv.metrics)
	mux.HandleFunc("/endpoints", srv.endpoints)
	mux.HandleFunc("/admin/mine", srv.mineSlot)
	logEvent("ops_server_starting", map[string]any{"addr": addr})
	return http.ListenAndServe(addr, mux)
}

func (s *OpsServer) endpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":  "pokoinpos-node",
		"currency": "PK",
		"endpoints": []EndpointDescription{
			{
				Method:         http.MethodGet,
				Path:           "/health",
				Summary:        "Liveness and compact node status for dashboards and load balancers.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"status", "currencySymbol", "peerCount", "chainHeight", "committedHeight", "finalityDepth", "mempoolDepth", "acceptedBlocks", "acceptedTxs"},
				UseCases:       []string{"health page", "uptime checks", "load balancer probes"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/ready",
				Summary:        "Readiness status; returns 503 while node runtime is not fully initialized.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"status", "ready"},
				UseCases:       []string{"deployment readiness gate", "container health checks"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/status",
				Summary:        "Detailed chain, finality, peer, mempool, and uptime status.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"currencySymbol", "height", "committedHeight", "finalityDepth", "peerCount", "mempoolDepth", "txCount", "acceptedBlocks", "minedBlocks", "uptimeSeconds"},
				UseCases:       []string{"public health page", "chain explorer status", "node operations dashboard"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/metrics",
				Summary:        "Prometheus text metrics for monitoring and alerting.",
				Authentication: "none",
				ContentType:    "text/plain; version=0.0.4",
				ResponseFields: []string{"pokoinpos_chain_height", "pokoinpos_committed_height", "pokoinpos_finality_depth", "pokoinpos_peer_count", "pokoinpos_mempool_depth", "pokoinpos_blocks_accepted_total", "pokoinpos_blocks_mined_total", "pokoinpos_transactions_accepted_total", "pokoinpos_ledger_transaction_count", "pokoinpos_uptime_seconds"},
				UseCases:       []string{"Prometheus scrape target", "Grafana dashboards", "alerts"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/endpoints",
				Summary:        "Machine-readable catalog of all exposed node API endpoints.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"service", "currency", "endpoints"},
				UseCases:       []string{"website health page", "API documentation UI", "runtime endpoint discovery"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/mine",
				Summary:        "Manually mine a requested slot; intended for controlled operations and smoke tests.",
				Authentication: "Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>",
				ContentType:    "application/json",
				QueryParams:    []string{"slot: positive integer"},
				ResponseFields: []string{"slot", "mined"},
				UseCases:       []string{"admin smoke tests", "manual validator operation"},
			},
		},
	})
}

func (s *OpsServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	body := map[string]any{
		"status":          "ok",
		"currencySymbol":  "PK",
		"peerCount":       s.peer.PeerCount(),
		"chainHeight":     s.peer.ChainHeight(),
		"committedHeight": s.peer.CommittedHeight(),
		"finalityDepth":   s.peer.FinalityDepth(),
		"mempoolDepth":    s.peer.MempoolSize(),
		"acceptedBlocks":  s.peer.AcceptedBlocks(),
		"acceptedTxs":     s.peer.AcceptedTransactions(),
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *OpsServer) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	ready := s.peer.Ready()
	status := "ok"
	code := http.StatusOK
	if !ready {
		status = "initializing"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"status": status,
		"ready":  ready,
	})
}

func (s *OpsServer) chainStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currencySymbol":  "PK",
		"height":          s.peer.ChainHeight(),
		"committedHeight": s.peer.CommittedHeight(),
		"finalityDepth":   s.peer.FinalityDepth(),
		"peerCount":       s.peer.PeerCount(),
		"mempoolDepth":    s.peer.MempoolSize(),
		"txCount":         s.peer.TxHistoryCount(),
		"acceptedBlocks":  s.peer.AcceptedBlocks(),
		"minedBlocks":     s.peer.MinedBlocks(),
		"uptimeSeconds":   s.peer.UptimeSeconds(),
	})
}

func (s *OpsServer) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "pokoinpos_chain_height %d\n", s.peer.ChainHeight())
	fmt.Fprintf(w, "pokoinpos_committed_height %d\n", s.peer.CommittedHeight())
	fmt.Fprintf(w, "pokoinpos_finality_depth %d\n", s.peer.FinalityDepth())
	fmt.Fprintf(w, "pokoinpos_peer_count %d\n", s.peer.PeerCount())
	fmt.Fprintf(w, "pokoinpos_mempool_depth %d\n", s.peer.MempoolSize())
	fmt.Fprintf(w, "pokoinpos_blocks_accepted_total %d\n", s.peer.AcceptedBlocks())
	fmt.Fprintf(w, "pokoinpos_blocks_mined_total %d\n", s.peer.MinedBlocks())
	fmt.Fprintf(w, "pokoinpos_transactions_accepted_total %d\n", s.peer.AcceptedTransactions())
	fmt.Fprintf(w, "pokoinpos_ledger_transaction_count %d\n", s.peer.TxHistoryCount())
	fmt.Fprintf(w, "pokoinpos_uptime_seconds %d\n", s.peer.UptimeSeconds())
}

func (s *OpsServer) mineSlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	slot, err := strconv.Atoi(r.URL.Query().Get("slot"))
	if err != nil || slot < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_slot"})
		return
	}
	block := s.peer.MineOneSlot(slot)
	writeJSON(w, http.StatusOK, map[string]any{
		"slot":  slot,
		"mined": block != nil,
	})
}

func (s *OpsServer) isAdminAuthorized(r *http.Request) bool {
	if s.adminToken == "" {
		return false
	}
	return r.Header.Get("Authorization") == "Bearer "+s.adminToken
}

func writeJSON(w http.ResponseWriter, code int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
