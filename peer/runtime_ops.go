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

func StartOpsServer(addr string, peer *Peer, adminToken string) error {
	srv := &OpsServer{peer: peer, adminToken: adminToken}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/ready", srv.ready)
	mux.HandleFunc("/chain/status", srv.chainStatus)
	mux.HandleFunc("/metrics", srv.metrics)
	mux.HandleFunc("/admin/mine", srv.mineSlot)
	logEvent("ops_server_starting", map[string]any{"addr": addr})
	return http.ListenAndServe(addr, mux)
}

func (s *OpsServer) health(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{
		"status":         "ok",
		"peerCount":      s.peer.PeerCount(),
		"chainHeight":    s.peer.ChainHeight(),
		"mempoolDepth":   s.peer.MempoolSize(),
		"acceptedBlocks": s.peer.AcceptedBlocks(),
		"acceptedTxs":    s.peer.AcceptedTransactions(),
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *OpsServer) ready(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"height":         s.peer.ChainHeight(),
		"peerCount":      s.peer.PeerCount(),
		"mempoolDepth":   s.peer.MempoolSize(),
		"txCount":        s.peer.TxHistoryCount(),
		"acceptedBlocks": s.peer.AcceptedBlocks(),
		"minedBlocks":    s.peer.MinedBlocks(),
		"uptimeSeconds":  s.peer.UptimeSeconds(),
	})
}

func (s *OpsServer) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "pokoinpos_chain_height %d\n", s.peer.ChainHeight())
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
