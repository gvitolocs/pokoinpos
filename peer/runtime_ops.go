package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"peer/account"
	"sort"
	"strconv"
	"strings"
	"time"
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
	mux.HandleFunc("/explorer/blocks", srv.explorerBlocks)
	mux.HandleFunc("/explorer/blocks/", srv.explorerBlock)
	mux.HandleFunc("/explorer/tx/", srv.explorerTx)
	mux.HandleFunc("/explorer/address/", srv.explorerAddress)
	mux.HandleFunc("/explorer/search", srv.explorerSearch)
	mux.HandleFunc("/rpc", srv.rpc)
	mux.HandleFunc("/admin/mine", srv.mineSlot)
	mux.HandleFunc("/admin/mint", srv.mint)
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
		"currency": "PKN",
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
				UseCases:       []string{"MetaMask custom network", "EVM wallet network detection", "read-only PKN balance checks"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/explorer/blocks",
				Summary:        "Paginated canonical block list for block explorers.",
				Authentication: "none",
				ContentType:    "application/json",
				QueryParams:    []string{"limit: max 100", "cursor: next block number"},
				ResponseFields: []string{"blocks", "nextCursor"},
				UseCases:       []string{"block explorer", "indexer sync", "public chain discovery"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/explorer/tx/{hash}",
				Summary:        "Canonical transaction details by hash.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "from", "to", "amount", "blockHash", "blockNumber", "transactionIndex"},
				UseCases:       []string{"transaction page", "wallet support", "chain registry validation"},
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
			{
				Method:         http.MethodPost,
				Path:           "/admin/mint",
				Summary:        "Admin-only controlled PKN mint into a target account.",
				Authentication: "Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "to", "amount"},
				UseCases:       []string{"treasury allocation", "wrapped-token reserve funding", "sale inventory setup"},
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
		return hexQuantity(pkToEVMValue(s.peer.BestBalanceOf(strings.ToLower(address)))), nil
	case "eth_getTransactionCount":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionCount requires address"}
		}
		var address string
		if err := json.Unmarshal(req.Params[0], &address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid address"}
		}
		tag := "latest"
		if len(req.Params) > 1 {
			_ = json.Unmarshal(req.Params[1], &tag)
		}
		address = strings.ToLower(address)
		nonce := s.peer.BestNonceOf(address)
		if tag == "pending" {
			nonce = s.peer.PendingNonceOf(address)
		}
		return hexQuantity(new(big.Int).SetUint64(nonce)), nil
	case "eth_getCode":
		return "0x", nil
	case "eth_getBlockByNumber":
		tag := "latest"
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params[0], &tag)
		}
		includeTxs := false
		if len(req.Params) > 1 {
			_ = json.Unmarshal(req.Params[1], &includeTxs)
		}
		return s.evmBlockObject(tag, includeTxs), nil
	case "eth_getBlockByHash":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getBlockByHash requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		includeTxs := false
		if len(req.Params) > 1 {
			_ = json.Unmarshal(req.Params[1], &includeTxs)
		}
		return s.evmBlockObjectByHash(hash, includeTxs), nil
	case "eth_getBlockTransactionCountByNumber":
		tag := "latest"
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params[0], &tag)
		}
		block, ok := s.explorerBlockByTag(tag)
		if !ok {
			return nil, nil
		}
		return hexQuantity(big.NewInt(int64(block.TransactionCount))), nil
	case "eth_getBlockTransactionCountByHash":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getBlockTransactionCountByHash requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		block, ok := BuildExplorerIndex(s.peer).BlocksByHash[strings.ToLower(hash)]
		if !ok {
			return nil, nil
		}
		return hexQuantity(big.NewInt(int64(block.TransactionCount))), nil
	case "eth_getTransactionByBlockNumberAndIndex":
		if len(req.Params) < 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockNumberAndIndex requires block and index"}
		}
		var tag string
		var indexRaw string
		_ = json.Unmarshal(req.Params[0], &tag)
		_ = json.Unmarshal(req.Params[1], &indexRaw)
		block, ok := s.explorerBlockByTag(tag)
		if !ok {
			return nil, nil
		}
		index, ok := parseHexUint64(indexRaw)
		if !ok || int(index) >= len(block.Transactions) {
			return nil, nil
		}
		tx := block.Transactions[index]
		return evmTransactionObject(signedTransactionFromExplorer(tx), tx.BlockHash, tx.BlockNumber, uint64(tx.TransactionIndex)), nil
	case "eth_getTransactionByBlockHashAndIndex":
		if len(req.Params) < 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockHashAndIndex requires block hash and index"}
		}
		var hash string
		var indexRaw string
		_ = json.Unmarshal(req.Params[0], &hash)
		_ = json.Unmarshal(req.Params[1], &indexRaw)
		block, ok := BuildExplorerIndex(s.peer).BlocksByHash[strings.ToLower(hash)]
		if !ok {
			return nil, nil
		}
		index, ok := parseHexUint64(indexRaw)
		if !ok || int(index) >= len(block.Transactions) {
			return nil, nil
		}
		tx := block.Transactions[index]
		return evmTransactionObject(signedTransactionFromExplorer(tx), tx.BlockHash, tx.BlockNumber, uint64(tx.TransactionIndex)), nil
	case "eth_getTransactionByHash":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByHash requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		tx, blockHash, blockNumber, txIndex, exists := s.peer.TransactionLocation(strings.ToLower(hash))
		if !exists {
			return nil, nil
		}
		return evmTransactionObject(tx, blockHash, blockNumber, txIndex), nil
	case "eth_getTransactionReceipt":
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionReceipt requires hash"}
		}
		var hash string
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid hash"}
		}
		tx, blockHash, blockNumber, txIndex, exists := s.peer.TransactionLocation(strings.ToLower(hash))
		if !exists {
			return nil, nil
		}
		if blockHash == "" {
			return nil, nil
		}
		return map[string]any{
			"transactionHash":   tx.ID,
			"transactionIndex":  hexQuantity(new(big.Int).SetUint64(txIndex)),
			"blockHash":         blockHash,
			"blockNumber":       hexQuantity(big.NewInt(int64(blockNumber))),
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
	case "eth_gasPrice", "eth_maxPriorityFeePerGas":
		return evmGasPrice(), nil
	case "eth_estimateGas":
		return "0x5208", nil
	case "eth_feeHistory":
		return s.evmFeeHistory(req.Params), nil
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
		if tx.Nonce < s.peer.BestNonceOf(tx.From) || tx.Nonce > s.peer.PendingNonceOf(tx.From) {
			return nil, &JSONRPCError{Code: -32000, Message: "invalid nonce"}
		}
		if s.peer.BestBalanceOf(tx.From) < tx.Amount {
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

func evmTransactionObject(tx account.SignedTransaction, blockHash string, blockNumber int, txIndex uint64) map[string]any {
	var blockHashValue any
	var blockNumberValue any
	var transactionIndexValue any
	if blockHash != "" {
		blockHashValue = blockHash
		blockNumberValue = hexQuantity(big.NewInt(int64(blockNumber)))
		transactionIndexValue = hexQuantity(new(big.Int).SetUint64(txIndex))
	}
	return map[string]any{
		"hash":             tx.ID,
		"nonce":            hexQuantity(new(big.Int).SetUint64(tx.Nonce)),
		"blockHash":        blockHashValue,
		"blockNumber":      blockNumberValue,
		"transactionIndex": transactionIndexValue,
		"from":             tx.From,
		"to":               tx.To,
		"value":            hexQuantity(pkToEVMValue(tx.Amount)),
		"gas":              "0x5208",
		"gasPrice":         evmGasPrice(),
		"input":            "0x",
	}
}

func (s *OpsServer) evmBlockObject(tag string, includeTxs bool) any {
	height := s.peer.ChainHeight()
	if tag == "earliest" {
		height = 0
	}
	if strings.HasPrefix(tag, "0x") {
		parsed, ok := parseHexUint64(tag)
		if !ok {
			return nil
		}
		height = int(parsed)
	}
	block, blockNumber, exists := s.peer.BlockByNumber(height)
	if !exists {
		return nil
	}
	return s.evmBlockObjectFromBlock(block, blockNumber, includeTxs)
}

func (s *OpsServer) evmBlockObjectByHash(hash string, includeTxs bool) any {
	block, blockNumber, exists := s.peer.BlockByHash(hash)
	if !exists {
		return nil
	}
	return s.evmBlockObjectFromBlock(block, blockNumber, includeTxs)
}

func (s *OpsServer) evmBlockObjectFromBlock(block account.Block, height int, includeTxs bool) map[string]any {
	number := hexQuantity(big.NewInt(int64(height)))
	hashBytes := block.GetBlockHash()
	hash := "0x" + fmt.Sprintf("%x", hashBytes[:])
	transactions := []any{}
	txs, err := account.DecodeBlockTransactions(block.MetaData)
	if err == nil {
		for i, tx := range txs {
			if includeTxs {
				transactions = append(transactions, evmTransactionObject(tx, hash, height, uint64(i)))
			} else {
				transactions = append(transactions, tx.ID)
			}
		}
	}
	return map[string]any{
		"number":           number,
		"hash":             hash,
		"parentHash":       evmParentHash(block.ParentHash),
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
		"gasLimit":         evmGasLimit(),
		"gasUsed":          "0x0",
		"baseFeePerGas":    evmGasPrice(),
		"timestamp":        hexQuantity(big.NewInt(s.peer.UptimeSeconds())),
		"transactions":     transactions,
		"uncles":           []any{},
	}
}

func evmParentHash(parentHash string) string {
	if parentHash == "" {
		return "0x0000000000000000000000000000000000000000000000000000000000000000"
	}
	decoded, err := base64.StdEncoding.DecodeString(parentHash)
	if err != nil {
		return "0x0000000000000000000000000000000000000000000000000000000000000000"
	}
	return "0x" + fmt.Sprintf("%064x", decoded)
}

func (s *OpsServer) evmFeeHistory(params []json.RawMessage) map[string]any {
	blockCount := uint64(1)
	if len(params) > 0 {
		var raw string
		if err := json.Unmarshal(params[0], &raw); err == nil {
			if parsed, ok := parseHexUint64(raw); ok && parsed > 0 {
				blockCount = parsed
			}
		}
	}
	if blockCount > 20 {
		blockCount = 20
	}
	baseFees := make([]string, blockCount+1)
	gasUsedRatios := make([]float64, blockCount)
	for i := range baseFees {
		baseFees[i] = evmGasPrice()
	}
	oldest := uint64(0)
	height := uint64(s.peer.ChainHeight())
	if height+1 > blockCount {
		oldest = height + 1 - blockCount
	}
	return map[string]any{
		"oldestBlock":   hexQuantity(new(big.Int).SetUint64(oldest)),
		"baseFeePerGas": baseFees,
		"gasUsedRatio":  gasUsedRatios,
		"reward":        []any{},
	}
}

func (s *OpsServer) explorerBlockByTag(tag string) (ExplorerBlock, bool) {
	idx := BuildExplorerIndex(s.peer)
	if tag == "" || tag == "latest" {
		block, ok := idx.BlocksByNumber[idx.LatestHeight]
		return block, ok
	}
	if tag == "earliest" {
		block, ok := idx.BlocksByNumber[0]
		return block, ok
	}
	if strings.HasPrefix(tag, "0x") {
		height, ok := parseHexUint64(tag)
		if !ok {
			return ExplorerBlock{}, false
		}
		block, exists := idx.BlocksByNumber[int(height)]
		return block, exists
	}
	height, err := strconv.Atoi(tag)
	if err != nil {
		return ExplorerBlock{}, false
	}
	block, ok := idx.BlocksByNumber[height]
	return block, ok
}

func parseHexUint64(value string) (uint64, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 16, 64)
	return parsed, err == nil
}

func (s *OpsServer) explorerBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	idx := BuildExplorerIndex(s.peer)
	limit := boundedQueryInt(r, "limit", 25, 1, 100)
	cursor := queryInt(r, "cursor", idx.LatestHeight)
	blocks := make([]ExplorerBlock, 0, limit)
	for height := cursor; height >= 0 && len(blocks) < limit; height-- {
		if block, ok := idx.BlocksByNumber[height]; ok {
			block.Transactions = nil
			blocks = append(blocks, block)
		}
	}
	nextCursor := any(nil)
	if len(blocks) == limit && blocks[len(blocks)-1].Number > 0 {
		nextCursor = blocks[len(blocks)-1].Number - 1
	}
	writeJSONAny(w, http.StatusOK, map[string]any{
		"blocks":          blocks,
		"nextCursor":      nextCursor,
		"latestHeight":    idx.LatestHeight,
		"finalizedHeight": idx.FinalizedHeight,
	})
}

func (s *OpsServer) explorerBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/explorer/blocks/")
	idx := BuildExplorerIndex(s.peer)
	var block ExplorerBlock
	var ok bool
	if strings.HasPrefix(strings.ToLower(key), "0x") {
		block, ok = idx.BlocksByHash[strings.ToLower(key)]
	} else if height, err := strconv.Atoi(key); err == nil {
		block, ok = idx.BlocksByNumber[height]
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "block_not_found"})
		return
	}
	writeJSONAny(w, http.StatusOK, block)
}

func (s *OpsServer) explorerTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	hash := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/explorer/tx/"))
	tx, ok := BuildExplorerIndex(s.peer).TxsByHash[hash]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "transaction_not_found"})
		return
	}
	writeJSONAny(w, http.StatusOK, tx)
}

func (s *OpsServer) explorerAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/explorer/address/")
	address := strings.ToLower(strings.TrimSuffix(path, "/txs"))
	idx := BuildExplorerIndex(s.peer)
	txs := idx.TxsByAddress[address]
	sortExplorerTxs(txs)
	limit := boundedQueryInt(r, "limit", 25, 1, 100)
	cursor := queryInt(r, "cursor", 0)
	if cursor < 0 {
		cursor = 0
	}
	end := cursor + limit
	if end > len(txs) {
		end = len(txs)
	}
	page := []ExplorerTx{}
	if cursor < len(txs) {
		page = txs[cursor:end]
	}
	nextCursor := any(nil)
	if end < len(txs) {
		nextCursor = end
	}
	if strings.HasSuffix(path, "/txs") {
		writeJSONAny(w, http.StatusOK, map[string]any{"transactions": page, "nextCursor": nextCursor})
		return
	}
	writeJSONAny(w, http.StatusOK, ExplorerAddress{
		Address:          address,
		Balance:          s.peer.BestBalanceOf(address),
		Nonce:            s.peer.BestNonceOf(address),
		TransactionCount: len(txs),
		Transactions:     page,
	})
}

func (s *OpsServer) explorerSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	idx := BuildExplorerIndex(s.peer)
	if tx, ok := idx.TxsByHash[q]; ok {
		writeJSONAny(w, http.StatusOK, map[string]any{"type": "transaction", "result": tx})
		return
	}
	if block, ok := idx.BlocksByHash[q]; ok {
		writeJSONAny(w, http.StatusOK, map[string]any{"type": "block", "result": block})
		return
	}
	if height, err := strconv.Atoi(q); err == nil {
		if block, ok := idx.BlocksByNumber[height]; ok {
			writeJSONAny(w, http.StatusOK, map[string]any{"type": "block", "result": block})
			return
		}
	}
	if txs, ok := idx.TxsByAddress[q]; ok {
		sortExplorerTxs(txs)
		writeJSONAny(w, http.StatusOK, map[string]any{"type": "address", "result": ExplorerAddress{
			Address:          q,
			Balance:          s.peer.BestBalanceOf(q),
			Nonce:            s.peer.BestNonceOf(q),
			TransactionCount: len(txs),
			Transactions:     limitExplorerTxs(txs, 25),
		}})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
}

func boundedQueryInt(r *http.Request, key string, fallback, min, max int) int {
	value := queryInt(r, key, fallback)
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func sortExplorerTxs(txs []ExplorerTx) {
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].BlockNumber == txs[j].BlockNumber {
			return txs[i].TransactionIndex > txs[j].TransactionIndex
		}
		return txs[i].BlockNumber > txs[j].BlockNumber
	})
}

func limitExplorerTxs(txs []ExplorerTx, limit int) []ExplorerTx {
	if len(txs) <= limit {
		return txs
	}
	return txs[:limit]
}

func (s *OpsServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	body := map[string]any{
		"status":          "ok",
		"currencySymbol":  "PKN",
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
		"currencySymbol":  "PKN",
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

type mintRequest struct {
	To     string `json:"to"`
	Amount int    `json:"amount"`
}

func (s *OpsServer) mint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req mintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	req.To = strings.ToLower(strings.TrimSpace(req.To))
	if req.To == "" || req.Amount < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_mint_request"})
		return
	}
	tx, ok := s.peer.FloodMintTransaction(fmt.Sprintf("mint-%d-%s-%d", time.Now().UnixNano(), req.To, req.Amount), req.To, req.Amount)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "mint_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"hash":   tx.ID,
		"to":     tx.To,
		"amount": tx.Amount,
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

func pkToEVMValue(amount int) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(amount)), account.EVMUnit)
}

func evmGasPrice() string {
	return "0x3b9aca00" // 1 gwei; accepted for wallet compatibility, not charged by the native ledger.
}

func evmGasLimit() string {
	return "0x1c9c380" // 30,000,000 gas.
}
