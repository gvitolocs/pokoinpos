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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type OpsServer struct {
	peer                    *Peer
	adminToken              string
	evmChainID              int64
	evmNetworkID            string
	bootstrap               *BootstrapRegistry
	reservedSupplyAddresses []string
}

const nodeVersion = "PokoinPoS/v0.2.0"

type SupplySnapshot struct {
	TotalSupply       int      `json:"totalSupply"`
	CirculatingSupply int      `json:"circulatingSupply"`
	ReservedSupply    int      `json:"reservedSupply"`
	ReservedAddresses []string `json:"reservedAddresses"`
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

func StartOpsServer(addr string, peer *Peer, adminToken string, evmChainID int64, evmNetworkID string, bootstrap *BootstrapRegistry, reservedSupplyAddresses []string) error {
	srv := &OpsServer{peer: peer, adminToken: adminToken, evmChainID: evmChainID, evmNetworkID: evmNetworkID, bootstrap: bootstrap, reservedSupplyAddresses: reservedSupplyAddresses}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/ready", srv.ready)
	mux.HandleFunc("/dashboard", srv.dashboard)
	mux.HandleFunc("/deshboard", srv.dashboard)
	mux.HandleFunc("/chain/status", srv.chainStatus)
	mux.HandleFunc("/chain/info", srv.chainInfo)
	mux.HandleFunc("/chain/exchange-info", srv.chainInfo)
	mux.HandleFunc("/chain/supply", srv.chainSupply)
	mux.HandleFunc("/chain/supply/total.txt", srv.totalSupplyText)
	mux.HandleFunc("/chain/supply/circulating.txt", srv.circulatingSupplyText)
	mux.HandleFunc("/chain/nfts", srv.chainNFTs)
	mux.HandleFunc("/chain/nfts/", srv.chainNFTs)
	mux.HandleFunc("/chain/swap/pools", srv.swapPools)
	mux.HandleFunc("/chain/swap/pools/", srv.swapPools)
	mux.HandleFunc("/chain/swap/balances/", srv.swapBalances)
	mux.HandleFunc("/chain/swap/quote", srv.swapQuote)
	mux.HandleFunc("/chain/validators", srv.validators)
	mux.HandleFunc("/chain/bootstrap", srv.bootstrapStatus)
	mux.HandleFunc("/metrics", srv.metrics)
	mux.HandleFunc("/endpoints", srv.endpoints)
	mux.HandleFunc("/explorer/blocks", srv.explorerBlocks)
	mux.HandleFunc("/explorer/blocks/", srv.explorerBlock)
	mux.HandleFunc("/explorer/tx/", srv.explorerTx)
	mux.HandleFunc("/explorer/address/", srv.explorerAddress)
	mux.HandleFunc("/explorer/search", srv.explorerSearch)
	mux.HandleFunc("/", srv.root)
	mux.HandleFunc("/rpc", srv.rpc)
	mux.HandleFunc("/admin/dashboard/status", srv.adminDashboardStatus)
	mux.HandleFunc("/admin/mempool", srv.adminMempool)
	mux.HandleFunc("/admin/mine", srv.mineSlot)
	mux.HandleFunc("/admin/mint", srv.mint)
	mux.HandleFunc("/admin/withdraw", srv.withdraw)
	mux.HandleFunc("/admin/nft/mint", srv.nftMint)
	mux.HandleFunc("/admin/nft/transfer", srv.nftTransfer)
	mux.HandleFunc("/admin/swap/pools", srv.adminSwapPool)
	mux.HandleFunc("/admin/swap/liquidity/add", srv.adminSwapAddLiquidity)
	mux.HandleFunc("/admin/assets/credit", srv.adminAssetCredit)
	mux.HandleFunc("/admin/assets/debit", srv.adminAssetDebit)
	logEvent("ops_server_starting", map[string]any{"addr": addr})
	return http.ListenAndServe(addr, mux)
}

func (s *OpsServer) root(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setRPCHeaders(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method == http.MethodPost {
		s.rpc(w, r)
		return
	}
	http.NotFound(w, r)
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
				ResponseFields: []string{"status", "version", "currencySymbol", "peerCount", "chainHeight", "committedHeight", "finalityDepth", "mempoolDepth", "validatorStake", "acceptedBlocks", "acceptedTxs"},
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
				ResponseFields: []string{"version", "currencySymbol", "height", "committedHeight", "finalityDepth", "peerCount", "mempoolDepth", "validatorStake", "txCount", "acceptedBlocks", "minedBlocks", "uptimeSeconds"},
				UseCases:       []string{"public health page", "chain explorer status", "node operations dashboard"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/info",
				Summary:        "Exchange and scanner friendly network metadata, native token data, supply, finality, consensus, RPC, and latest block status.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"network", "nativeCurrency", "rpc", "explorer", "status", "supply", "consensus", "validators", "latestBlock"},
				UseCases:       []string{"exchange listing", "block scanner config", "wallet registry", "market data integrations"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/validators",
				Summary:        "Dynamic connected peer validator view based on advertised validator account and P2P membership.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"validators"},
				UseCases:       []string{"node operator dashboard", "validator authorization checks", "peer diagnostics"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/nfts",
				Summary:        "Native Pokoin NFT registry with token metadata and ownership.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"tokens", "count"},
				QueryParams:    []string{"owner: optional wallet/account owner filter"},
				UseCases:       []string{"Card Vault inventory", "native NFT explorer", "wallet ownership display"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/nfts/{collectionId}/{tokenId}",
				Summary:        "Native Pokoin NFT token lookup by collection and token ID.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"collectionId", "tokenId", "owner", "metadataUri", "metadataHash", "imageUri", "mintTx", "lastTx"},
				UseCases:       []string{"NFT detail page", "metadata integrity check", "ownership verification"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/swap/pools",
				Summary:        "Native PokoinSwap AMM pool registry and reserves.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"pools", "count"},
				UseCases:       []string{"swap UI", "liquidity dashboard", "native DEX discovery"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/swap/quote",
				Summary:        "Native PokoinSwap deterministic quote for a pool and input amount.",
				Authentication: "none",
				ContentType:    "application/json",
				QueryParams:    []string{"pool", "assetIn", "amountIn"},
				ResponseFields: []string{"poolId", "assetIn", "assetOut", "amountIn", "amountOut", "feeBps"},
				UseCases:       []string{"swap preview", "slippage checks", "Card Vault wallet quoting"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/chain/bootstrap",
				Summary:        "Dynamic bootstrap registry, vetting, uptime, and fallback peer status.",
				Authentication: "none",
				ContentType:    "application/json",
				ResponseFields: []string{"manifestUrl", "lastRefreshAt", "lastError", "policy", "peers", "fallbackPeers"},
				UseCases:       []string{"local dashboard", "bootstrap vetting visibility", "peer discovery diagnostics"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/metrics",
				Summary:        "Prometheus text metrics for monitoring and alerting.",
				Authentication: "none",
				ContentType:    "text/plain; version=0.0.4",
				ResponseFields: []string{"pokoinpos_chain_height", "pokoinpos_committed_height", "pokoinpos_finality_depth", "pokoinpos_peer_count", "pokoinpos_mempool_depth", "pokoinpos_validator_stake", "pokoinpos_blocks_accepted_total", "pokoinpos_blocks_mined_total", "pokoinpos_transactions_accepted_total", "pokoinpos_ledger_transaction_count", "pokoinpos_uptime_seconds"},
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
				Method:         http.MethodGet,
				Path:           "/dashboard",
				Summary:        "Local node-host dashboard for operators running this PokoinPoS node.",
				Authentication: "none for read-only cards; operator actions require Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "text/html; charset=utf-8",
				UseCases:       []string{"local node hosting dashboard", "operator status checks", "guarded admin actions"},
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
				Path:           "/admin/mempool",
				Summary:        "Operator-only node-local mempool diagnostics with pending transaction summaries and best-ledger validity.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"count", "transactions"},
				UseCases:       []string{"debug stale mempool entries", "compare node-local pending transactions", "operator diagnostics"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/mine",
				Summary:        "Manually mine a requested slot; intended for controlled operations and smoke tests.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				QueryParams:    []string{"slot: positive integer"},
				ResponseFields: []string{"slot", "mined"},
				UseCases:       []string{"admin smoke tests", "manual validator operation"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/mint",
				Summary:        "Admin-only controlled PKN mint into a target account.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "to", "amount"},
				UseCases:       []string{"treasury allocation", "wrapped-token reserve funding", "sale inventory setup"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/withdraw",
				Summary:        "Withdraw this validator's spendable PKN balance to a payout EVM wallet address.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "to", "amount"},
				UseCases:       []string{"validator payout", "operator rewards withdrawal"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/nft/mint",
				Summary:        "Admin-only native NFT mint into a target owner account.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "collectionId", "tokenId", "owner", "metadataUri", "metadataHash", "imageUri"},
				UseCases:       []string{"Card Vault minting", "collection issuance", "metadata anchoring"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/nft/transfer",
				Summary:        "Admin-only native NFT transfer from the validator-owned token inventory.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "collectionId", "tokenId", "to"},
				UseCases:       []string{"operator inventory transfer", "custodial Card Vault delivery"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/swap/pools",
				Summary:        "Admin-only native PokoinSwap pool creation.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "poolId", "assetA", "assetB"},
				UseCases:       []string{"PKN/wPKN pool setup", "operator liquidity initialization"},
			},
			{
				Method:         http.MethodPost,
				Path:           "/admin/swap/liquidity/add",
				Summary:        "Admin-only native PokoinSwap liquidity addition.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"hash", "poolId", "amountA", "amountB"},
				UseCases:       []string{"reserve seeding", "operator liquidity management"},
			},
			{
				Method:         http.MethodGet,
				Path:           "/admin/dashboard/status",
				Summary:        "Protected dashboard capability check for the local node-host dashboard.",
				Authentication: "Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>",
				ContentType:    "application/json",
				ResponseFields: []string{"adminEnabled", "actions", "chainId", "networkId"},
				UseCases:       []string{"dashboard token validation", "operator admin readiness"},
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
		return nodeVersion, nil
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
		if len(req.Params) < 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getCode requires address"}
		}
		var address string
		if err := json.Unmarshal(req.Params[0], &address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid address"}
		}
		return "0x" + common.Bytes2Hex(s.peer.BestEVMCode(address)), nil
	case "eth_getStorageAt":
		if len(req.Params) < 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getStorageAt requires address and slot"}
		}
		var address string
		var slot string
		if err := json.Unmarshal(req.Params[0], &address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid address"}
		}
		if err := json.Unmarshal(req.Params[1], &slot); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid slot"}
		}
		return s.peer.BestEVMStorage(address, common.HexToHash(slot).Hex()), nil
	case "eth_call":
		return s.evmCall(req.Params)
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
		receipt, hasReceipt := s.peer.EVMReceipt(tx.ID)
		var contractAddress any
		gasUsed := uint64(21_000)
		logs := []any{}
		if hasReceipt {
			gasUsed = receipt.GasUsed
			if receipt.ContractAddress != "" {
				contractAddress = receipt.ContractAddress
			}
			logs = evmReceiptLogs(receipt, tx.ID, blockHash, blockNumber, txIndex)
		} else if tx.Raw != "" {
			if decoded, err := tx.DecodeEVMForRPC(); err == nil && decoded.To() == nil {
				contractAddress = strings.ToLower(crypto.CreateAddress(common.HexToAddress(tx.From), tx.Nonce).Hex())
			}
		}
		return map[string]any{
			"transactionHash":   tx.ID,
			"transactionIndex":  hexQuantity(new(big.Int).SetUint64(txIndex)),
			"blockHash":         blockHash,
			"blockNumber":       hexQuantity(big.NewInt(int64(blockNumber))),
			"from":              tx.From,
			"to":                tx.To,
			"cumulativeGasUsed": hexQuantity(new(big.Int).SetUint64(gasUsed)),
			"gasUsed":           hexQuantity(new(big.Int).SetUint64(gasUsed)),
			"contractAddress":   contractAddress,
			"logs":              logs,
			"status":            "0x1",
		}, nil
	case "eth_mining":
		return false, nil
	case "eth_hashrate":
		return "0x0", nil
	case "eth_gasPrice", "eth_maxPriorityFeePerGas":
		return evmGasPrice(), nil
	case "eth_estimateGas":
		return s.evmEstimateGas(req.Params)
	case "eth_getLogs":
		return s.evmGetLogs(req.Params)
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
	input := "0x"
	if tx.Raw != "" {
		if decoded, err := tx.DecodeEVMForRPC(); err == nil {
			input = "0x" + fmt.Sprintf("%x", decoded.Data())
		}
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
		"input":            input,
	}
}

func (s *OpsServer) evmCall(params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) < 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_call requires call object"}
	}
	var call map[string]any
	if err := json.Unmarshal(params[0], &call); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid call object"}
	}
	to, _ := call["to"].(string)
	if to == "" {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_call requires to"}
	}
	from, _ := call["from"].(string)
	if from == "" {
		from = "0x0000000000000000000000000000000000000000"
	}
	data, _ := call["data"].(string)
	if data == "" {
		data, _ = call["input"].(string)
	}
	ret, ok := s.peer.BestEVMCall(from, to, common.FromHex(data))
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "evm call reverted"}
	}
	return "0x" + common.Bytes2Hex(ret), nil
}

func evmReceiptLogs(receipt account.EVMReceipt, txHash string, blockHash string, blockNumber int, txIndex uint64) []any {
	logs := make([]any, 0, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs = append(logs, map[string]any{
			"address":          log.Address,
			"topics":           log.Topics,
			"data":             log.Data,
			"blockNumber":      hexQuantity(big.NewInt(int64(blockNumber))),
			"transactionHash":  txHash,
			"transactionIndex": hexQuantity(new(big.Int).SetUint64(txIndex)),
			"blockHash":        blockHash,
			"logIndex":         hexQuantity(big.NewInt(int64(i))),
			"removed":          false,
		})
	}
	return logs
}

func (s *OpsServer) evmEstimateGas(params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) < 1 {
		return "0x5208", nil
	}
	var call map[string]any
	if err := json.Unmarshal(params[0], &call); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid call object"}
	}
	data, _ := call["data"].(string)
	if data == "" {
		data, _ = call["input"].(string)
	}
	if data != "" && data != "0x" {
		return hexQuantity(big.NewInt(5_000_000)), nil
	}
	return "0x5208", nil
}

func (s *OpsServer) evmGetLogs(params []json.RawMessage) (any, *JSONRPCError) {
	filter := map[string]any{}
	if len(params) > 0 {
		if err := json.Unmarshal(params[0], &filter); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid log filter"}
		}
	}
	addressFilter := map[string]bool{}
	switch raw := filter["address"].(type) {
	case string:
		addressFilter[strings.ToLower(raw)] = true
	case []any:
		for _, item := range raw {
			if address, ok := item.(string); ok {
				addressFilter[strings.ToLower(address)] = true
			}
		}
	}
	idx := BuildExplorerIndex(s.peer)
	out := []any{}
	for _, tx := range idx.TxsByHash {
		receipt, ok := s.peer.EVMReceipt(tx.Hash)
		if !ok {
			continue
		}
		logs := evmReceiptLogs(receipt, tx.Hash, tx.BlockHash, tx.BlockNumber, uint64(tx.TransactionIndex))
		for _, rawLog := range logs {
			log, ok := rawLog.(map[string]any)
			if !ok {
				continue
			}
			if len(addressFilter) > 0 {
				address, _ := log["address"].(string)
				if !addressFilter[strings.ToLower(address)] {
					continue
				}
			}
			out = append(out, log)
		}
	}
	return out, nil
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
		"version":         nodeVersion,
		"currencySymbol":  "PKN",
		"peerCount":       s.peer.PeerCount(),
		"chainHeight":     s.peer.ChainHeight(),
		"committedHeight": s.peer.CommittedHeight(),
		"finalityDepth":   s.peer.FinalityDepth(),
		"mempoolDepth":    s.peer.MempoolSize(),
		"validatorStake":  s.peer.ValidatorStake(),
		"acceptedBlocks":  s.peer.AcceptedBlocks(),
		"acceptedTxs":     s.peer.AcceptedTransactions(),
		"lotteryAttempts": s.peer.LotteryAttempts(),
		"lotteryWins":     s.peer.LotteryWins(),
		"lotteryMisses":   s.peer.LotteryMisses(),
		"lotteryNoTicket": s.peer.LotteryNoTicket(),
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
	idx := BuildExplorerIndex(s.peer)
	latest := idx.BlocksByNumber[idx.LatestHeight]
	avgSlots := s.peer.AverageRecentBlockSlots(32)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":           nodeVersion,
		"currencySymbol":    "PKN",
		"currencyName":      "Pokoin",
		"decimals":          18,
		"chainId":           s.evmChainID,
		"networkId":         s.evmNetworkID,
		"height":            s.peer.ChainHeight(),
		"committedHeight":   s.peer.CommittedHeight(),
		"finalityDepth":     s.peer.FinalityDepth(),
		"latestBlockHash":   latest.Hash,
		"latestBlockSlot":   latest.Slot,
		"averageBlockSlots": avgSlots,
		"peerCount":         s.peer.PeerCount(),
		"mempoolDepth":      s.peer.MempoolSize(),
		"validatorStake":    s.peer.ValidatorStake(),
		"txCount":           s.peer.TxHistoryCount(),
		"acceptedBlocks":    s.peer.AcceptedBlocks(),
		"minedBlocks":       s.peer.MinedBlocks(),
		"lotteryAttempts":   s.peer.LotteryAttempts(),
		"lotteryWins":       s.peer.LotteryWins(),
		"lotteryMisses":     s.peer.LotteryMisses(),
		"lotteryNoTicket":   s.peer.LotteryNoTicket(),
		"uptimeSeconds":     s.peer.UptimeSeconds(),
	})
}

func (s *OpsServer) chainInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	idx := BuildExplorerIndex(s.peer)
	latest := idx.BlocksByNumber[idx.LatestHeight]
	supply := s.peer.SupplySnapshot(s.reservedSupplyAddresses)
	validators := s.peer.ActiveAuthorizedPeers()
	avgSlots := s.peer.AverageRecentBlockSlots(32)
	body := map[string]any{
		"network": map[string]any{
			"name":             "PokoinPoS",
			"shortName":        "pokoinpos",
			"chainId":          s.evmChainID,
			"networkId":        s.evmNetworkID,
			"slip44":           60,
			"networkType":      "permissioned-proof-of-stake",
			"environment":      "mainnet",
			"public":           true,
			"evmCompatible":    true,
			"consensus":        "permissioned-pos",
			"clientVersion":    nodeVersion,
			"nativeAsset":      "PKN",
			"canonicalRpcPath": "/rpc",
		},
		"nativeCurrency": map[string]any{
			"name":     "Pokoin",
			"symbol":   "PKN",
			"decimals": 18,
		},
		"rpc": map[string]any{
			"http":        "https://rpc.pokoin.com/rpc",
			"chainIdHex":  hexQuantity(big.NewInt(s.evmChainID)),
			"gasPriceWei": evmGasPrice(),
			"gasLimit":    evmGasLimit(),
		},
		"explorer": map[string]any{
			"baseUrl":          "https://explorer.pokoin.com",
			"blocksPath":       "/explorer/blocks",
			"transactionsPath": "/explorer/tx/{hash}",
			"addressesPath":    "/explorer/address/{address}",
		},
		"status": map[string]any{
			"ready":             s.peer.Ready(),
			"height":            s.peer.ChainHeight(),
			"committedHeight":   s.peer.CommittedHeight(),
			"finalityDepth":     s.peer.FinalityDepth(),
			"peerCount":         s.peer.PeerCount(),
			"mempoolDepth":      s.peer.MempoolSize(),
			"txCount":           s.peer.TxHistoryCount(),
			"uptimeSeconds":     s.peer.UptimeSeconds(),
			"averageBlockSlots": avgSlots,
		},
		"supply": supply,
		"consensus": map[string]any{
			"type":                    "permissioned-proof-of-stake",
			"slotSeconds":             1,
			"finalityDepth":           s.peer.FinalityDepth(),
			"validatorCount":          len(validators),
			"connectedValidatorCount": len(validators),
			"blockReward":             account.MINER_BLOCK_REWARD,
		},
		"latestBlock": latest,
		"validators":  validators,
		"integrations": map[string]any{
			"totalSupplyText":       "https://rpc.pokoin.com/chain/supply/total.txt",
			"circulatingSupplyText": "https://rpc.pokoin.com/chain/supply/circulating.txt",
			"prometheusMetrics":     "https://rpc.pokoin.com/metrics",
			"endpointCatalog":       "https://rpc.pokoin.com/endpoints",
		},
	}
	writeJSONAny(w, http.StatusOK, body)
}

func (s *OpsServer) chainSupply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSONAny(w, http.StatusOK, s.peer.SupplySnapshot(s.reservedSupplyAddresses))
}

func (s *OpsServer) totalSupplyText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	s.writeSupplyText(w, s.peer.SupplySnapshot(s.reservedSupplyAddresses).TotalSupply)
}

func (s *OpsServer) circulatingSupplyText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	s.writeSupplyText(w, s.peer.SupplySnapshot(s.reservedSupplyAddresses).CirculatingSupply)
}

func (s *OpsServer) chainNFTs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chain/nfts"), "/")
	if strings.HasPrefix(path, "owner/") {
		owner := strings.TrimPrefix(path, "owner/")
		tokens := s.peer.BestNFTsByOwner(owner)
		sortNFTTokens(tokens)
		writeJSONAny(w, http.StatusOK, map[string]any{"owner": owner, "count": len(tokens), "tokens": tokens})
		return
	}
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "nft_not_found"})
			return
		}
		token, exists := s.peer.BestNFT(parts[0], parts[1])
		if !exists || token.Burned {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "nft_not_found"})
			return
		}
		writeJSONAny(w, http.StatusOK, token)
		return
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if owner != "" {
		tokens := s.peer.BestNFTsByOwner(owner)
		sortNFTTokens(tokens)
		writeJSONAny(w, http.StatusOK, map[string]any{"owner": owner, "count": len(tokens), "tokens": tokens})
		return
	}
	tokens := make([]account.NFTToken, 0)
	for _, token := range s.peer.BestNFTs() {
		if !token.Burned {
			tokens = append(tokens, token)
		}
	}
	sortNFTTokens(tokens)
	writeJSONAny(w, http.StatusOK, map[string]any{"count": len(tokens), "tokens": tokens})
}

func sortNFTTokens(tokens []account.NFTToken) {
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].CollectionID == tokens[j].CollectionID {
			return tokens[i].TokenID < tokens[j].TokenID
		}
		return tokens[i].CollectionID < tokens[j].CollectionID
	})
}

func (s *OpsServer) swapPools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chain/swap/pools"), "/")
	if path != "" {
		pool, exists := s.peer.BestAMMPool(path)
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "pool_not_found"})
			return
		}
		writeJSONAny(w, http.StatusOK, pool)
		return
	}
	pools := make([]account.AMMPool, 0)
	for _, pool := range s.peer.BestAMMPools() {
		pools = append(pools, pool)
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i].ID < pools[j].ID })
	writeJSONAny(w, http.StatusOK, map[string]any{"count": len(pools), "pools": pools})
}

func (s *OpsServer) swapBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	address := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/chain/swap/balances/"))
	if address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_address"})
		return
	}
	writeJSONAny(w, http.StatusOK, map[string]any{"address": address, "balances": s.peer.BestAssetBalances(address)})
}

func (s *OpsServer) swapQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	poolID := strings.TrimSpace(r.URL.Query().Get("pool"))
	assetIn := strings.TrimSpace(r.URL.Query().Get("assetIn"))
	amountIn := queryInt(r, "amountIn", 0)
	pool, exists := s.peer.BestAMMPool(poolID)
	if !exists || amountIn < 1 || assetIn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_quote_request"})
		return
	}
	amountOut, ok := account.QuoteSwap(pool, assetIn, amountIn)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "quote_unavailable"})
		return
	}
	assetIn = strings.ToUpper(assetIn)
	assetOut := pool.AssetB
	reserveIn, reserveOut := pool.ReserveA, pool.ReserveB
	if assetIn == pool.AssetB {
		assetOut = pool.AssetA
		reserveIn, reserveOut = pool.ReserveB, pool.ReserveA
	}
	writeJSONAny(w, http.StatusOK, account.AMMQuote{
		PoolID:           pool.ID,
		AssetIn:          assetIn,
		AssetOut:         assetOut,
		AmountIn:         amountIn,
		AmountOut:        amountOut,
		FeeBps:           pool.FeeBps,
		ReserveIn:        reserveIn,
		ReserveOut:       reserveOut,
		PriceNumerator:   reserveOut,
		PriceDenominator: reserveIn,
	})
}

func (s *OpsServer) writeSupplyText(w http.ResponseWriter, value int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(strconv.Itoa(value)))
}

func (s *OpsServer) validators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"validators": s.peer.AuthorizedPeers(),
	})
}

func (s *OpsServer) bootstrapStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if s.bootstrap == nil {
		writeJSONAny(w, http.StatusOK, BootstrapRegistryStatus{})
		return
	}
	writeJSONAny(w, http.StatusOK, s.bootstrap.Status())
}

func (s *OpsServer) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	supply := s.peer.SupplySnapshot(s.reservedSupplyAddresses)
	activeValidators := s.peer.ActiveAuthorizedPeers()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "pokoinpos_chain_id %d\n", s.evmChainID)
	fmt.Fprintf(w, "pokoinpos_chain_height %d\n", s.peer.ChainHeight())
	fmt.Fprintf(w, "pokoinpos_committed_height %d\n", s.peer.CommittedHeight())
	fmt.Fprintf(w, "pokoinpos_finality_depth %d\n", s.peer.FinalityDepth())
	fmt.Fprintf(w, "pokoinpos_peer_count %d\n", s.peer.PeerCount())
	fmt.Fprintf(w, "pokoinpos_mempool_depth %d\n", s.peer.MempoolSize())
	fmt.Fprintf(w, "pokoinpos_validator_stake %d\n", s.peer.ValidatorStake())
	fmt.Fprintf(w, "pokoinpos_validator_count %d\n", len(activeValidators))
	fmt.Fprintf(w, "pokoinpos_connected_validator_count %d\n", len(activeValidators))
	fmt.Fprintf(w, "pokoinpos_total_supply %d\n", supply.TotalSupply)
	fmt.Fprintf(w, "pokoinpos_circulating_supply %d\n", supply.CirculatingSupply)
	fmt.Fprintf(w, "pokoinpos_reserved_supply %d\n", supply.ReservedSupply)
	fmt.Fprintf(w, "pokoinpos_average_block_slots %d\n", s.peer.AverageRecentBlockSlots(32))
	fmt.Fprintf(w, "pokoinpos_blocks_accepted_total %d\n", s.peer.AcceptedBlocks())
	fmt.Fprintf(w, "pokoinpos_blocks_mined_total %d\n", s.peer.MinedBlocks())
	fmt.Fprintf(w, "pokoinpos_lottery_attempts_total %d\n", s.peer.LotteryAttempts())
	fmt.Fprintf(w, "pokoinpos_lottery_wins_total %d\n", s.peer.LotteryWins())
	fmt.Fprintf(w, "pokoinpos_lottery_misses_total %d\n", s.peer.LotteryMisses())
	fmt.Fprintf(w, "pokoinpos_lottery_no_ticket_total %d\n", s.peer.LotteryNoTicket())
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

func (s *OpsServer) adminMempool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	txs := s.peer.MempoolSummaries()
	writeJSON(w, http.StatusOK, map[string]any{
		"count":        len(txs),
		"transactions": txs,
	})
}

type mintRequest struct {
	To     string `json:"to"`
	Amount int    `json:"amount"`
}

type withdrawRequest struct {
	To     string `json:"to"`
	Amount int    `json:"amount"`
}

type nftMintRequest struct {
	CollectionID string `json:"collectionId"`
	TokenID      string `json:"tokenId"`
	Owner        string `json:"owner"`
	MetadataURI  string `json:"metadataUri"`
	MetadataHash string `json:"metadataHash"`
	ImageURI     string `json:"imageUri"`
}

type nftTransferRequest struct {
	CollectionID string `json:"collectionId"`
	TokenID      string `json:"tokenId"`
	To           string `json:"to"`
}

type assetAccountingRequest struct {
	Asset   string `json:"asset"`
	Account string `json:"account"`
	Amount  int    `json:"amount"`
}

type swapPoolRequest struct {
	AssetA string `json:"assetA"`
	AssetB string `json:"assetB"`
}

type swapAddLiquidityRequest struct {
	PoolID  string `json:"poolId"`
	AmountA int    `json:"amountA"`
	AmountB int    `json:"amountB"`
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
	req.To = strings.TrimSpace(req.To)
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

func (s *OpsServer) withdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	req.To = strings.ToLower(strings.TrimSpace(req.To))
	if req.To == "" || req.Amount < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_withdraw_request"})
		return
	}
	tx, ok := s.peer.FloodValidatorWithdrawTransaction(fmt.Sprintf("withdraw-%d-%s-%d", time.Now().UnixNano(), req.To, req.Amount), req.To, req.Amount)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "withdraw_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"hash":   tx.ID,
		"to":     tx.To,
		"amount": tx.Amount,
	})
}

func (s *OpsServer) nftMint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req nftMintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	req.TokenID = strings.TrimSpace(req.TokenID)
	req.Owner = strings.TrimSpace(req.Owner)
	if req.CollectionID == "" || req.TokenID == "" || req.Owner == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_nft_mint_request"})
		return
	}
	id := account.StableNFTTxID("mint", time.Now().UTC().Format(time.RFC3339Nano), req.CollectionID, req.TokenID, req.Owner, req.MetadataURI, req.MetadataHash, req.ImageURI)
	tx, ok := s.peer.FloodNFTMintTransaction(id, req.CollectionID, req.TokenID, req.Owner, req.MetadataURI, req.MetadataHash, req.ImageURI)
	if !ok || tx.NFT == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "nft_mint_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"hash":         tx.ID,
		"collectionId": tx.NFT.CollectionID,
		"tokenId":      tx.NFT.TokenID,
		"owner":        tx.NFT.Owner,
		"metadataUri":  tx.NFT.MetadataURI,
		"metadataHash": tx.NFT.MetadataHash,
		"imageUri":     tx.NFT.ImageURI,
	})
}

func (s *OpsServer) nftTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req nftTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	req.TokenID = strings.TrimSpace(req.TokenID)
	req.To = strings.TrimSpace(req.To)
	if req.CollectionID == "" || req.TokenID == "" || req.To == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_nft_transfer_request"})
		return
	}
	id := account.StableNFTTxID("transfer", time.Now().UTC().Format(time.RFC3339Nano), req.CollectionID, req.TokenID, req.To)
	tx, ok := s.peer.FloodNFTTransferTransaction(id, req.CollectionID, req.TokenID, req.To)
	if !ok || tx.NFT == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "nft_transfer_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"hash":         tx.ID,
		"collectionId": tx.NFT.CollectionID,
		"tokenId":      tx.NFT.TokenID,
		"to":           tx.To,
	})
}

func (s *OpsServer) adminAssetCredit(w http.ResponseWriter, r *http.Request) {
	s.adminAssetAccounting(w, r, true)
}

func (s *OpsServer) adminAssetDebit(w http.ResponseWriter, r *http.Request) {
	s.adminAssetAccounting(w, r, false)
}

func (s *OpsServer) adminAssetAccounting(w http.ResponseWriter, r *http.Request, credit bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req assetAccountingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	req.Asset = strings.TrimSpace(req.Asset)
	req.Account = strings.TrimSpace(req.Account)
	if req.Asset == "" || req.Account == "" || req.Amount < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_asset_request"})
		return
	}
	idKind := "asset-credit"
	var tx *account.SignedTransaction
	var ok bool
	if credit {
		tx, ok = s.peer.FloodAssetCreditTransaction(fmt.Sprintf("%s-%d-%s-%s-%d", idKind, time.Now().UnixNano(), req.Asset, req.Account, req.Amount), req.Asset, req.Account, req.Amount)
	} else {
		idKind = "asset-debit"
		tx, ok = s.peer.FloodAssetDebitTransaction(fmt.Sprintf("%s-%d-%s-%s-%d", idKind, time.Now().UnixNano(), req.Asset, req.Account, req.Amount), req.Asset, req.Account, req.Amount)
	}
	if !ok || tx.AMM == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "asset_accounting_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"hash": tx.ID, "asset": tx.AMM.Asset, "account": tx.To, "amount": tx.AMM.Amount, "action": tx.AMM.Action})
}

func (s *OpsServer) adminSwapPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req swapPoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	poolID := account.PoolID(req.AssetA, req.AssetB)
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_pool_request"})
		return
	}
	tx, ok := s.peer.FloodAMMCreatePoolTransaction(fmt.Sprintf("amm-pool-%d-%s", time.Now().UnixNano(), poolID), req.AssetA, req.AssetB)
	if !ok || tx.AMM == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "pool_create_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"hash": tx.ID, "poolId": tx.AMM.PoolID, "assetA": tx.AMM.AssetA, "assetB": tx.AMM.AssetB})
}

func (s *OpsServer) adminSwapAddLiquidity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	var req swapAddLiquidityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	if strings.TrimSpace(req.PoolID) == "" || req.AmountA < 1 || req.AmountB < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_liquidity_request"})
		return
	}
	tx, ok := s.peer.FloodAMMAddLiquidityTransaction(fmt.Sprintf("amm-liquidity-%d-%s-%d-%d", time.Now().UnixNano(), req.PoolID, req.AmountA, req.AmountB), req.PoolID, req.AmountA, req.AmountB)
	if !ok || tx.AMM == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "liquidity_add_failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"hash": tx.ID, "poolId": tx.AMM.PoolID, "amountA": tx.AMM.AmountA, "amountB": tx.AMM.AmountB})
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
