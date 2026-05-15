package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"peer/account"
	"strconv"
	"strings"
)

type OpsServer struct {
	peer         *Peer
	adminToken   string
	evmChainID   int64
	evmNetworkID string
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

type JSONRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      any               `json:"id,omitempty"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func StartOpsServer(addr string, peer *Peer, adminToken string, evmChainID int64, evmNetworkID string) error {
	srv := &OpsServer{peer: peer, adminToken: adminToken, evmChainID: evmChainID, evmNetworkID: evmNetworkID}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/ready", srv.ready)
	mux.HandleFunc("/chain/status", srv.chainStatus)
	mux.HandleFunc("/metrics", srv.metrics)
	mux.HandleFunc("/endpoints", srv.endpoints)
	mux.HandleFunc("/rpc", srv.rpc)
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
				Path:           "/rpc",
				Summary:        "EVM-style JSON-RPC compatibility endpoint for MetaMask-style wallets.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"jsonrpc", "id", "result", "error"},
				UseCases:       []string{"MetaMask custom network", "EVM wallet network detection", "read-only PK balance checks"},
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

func (s *OpsServer) rpc(w http.ResponseWriter, r *http.Request) {
	setRPCHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"jsonrpc": "2.0",
			"error":   JSONRPCError{Code: -32700, Message: "parse error"},
		})
		return
	}
	if len(raw) > 0 && raw[0] == '[' {
		var batch []JSONRPCRequest
		if err := json.Unmarshal(raw, &batch); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"error":   JSONRPCError{Code: -32700, Message: "parse error"},
			})
			return
		}
		responses := make([]map[string]any, 0, len(batch))
		for _, batchReq := range batch {
			responses = append(responses, s.rpcResponse(batchReq))
		}
		writeJSONAny(w, http.StatusOK, responses)
		return
	}
	var req JSONRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"jsonrpc": "2.0",
			"error":   JSONRPCError{Code: -32700, Message: "parse error"},
		})
		return
	}
	writeJSONAny(w, http.StatusOK, s.rpcResponse(req))
}

func (s *OpsServer) rpcResponse(req JSONRPCRequest) map[string]any {
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	result, rpcErr := s.handleRPC(req)
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}
	if rpcErr != nil {
		response["error"] = rpcErr
	} else {
		response["result"] = result
	}
	return response
}

func setRPCHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *OpsServer) handleRPC(req JSONRPCRequest) (any, *JSONRPCError) {
	switch req.Method {
	case "web3_clientVersion":
		return "PokoinPoS/v0.1.0", nil
	case "net_version":
		return s.evmNetworkID, nil
	case "eth_chainId":
		return hexQuantity(big.NewInt(s.evmChainID)), nil
	case "eth_blockNumber":
		return hexQuantity(big.NewInt(int64(s.peer.ChainHeight()))), nil
	case "eth_syncing":
		return false, nil
	case "eth_accounts":
		return []string{}, nil
	case "eth_requestAccounts":
		return []string{}, nil
	case "eth_getBalance":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getBalance requires address"}
		}
		var address string
		if err := json.Unmarshal(req.Params[0], &address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid address"}
		}
		return hexQuantity(big.NewInt(int64(s.peer.BalanceOf(strings.ToLower(address))))), nil
	case "eth_getTransactionCount":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionCount requires address"}
		}
		var address string
		if err := json.Unmarshal(req.Params[0], &address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid address"}
		}
		return hexQuantity(new(big.Int).SetUint64(s.peer.NonceOf(strings.ToLower(address)))), nil
	case "eth_getCode":
		return "0x", nil
	case "eth_getBlockByNumber":
		tag := "latest"
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params[0], &tag)
		}
		return s.evmBlockObject(tag), nil
	case "eth_getTransactionByHash":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByHash requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		tx, exists := s.peer.TransactionByID(strings.ToLower(hash))
		if !exists {
			return nil, nil
		}
		return evmTransactionObject(tx), nil
	case "eth_getTransactionReceipt":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionReceipt requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		tx, exists := s.peer.TransactionByID(strings.ToLower(hash))
		if !exists {
			return nil, nil
		}
		return map[string]any{
			"transactionHash":   tx.ID,
			"transactionIndex":  "0x0",
			"blockHash":         "0x0",
			"blockNumber":       hexQuantity(big.NewInt(int64(s.peer.ChainHeight()))),
			"from":              tx.From,
			"to":                tx.To,
			"cumulativeGasUsed": "0x5208",
			"gasUsed":           "0x5208",
			"contractAddress":   nil,
			"logs":              []any{},
			"status":            "0x1",
		}, nil
	case "eth_mining":
		return false, nil
	case "eth_hashrate":
		return "0x0", nil
	case "eth_gasPrice":
		return "0x0", nil
	case "eth_estimateGas":
		return "0x5208", nil
	case "eth_sendRawTransaction":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_sendRawTransaction requires raw transaction"}
		}
		var raw string
		if err := json.Unmarshal(req.Params[0], &raw); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid raw transaction"}
		}
		tx, err := account.NewEVMTransaction(raw, s.evmChainID)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if s.peer.NonceOf(tx.From) != tx.Nonce {
			return nil, &JSONRPCError{Code: -32000, Message: "invalid nonce"}
		}
		if s.peer.BalanceOf(tx.From) < tx.Amount {
			return nil, &JSONRPCError{Code: -32000, Message: "insufficient funds"}
		}
		s.peer.FloodPoSTransaction(tx)
		return tx.ID, nil
	case "eth_sendTransaction":
		return nil, &JSONRPCError{Code: -32000, Message: "eth_sendTransaction requires an unlocked node account; use wallet-signed eth_sendRawTransaction"}
	default:
		return nil, &JSONRPCError{Code: -32601, Message: "method not found"}
	}
}

func hexQuantity(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

func evmTransactionObject(tx account.SignedTransaction) map[string]any {
	return map[string]any{
		"hash":             tx.ID,
		"nonce":            hexQuantity(new(big.Int).SetUint64(tx.Nonce)),
		"blockHash":        nil,
		"blockNumber":      nil,
		"transactionIndex": nil,
		"from":             tx.From,
		"to":               tx.To,
		"value":            hexQuantity(big.NewInt(int64(tx.Amount))),
		"gas":              "0x5208",
		"gasPrice":         "0x0",
		"input":            "0x",
	}
}

func (s *OpsServer) evmBlockObject(tag string) map[string]any {
	height := s.peer.ChainHeight()
	if tag == "earliest" {
		height = 0
	}
	number := hexQuantity(big.NewInt(int64(height)))
	return map[string]any{
		"number":           number,
		"hash":             "0x0000000000000000000000000000000000000000000000000000000000000000",
		"parentHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"logsBloom":        "0x" + strings.Repeat("0", 512),
		"transactionsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"stateRoot":        "0x0000000000000000000000000000000000000000000000000000000000000000",
		"receiptsRoot":     "0x0000000000000000000000000000000000000000000000000000000000000000",
		"miner":            "0x0000000000000000000000000000000000000000",
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        "0x",
		"size":             "0x0",
		"gasLimit":         "0x0",
		"gasUsed":          "0x0",
		"timestamp":        hexQuantity(big.NewInt(s.peer.UptimeSeconds())),
		"transactions":     []any{},
		"uncles":           []any{},
	}
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
	writeJSONAny(w, code, body)
}

func writeJSONAny(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
