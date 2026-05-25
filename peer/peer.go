package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"peer/account"
	"peer/helpers"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Peer struct {
	listenport int
	id         string
	conns      map[string]net.Conn
	// peerValidators maps network peer IDs to their advertised validator account.
	peerValidators map[string]string
	// peerAddresses maps peer IDs to advertised reachable TCP endpoints.
	peerAddresses map[string]PeerAddress
	// advertiseHost is the externally reachable host this peer announces.
	advertiseHost string
	// output serializes logging to stdout.
	output chan string
	// verbose controls per-message logs.
	// Kept false so terminal output stays TA-friendly (ledger-focused, not ping-pong spam).
	verbose bool
	// received forwards messages to the demo WaitGroup.
	received chan Message
	lock     sync.Mutex
	// sendLock prevents interleaved writes on a connection.
	sendLock sync.Mutex
	// Map of messages this has sent on (key is message ID).
	msgHistory map[string]MessageHistory
	// Lock for flooding mechanisms while handling reception of a message from the network.
	floodingLock sync.Mutex
	// Ledger and transaction.
	ledger *account.Ledger
	// Blockchain state for Exercise 16.2 (PoS total-order).
	blockchain *account.Blockchain
	genesis    *account.GenesisMetaData
	miner      *account.Account
	// Mempool of valid txs waiting for inclusion in a block.
	mempool     map[string]account.SignedTransaction
	mempoolLock sync.Mutex
	// Number of blocks to keep tentative before finalizing state-machine execution.
	finalityDepth int
	// Runtime counters for operations and observability.
	startedAt       time.Time
	acceptedBlocks  atomic.Uint64
	minedBlocks     atomic.Uint64
	acceptedTxs     atomic.Uint64
	lotteryAttempts atomic.Uint64
	lotteryWins     atomic.Uint64
	lotteryMisses   atomic.Uint64
	lotteryNoTicket atomic.Uint64
}

type MessageHistory struct {
	content      *Message // The actual message.
	receivedFrom []string // List of IDs from all peers this peer has received this message from.
	isSent       bool     // Has this peer already sent this message.
}

type connectPayload struct {
	Peers       []string      `json:"peers"`
	Addresses   []PeerAddress `json:"addresses,omitempty"`
	Address     string        `json:"address,omitempty"`
	Validator   string        `json:"validator,omitempty"`
	GenesisHash string        `json:"genesisHash,omitempty"`
	Height      int           `json:"height,omitempty"`
	BestHash    string        `json:"bestHash,omitempty"`
}

type chainSyncRequest struct {
	GenesisHash string `json:"genesisHash"`
	FromHeight  int    `json:"fromHeight"`
}

type chainSyncResponse struct {
	GenesisHash string          `json:"genesisHash"`
	Height      int             `json:"height"`
	Blocks      []account.Block `json:"blocks"`
}

type chainStatusSummary struct {
	GenesisHash string
	Height      int
	BestHash    string
}

type PeerAddress struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

func NewMessageHistory(msg *Message) *MessageHistory {
	hist := new(MessageHistory)
	hist.content = msg
	hist.receivedFrom = make([]string, 0)
	hist.isSent = false
	return hist
}

// Create a new Peer object.
func NewPeer(listenport int) *Peer {
	peer := new(Peer)
	peer.id = strconv.Itoa(listenport)
	peer.listenport = listenport
	peer.conns = make(map[string]net.Conn)
	peer.peerValidators = make(map[string]string)
	peer.peerAddresses = make(map[string]PeerAddress)
	peer.output = make(chan string, 32)
	peer.received = make(chan Message, 32)
	peer.msgHistory = make(map[string]MessageHistory)
	peer.verbose = false
	// Ledger must be initialized or Transaction() would panic on nil map.
	peer.ledger = account.MakeLedger()
	peer.mempool = make(map[string]account.SignedTransaction)
	peer.startedAt = time.Now().UTC()
	peer.finalityDepth = 0
	return peer
}

func (p *Peer) SetAdvertiseHost(host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}
	p.lock.Lock()
	oldID := p.id
	newID := peerID(host, p.listenport)
	if newID != oldID {
		p.rekeyLocked(oldID, newID)
	}
	p.advertiseHost = host
	p.peerAddresses[p.id] = PeerAddress{ID: p.id, Host: host, Port: p.listenport}
	p.lock.Unlock()
}

func peerID(host string, port int) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return strconv.Itoa(port)
	}
	sum := sha256.Sum256([]byte(net.JoinHostPort(host, strconv.Itoa(port))))
	return "peer-" + base64.RawURLEncoding.EncodeToString(sum[:16])
}

func (p *Peer) rekeyLocked(oldID string, newID string) {
	if oldID == "" || newID == "" || oldID == newID {
		return
	}
	p.id = newID
	if conn, exists := p.conns[oldID]; exists {
		delete(p.conns, oldID)
		p.conns[newID] = conn
	}
	if validator, exists := p.peerValidators[oldID]; exists {
		delete(p.peerValidators, oldID)
		p.peerValidators[newID] = validator
	}
	if address, exists := p.peerAddresses[oldID]; exists {
		delete(p.peerAddresses, oldID)
		address.ID = newID
		p.peerAddresses[newID] = address
	}
}

// ConfigurePoS initializes blockchain state for Exercise 16.2.
// All peers must receive the same genesis metadata.
func (p *Peer) ConfigurePoS(genesis *account.GenesisMetaData, miner *account.Account) {
	p.genesis = genesis
	p.miner = miner
	if miner != nil {
		p.lock.Lock()
		p.peerValidators[p.id] = miner.SafeEncode()
		p.lock.Unlock()
	}
	p.blockchain = account.NewBlockchainWithGenesis(genesis)
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
}

func (p *Peer) SetFinalityDepth(depth int) {
	if depth < 0 {
		depth = 0
	}
	p.finalityDepth = depth
	if p.blockchain != nil {
		p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	}
}

// Start a peer and try to connect to port.
func (p *Peer) StartWithConnection(addr string, port int) {
	p.Start()
	peers, err := p.Connect(addr, port)
	if err != nil { // If connection fails, then there is no network.
		return // Do not propagate error. This peer starts its own network, so OK.
	}
	// Otherwise, a connection message was sent, and remaining connections can be established.
	p.joinNetwork(addr, peers)
	p.joinKnownPeers(p.knownPeerAddresses())
	// Tell the whole network we joined (so peers that don't have us yet can add us).
	p.floodJoin()
}

// Start listening on the port.
func (p *Peer) Start() error {
	// Listen for connection.
	listener, err := net.Listen(helpers.PROTOCOL, ":"+strconv.Itoa(p.listenport))
	if err != nil {
		return err
	}
	// Add self to map of connections.
	p.lock.Lock()
	p.conns[p.id] = nil
	p.lock.Unlock()
	// Goroutine to listen to any number of peers.
	go p.listenForPeers(listener)
	// Print logs from a single goroutine to avoid interleaving.
	go p.printOutput()
	return nil
}

// Listen for peers. When connection is established, prepare communication with them.
func (p *Peer) listenForPeers(listener net.Listener) {
	// Defer ensures listener is closed when this goroutine returns.
	defer listener.Close()
	for {
		// Wait to establish connection.
		conn, err := listener.Accept()
		if err != nil {
			p.logf("accept failed: %v", err)
			continue
		}
		// Prepare network communication with the new connection. We are the server (acceptor).
		p.prepareConnection(conn, true)
	}
}

// Prepare communication with a peer. fromAccept is true when we accepted the connection (we are "server").
func (p *Peer) prepareConnection(conn net.Conn, fromAccept bool) ([]PeerAddress, error) {
	reader := bufio.NewReader(conn)
	// Handshake: announce our id, validator identity, and known peers.
	payload, _ := json.Marshal(p.connectPayload())
	_ = p.writeMessage(conn, &Message{Type: helpers.CONNECT_MESSAGE_TYPE, From: p.id, Payload: payload})
	// Wait for reply to establish their name.
	msg, err := p.readMessage(reader)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	p.lock.Lock()
	p.conns[msg.From] = conn
	p.lock.Unlock()
	p.recordPeerAddress(addressFromConnectPayload(msg.From, msg.Payload, conn.RemoteAddr()))
	p.recordPeerValidator(msg.From, validatorFromConnectPayload(msg.Payload))
	go p.handleDecode(msg.From, conn, reader)
	p.maybeRequestChainSync(msg.From, msg.Payload)
	// Catch-up: when we are the server (we accepted), send the new peer all messages we already have
	// so they get the same ledger and message history as the rest of the network.
	if fromAccept {
		p.sendCatchUp(conn)
	}
	// Unmarshal the received list of peers the connection knew about.
	peers := p.addressesFromConnectPayload(msg.Payload)
	return peers, nil
}

// Wait for messages from connection.
func (p *Peer) handleDecode(peerID string, conn net.Conn, reader *bufio.Reader) {
	// Defer ensures cleanup when the reader loop ends.
	defer func() {
		_ = conn.Close()
		p.lock.Lock()
		// Only remove this peer entry if it still points to this exact connection.
		// Multiple reconnects can coexist; deleting unconditionally may drop a newer connection.
		if peerID != p.id {
			current, exists := p.conns[peerID]
			if exists && current == conn {
				delete(p.conns, peerID)
			}
		}
		p.lock.Unlock()
	}()
	for {
		// Wait until receving a message from the connection connected to this decoder.
		msg, err := p.readMessage(reader)
		if err != nil {
			return
		}
		// Do something with the message.
		p.OnMessage(msg.From, msg)
	}
}

// Handle messages received.
func (p *Peer) OnMessage(from string, msg *Message) {
	switch msg.Type {
	case helpers.PING_MESSAGE_TYPE:
		p.logf("Peer %s sending Pong (MsgID: %s) to Peer %s", p.id, msg.MsgID, from)
		p.Send(from, &Message{Type: helpers.PONG_MESSAGE_TYPE, MsgID: msg.MsgID, From: p.id})
		p.received <- *msg
		return // Do not flood ping messages.
	case helpers.TRANSACTION_MESSAGE_TYPE:
		// Critical for convergence: apply each Tx exactly once per peer.
		// If this returns false, this delivery is a duplicate and must be ignored.
		if !p.handleTransaction(msg) {
			return
		}
	case helpers.POS_TRANSACTION_MESSAGE_TYPE:
		// PoS mode: receive transaction into mempool (do not apply directly).
		if !p.handlePoSTransaction(msg) {
			return
		}
	case helpers.BLOCK_MESSAGE_TYPE:
		// PoS mode: validate/add block, then rebuild ledger from best chain.
		if !p.handleBlock(msg) {
			return
		}
	case helpers.CHAIN_SYNC_REQUEST_MESSAGE_TYPE:
		p.handleChainSyncRequest(from, msg)
		return
	case helpers.CHAIN_SYNC_RESPONSE_MESSAGE_TYPE:
		if !p.handleChainSyncResponse(msg) {
			return
		}
		return
	case helpers.JOIN_MESSAGE_TYPE:
		// Another peer joined the network; if we don't know them yet, connect so we stay fully connected.
		p.handleJoin(msg)
	}
	// Remember we got this message (for dedup) and forward it to neighbours.
	p.addReceivedFloodMessage(msg)
	p.FloodNetwork(msg)
	// Keep received-channel traffic only for message types used by legacy tests.
	// Join/connect chatter can otherwise fill the channel and stall decode goroutines in the handin demo.
	if msg.Type == helpers.TRANSACTION_MESSAGE_TYPE || msg.Type == "Test-flood-message" {
		p.received <- *msg
	}
}

// Note this message as beeing received (should be used before calling FloodNetwork on a message).
func (p *Peer) addReceivedFloodMessage(msg *Message) {
	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	var hist MessageHistory
	hist, exists := p.msgHistory[msg.MsgID]

	if !exists { // If the msg has never been received before, create a new history for it.
		hist = *NewMessageHistory(msg)
	}
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist
}

// Flood a message across the network.
func (p *Peer) FloodNetwork(msg *Message) {
	p.floodingLock.Lock()
	hist, _ := p.msgHistory[msg.MsgID]

	if hist.isSent { // If it did exist and was already sent, abort.
		p.floodingLock.Unlock()
		return
	}
	// Set the message to be sent.
	hist.isSent = true
	p.msgHistory[msg.MsgID] = hist
	p.floodingLock.Unlock()
	p.logf("Peer %s flooding %s (MsgID: %s)", p.id, msg.Type, msg.MsgID)
	// Change the sender, so that others are aware they received this message version from this peer.
	outbound := *msg
	outbound.From = p.id
	conns := p.connectionSnapshot()
	// Send to all peers it did not receive the message from (and also not itself).
	for peer, conn := range conns {
		if peer != p.id && !slices.Contains(hist.receivedFrom, peer) {
			if err := p.writeMessage(conn, &outbound); err != nil {
				fmt.Printf("writeMessage failed from=%s to=%s type=%s err=%v\n", p.id, peer, msg.Type, err)
			}
		}
	}
}

func (p *Peer) printOutput() {
	for msg := range p.output {
		fmt.Println(msg)
	}
}

// Connect to another peer.
func (p *Peer) Connect(addr string, port int) ([]string, error) {
	addresses, err := p.ConnectAddress(PeerAddress{Host: addr, Port: port})
	if err != nil {
		return nil, err
	}
	peers := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.ID != "" {
			peers = append(peers, address.ID)
		}
		p.recordPeerAddress(address)
	}
	return peers, nil
}

func (p *Peer) ConnectAddress(address PeerAddress) ([]PeerAddress, error) {
	if address.Host == "" || address.Port <= 0 {
		return nil, fmt.Errorf("invalid peer address %s:%d", address.Host, address.Port)
	}
	conn, err := net.DialTimeout(helpers.PROTOCOL, net.JoinHostPort(address.Host, strconv.Itoa(address.Port)), 5*time.Second)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	// We are the client (joiner), so fromAccept is false (no catch-up from our side).
	peers, err := p.prepareConnection(conn, false)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return peers, nil
}

// Connect to a list of peers (may be known, in which case, it does notrhing).
func (p *Peer) joinNetwork(addr string, peers []string) {
	for _, peer := range peers {
		if !p.hasConnection(peer) {
			port, err := strconv.Atoi(peer)
			if err != nil {
				continue
			}
			_, _ = p.Connect(addr, port)
		}
	}
}

func (p *Peer) joinKnownPeers(addresses []PeerAddress) {
	for _, address := range addresses {
		if address.ID == "" || address.ID == p.id || p.hasConnection(address.ID) {
			continue
		}
		p.recordPeerAddress(address)
		_, _ = p.ConnectAddress(address)
	}
}

// floodJoin announces to the network that we joined, so others can add us to their peer set.
func (p *Peer) floodJoin() {
	payload, _ := json.Marshal(p.connectPayload())
	joinMsg := &Message{
		Type:    helpers.JOIN_MESSAGE_TYPE,
		MsgID:   "join-" + p.id,
		From:    p.id,
		Payload: payload,
	}
	// Put in history first so we (and catch-up) have the content.
	p.addReceivedFloodMessage(joinMsg)
	p.FloodNetwork(joinMsg)
}

// sendCatchUp sends to conn all messages we have already seen (so a new peer gets the same ledger).
func (p *Peer) sendCatchUp(conn net.Conn) {
	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	for _, hist := range p.msgHistory {
		// Only send if we stored the message content (we do for received and for floodJoin).
		if hist.content != nil {
			_ = p.writeMessage(conn, hist.content)
		}
	}
}

func (p *Peer) maybeRequestChainSync(peerID string, raw []byte) {
	var payload connectPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	local := p.chainStatus()
	if payload.GenesisHash == "" || local.GenesisHash == "" {
		return
	}
	if payload.GenesisHash != local.GenesisHash && !p.canAdoptRemoteGenesis() {
		logEvent("chain_sync_genesis_mismatch", map[string]any{
			"severity":      "critical",
			"peerID":        peerID,
			"localGenesis":  local.GenesisHash,
			"remoteGenesis": payload.GenesisHash,
			"localHeight":   local.Height,
			"remoteHeight":  payload.Height,
		})
		return
	}
	if payload.Height <= local.Height {
		return
	}
	fromHeight := local.Height + 1
	if payload.GenesisHash != local.GenesisHash && p.canAdoptRemoteGenesis() {
		fromHeight = 0
	}
	req := chainSyncRequest{
		GenesisHash: payload.GenesisHash,
		FromHeight:  fromHeight,
	}
	p.logf("chain sync requesting from %s: localHeight=%d remoteHeight=%d fromHeight=%d", peerID, local.Height, payload.Height, fromHeight)
	p.requestChainSync(peerID, req)
}

func (p *Peer) requestChainSync(peerID string, req chainSyncRequest) {
	body, _ := json.Marshal(req)
	_ = p.Send(peerID, &Message{
		Type:    helpers.CHAIN_SYNC_REQUEST_MESSAGE_TYPE,
		MsgID:   fmt.Sprintf("sync-req-%s-%d", p.id, time.Now().UnixNano()),
		From:    p.id,
		Payload: body,
	})
}

func (p *Peer) RequestChainSyncFromPeers() {
	local := p.chainStatus()
	if local.GenesisHash == "" {
		return
	}
	for peerID, conn := range p.connectionSnapshot() {
		if peerID == p.id || conn == nil {
			continue
		}
		p.requestChainSync(peerID, chainSyncRequest{
			GenesisHash: local.GenesisHash,
			FromHeight:  local.Height + 1,
		})
	}
}

func (p *Peer) handleChainSyncRequest(from string, msg *Message) {
	if p.blockchain == nil {
		return
	}
	var req chainSyncRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return
	}
	local := p.chainStatus()
	if req.GenesisHash == "" || req.GenesisHash != local.GenesisHash {
		logEvent("chain_sync_request_genesis_mismatch", map[string]any{
			"severity":         "critical",
			"peerID":           from,
			"localGenesis":     local.GenesisHash,
			"requestedGenesis": req.GenesisHash,
			"localHeight":      local.Height,
		})
		return
	}
	blocks := p.blockchain.CanonicalBlocks(0)
	if req.FromHeight < 0 {
		req.FromHeight = 0
	}
	if req.FromHeight > len(blocks) {
		req.FromHeight = len(blocks)
	}
	p.logf("chain sync serving %d blocks to %s fromHeight=%d height=%d", len(blocks[req.FromHeight:]), from, req.FromHeight, local.Height)
	resp := chainSyncResponse{
		GenesisHash: local.GenesisHash,
		Height:      local.Height,
		Blocks:      blocks[req.FromHeight:],
	}
	body, _ := json.Marshal(resp)
	_ = p.Send(from, &Message{
		Type:    helpers.CHAIN_SYNC_RESPONSE_MESSAGE_TYPE,
		MsgID:   fmt.Sprintf("sync-resp-%s-%d", p.id, time.Now().UnixNano()),
		From:    p.id,
		Payload: body,
	})
}

func (p *Peer) handleChainSyncResponse(msg *Message) bool {
	if p.blockchain == nil {
		return false
	}
	var resp chainSyncResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return false
	}
	local := p.chainStatus()
	if resp.GenesisHash == "" {
		return false
	}
	if resp.GenesisHash != local.GenesisHash && !p.canAdoptRemoteGenesis() {
		logEvent("chain_sync_response_genesis_mismatch", map[string]any{
			"severity":      "critical",
			"peerID":        msg.From,
			"localGenesis":  local.GenesisHash,
			"remoteGenesis": resp.GenesisHash,
			"localHeight":   local.Height,
			"remoteHeight":  resp.Height,
		})
		return false
	}
	if resp.Height <= local.Height || len(resp.Blocks) == 0 {
		return false
	}
	p.logf("chain sync received %d blocks from %s remoteHeight=%d localHeight=%d", len(resp.Blocks), msg.From, resp.Height, local.Height)
	if resp.GenesisHash != local.GenesisHash {
		if len(resp.Blocks) != resp.Height+1 {
			logEvent("chain_sync_full_response_invalid", map[string]any{
				"severity":     "critical",
				"peerID":       msg.From,
				"remoteHeight": resp.Height,
				"blockCount":   len(resp.Blocks),
			})
			return false
		}
		if !p.adoptRemoteChain(resp.Blocks) {
			return false
		}
		p.logf("chain sync adopted remote genesis from %s: height %d", msg.From, p.ChainHeight())
		return true
	}
	if len(resp.Blocks) == resp.Height+1 {
		if !p.installFullSyncedChainFromCommonAncestor(msg.From, resp.Blocks) {
			return false
		}
		p.logf("chain sync switched to longer branch from %s: height %d -> %d", msg.From, local.Height, p.ChainHeight())
		return true
	}
	if resp.Height != local.Height+len(resp.Blocks) {
		if p.installSyncedOverlap(msg.From, resp.Blocks) {
			p.logf("chain sync applied overlapping suffix from %s: height %d -> %d", msg.From, local.Height, p.ChainHeight())
			return true
		}
		logEvent("chain_sync_suffix_height_mismatch", map[string]any{
			"severity":     "critical",
			"peerID":       msg.From,
			"localHeight":  local.Height,
			"remoteHeight": resp.Height,
			"blockCount":   len(resp.Blocks),
		})
		return false
	}
	if !p.installSyncedSuffix(msg.From, resp.GenesisHash, resp.Blocks) {
		return false
	}
	p.logf("chain sync applied from %s: height %d -> %d", msg.From, local.Height, p.ChainHeight())
	return true
}

func (p *Peer) canAdoptRemoteGenesis() bool {
	return p.blockchain != nil && p.ChainHeight() == 0 && p.TxHistoryCount() == 0 && p.AcceptedBlocks() == 0
}

func (p *Peer) adoptRemoteChain(blocks []account.Block) bool {
	if len(blocks) == 0 || p.blockchain == nil {
		return false
	}
	if blocks[0].ParentHash != account.NO_PARENT_HASH {
		logEvent("chain_sync_adopt_invalid_genesis_parent", map[string]any{
			"severity": "critical",
			"parent":   blocks[0].ParentHash,
		})
		return false
	}
	for i := 1; i < len(blocks); i++ {
		if blocks[i].ParentHash != blockHashString(blocks[i-1]) {
			logEvent("chain_sync_adopt_non_contiguous", map[string]any{
				"severity": "critical",
				"index":    i,
			})
			return false
		}
	}
	genesis, err := account.DecodeGenesisFromBlock(blocks[0])
	if err != nil {
		return false
	}
	p.genesis = genesis
	p.blockchain.ReplaceBlocks(blocks)
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	return true
}

func (p *Peer) installSyncedSuffix(peerID string, genesisHash string, blocks []account.Block) bool {
	localBlocks := p.blockchain.CanonicalBlocks(0)
	if len(localBlocks) == 0 || len(blocks) == 0 {
		return false
	}
	expectedParent := blockHashString(localBlocks[len(localBlocks)-1])
	if blocks[0].ParentHash != expectedParent {
		logEvent("chain_sync_suffix_parent_mismatch", map[string]any{
			"severity":       "critical",
			"peerID":         peerID,
			"firstParent":    blocks[0].ParentHash,
			"expectedParent": expectedParent,
			"localHeight":    len(localBlocks) - 1,
		})
		p.requestChainSync(peerID, chainSyncRequest{GenesisHash: genesisHash, FromHeight: 0})
		return false
	}
	merged := append([]account.Block{}, localBlocks...)
	for i, block := range blocks {
		if i > 0 && block.ParentHash != blockHashString(blocks[i-1]) {
			logEvent("chain_sync_suffix_non_contiguous", map[string]any{
				"severity": "critical",
				"index":    i,
			})
			return false
		}
		merged = append(merged, block)
	}
	p.blockchain.ReplaceBlocks(merged)
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	return true
}

func (p *Peer) installSyncedOverlap(peerID string, blocks []account.Block) bool {
	localBlocks := p.blockchain.CanonicalBlocks(0)
	if len(localBlocks) == 0 || len(blocks) == 0 {
		return false
	}
	localHashes := make(map[string]int, len(localBlocks))
	for i, block := range localBlocks {
		localHashes[blockHashString(block)] = i
	}
	commonLocalIndex := -1
	commonRemoteIndex := -1
	for i, block := range blocks {
		if i > 0 && block.ParentHash != blockHashString(blocks[i-1]) {
			logEvent("chain_sync_overlap_non_contiguous", map[string]any{
				"severity": "critical",
				"peerID":   peerID,
				"index":    i,
			})
			return false
		}
		if localIndex, ok := localHashes[blockHashString(block)]; ok {
			commonLocalIndex = localIndex
			commonRemoteIndex = i
		}
	}
	if commonRemoteIndex < 0 || commonRemoteIndex+1 >= len(blocks) {
		return false
	}
	if len(localBlocks[:commonLocalIndex+1])+len(blocks[commonRemoteIndex+1:]) <= len(localBlocks) {
		return false
	}
	replacement := append([]account.Block{}, localBlocks[:commonLocalIndex+1]...)
	replacement = append(replacement, blocks[commonRemoteIndex+1:]...)
	p.blockchain.ReplaceBlocks(replacement)
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	return true
}

func (p *Peer) installFullSyncedChainFromCommonAncestor(peerID string, blocks []account.Block) bool {
	localBlocks := p.blockchain.CanonicalBlocks(0)
	if len(localBlocks) == 0 || len(blocks) <= len(localBlocks) {
		return false
	}
	localHashes := make(map[string]int, len(localBlocks))
	for i, block := range localBlocks {
		localHashes[blockHashString(block)] = i
	}
	if genesisContentHash(blocks[0]) != genesisContentHash(localBlocks[0]) {
		logEvent("chain_sync_full_replacement_refused", map[string]any{
			"severity": "critical",
			"peerID":   peerID,
			"reason":   "genesis_content_mismatch",
		})
		return false
	}
	commonRemoteIndex := -1
	commonLocalIndex := -1
	for i, block := range blocks {
		if localIndex, ok := localHashes[blockHashString(block)]; ok {
			commonRemoteIndex = i
			commonLocalIndex = localIndex
		}
	}
	if commonRemoteIndex <= 0 || commonLocalIndex <= 0 {
		logEvent("chain_sync_full_replacement_refused", map[string]any{
			"severity":     "critical",
			"peerID":       peerID,
			"localHeight":  len(localBlocks) - 1,
			"remoteHeight": len(blocks) - 1,
			"reason":       "no_non_genesis_common_ancestor",
		})
		return false
	}
	for i := 1; i < len(blocks); i++ {
		if blocks[i].ParentHash != blockHashString(blocks[i-1]) {
			logEvent("chain_sync_full_non_contiguous", map[string]any{
				"severity": "critical",
				"peerID":   peerID,
				"index":    i,
			})
			return false
		}
	}
	replacement := append([]account.Block{}, localBlocks[:commonLocalIndex+1]...)
	replacement = append(replacement, blocks[commonRemoteIndex+1:]...)
	p.blockchain.ReplaceBlocks(replacement)
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	return true
}

// handleJoin adds the new peer from a Join message to our peer set and connects if we didn't know them.
func (p *Peer) handleJoin(msg *Message) {
	newPeerID := msg.From
	for _, address := range p.addressesFromConnectPayload(msg.Payload) {
		p.recordPeerAddress(address)
	}
	p.recordPeerValidator(newPeerID, validatorFromConnectPayload(msg.Payload))
	if p.hasConnection(newPeerID) {
		return
	}
	for _, address := range p.addressesFromConnectPayload(msg.Payload) {
		if address.ID == newPeerID {
			_, _ = p.ConnectAddress(address)
			return
		}
	}
	// Backwards-compatible fallback for local tests using port-only peer IDs.
	if port, err := strconv.Atoi(newPeerID); err == nil {
		_, _ = p.Connect("localhost", port)
	}
}

// Send a message to another peer.
func (p *Peer) Send(to string, msg *Message) error {
	// Try to find the connection connected to the receiver of this message.
	conn := p.connection(to)
	if conn == nil {
		return fmt.Errorf("Connection not found for receiver %s!", to)
	}
	// Send the message.
	return p.writeMessage(conn, msg)
}

// Marshal message and send it to the connection.
func (p *Peer) writeMessage(conn net.Conn, msg *Message) error {
	// Encode JSON to a buffer to compute length prefix.
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(msg); err != nil {
		return err
	}
	data := buf.Bytes()
	header := make([]byte, helpers.MESSAGE_HEADER_SIZE)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	// Write header + payload atomically per connection.
	p.sendLock.Lock()
	defer p.sendLock.Unlock()
	if _, err := conn.Write(header); err != nil {
		return err
	}
	if _, err := conn.Write(data); err != nil {
		return err
	}
	return nil
}

// Unmarshal message received from a connection.
func (p *Peer) readMessage(reader *bufio.Reader) (*Message, error) {
	// Read length prefix, then exact JSON payload.
	var header [helpers.MESSAGE_HEADER_SIZE]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length == 0 {
		return nil, fmt.Errorf("invalid message length 0")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	var msg Message
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// FloodMessage sends a message to all peers (with dedup). Same as FloodNetwork; name from exercise.
func (p *Peer) FloodMessage(msg *Message) {
	p.FloodNetwork(msg)
}

// FloodPoSTransaction broadcasts a transaction for block inclusion in PoS mode.
func (p *Peer) FloodPoSTransaction(tx *account.SignedTransaction) {
	payload, err := json.Marshal(&tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	msg := &Message{Type: helpers.POS_TRANSACTION_MESSAGE_TYPE, MsgID: tx.ID, From: p.id, Payload: payload}
	if !p.handlePoSTransaction(msg) {
		return
	}
	p.FloodNetwork(msg)
}

func (p *Peer) FloodMintTransaction(id string, to string, amount int) (*account.SignedTransaction, bool) {
	if p.miner == nil || amount < 1 || strings.TrimSpace(to) == "" {
		return nil, false
	}
	tx := account.NewMintTransaction(id, p.miner, strings.TrimSpace(to), amount)
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodValidatorWithdrawTransaction(id string, to string, amount int) (*account.SignedTransaction, bool) {
	if p.miner == nil || amount < 1 || strings.TrimSpace(to) == "" {
		return nil, false
	}
	to = strings.ToLower(strings.TrimSpace(to))
	if !strings.HasPrefix(to, "0x") || len(to) != 42 {
		return nil, false
	}
	tx := account.NewSignedTransaction(id, p.miner, to, amount)
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodNFTMintTransaction(id string, collectionID string, tokenID string, owner string, metadataURI string, metadataHash string, imageURI string) (*account.SignedTransaction, bool) {
	if p.miner == nil || strings.TrimSpace(collectionID) == "" || strings.TrimSpace(tokenID) == "" || strings.TrimSpace(owner) == "" {
		return nil, false
	}
	tx := account.NewNFTMintTransaction(id, p.miner, collectionID, tokenID, owner, metadataURI, metadataHash, imageURI)
	if tx.NFT == nil || tx.NFT.CollectionID == "" || tx.NFT.TokenID == "" {
		return nil, false
	}
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodNFTTransferTransaction(id string, collectionID string, tokenID string, to string) (*account.SignedTransaction, bool) {
	if p.miner == nil || strings.TrimSpace(collectionID) == "" || strings.TrimSpace(tokenID) == "" || strings.TrimSpace(to) == "" {
		return nil, false
	}
	tx := account.NewNFTTransferTransaction(id, p.miner, collectionID, tokenID, to)
	if tx.NFT == nil || tx.NFT.CollectionID == "" || tx.NFT.TokenID == "" {
		return nil, false
	}
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodAssetCreditTransaction(id string, asset string, to string, amount int) (*account.SignedTransaction, bool) {
	if p.miner == nil || strings.TrimSpace(asset) == "" || strings.TrimSpace(to) == "" || amount < 1 {
		return nil, false
	}
	tx := account.NewAssetCreditTransaction(id, p.miner, asset, to, amount)
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodAssetDebitTransaction(id string, asset string, accountName string, amount int) (*account.SignedTransaction, bool) {
	if p.miner == nil || strings.TrimSpace(asset) == "" || strings.TrimSpace(accountName) == "" || amount < 1 {
		return nil, false
	}
	tx := account.NewAssetDebitTransaction(id, p.miner, asset, accountName, amount)
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodAMMCreatePoolTransaction(id string, assetA string, assetB string) (*account.SignedTransaction, bool) {
	if p.miner == nil || account.PoolID(assetA, assetB) == "" {
		return nil, false
	}
	tx := account.NewAMMCreatePoolTransaction(id, p.miner, assetA, assetB)
	p.FloodPoSTransaction(tx)
	return tx, true
}

func (p *Peer) FloodAMMAddLiquidityTransaction(id string, poolID string, amountA int, amountB int) (*account.SignedTransaction, bool) {
	if p.miner == nil || strings.TrimSpace(poolID) == "" || amountA < 1 || amountB < 1 {
		return nil, false
	}
	tx := account.NewAMMAddLiquidityTransaction(id, p.miner, poolID, amountA, amountB)
	p.FloodPoSTransaction(tx)
	return tx, true
}

// MineOneSlot tries to produce and flood one block for the given slot.
// Returns the mined block when successful.
func (p *Peer) MineOneSlot(slot int) *account.Block {
	if p.blockchain == nil || p.miner == nil || p.genesis == nil {
		return nil
	}
	p.lotteryAttempts.Add(1)
	proof := account.SignLotteryProof(p.genesis.Seed, slot, p.miner)
	draw, ok := account.ComputeLotteryDrawFromProof(proof)
	if !ok {
		p.lotteryMisses.Add(1)
		return nil
	}
	tickets := p.ValidatorStake()
	if tickets <= 0 {
		p.lotteryNoTicket.Add(1)
		return nil
	}
	if !account.WinsLottery(tickets, p.optimisticLotteryHardness(), draw) {
		p.lotteryMisses.Add(1)
		return nil
	}

	parentBlocks := p.blockchain.CanonicalBlocks(0)
	parentLedger := account.LedgerFromBlockchain(p.blockchain, 0)
	parentHash := p.blockchain.BestLeafHash()
	// Build a block from txs that are valid on top of current best chain state.
	// This prevents one invalid tx from making the whole candidate block invalid.
	tempLedger := parentLedger
	p.pruneInvalidMempool("pre_mine")
	p.mempoolLock.Lock()
	selected := make([]account.SignedTransaction, 0, account.BLOCK_SIZE)
	for _, tx := range p.mempool {
		if tempLedger.Transaction(&tx) {
			selected = append(selected, tx)
		}
		if len(selected) >= account.BLOCK_SIZE {
			break
		}
	}
	p.mempoolLock.Unlock()
	hardness := account.EffectiveHardnessForMining(parentBlocks, p.genesis, len(selected))
	if !account.WinsLottery(tickets, hardness, draw) {
		// The optimistic precheck may pass with a larger pending mempool than the
		// final valid selection. Re-check against the exact candidate before mining.
		p.lotteryMisses.Add(1)
		return nil
	}
	p.lotteryWins.Add(1)

	block := account.NewCandidateBlock(p.miner, slot, parentHash, selected, p.genesis.Seed)
	payload, err := json.Marshal(block)
	if err != nil {
		return nil
	}
	hash := block.GetBlockHash()
	msgID := base64.StdEncoding.EncodeToString(hash[:])
	msg := &Message{Type: helpers.BLOCK_MESSAGE_TYPE, MsgID: msgID, From: p.id, Payload: payload}

	if !p.ApplyBlock(block) {
		return nil
	}
	p.minedBlocks.Add(1)
	p.FloodNetwork(msg)
	return block
}

func (p *Peer) optimisticLotteryHardness() int {
	if p.genesis == nil {
		return 0
	}
	if p.genesis.DynamicHardnessMax > 0 {
		return p.genesis.DynamicHardnessMax
	}
	if p.genesis.Hardness > 0 {
		return p.genesis.Hardness
	}
	return 1
}

// ApplyBlock force-applies a block locally (used by the handin demo for deterministic convergence).
func (p *Peer) ApplyBlock(block *account.Block) bool {
	if p.blockchain == nil {
		return false
	}
	if !p.blockchain.AddBlock(block) {
		return false
	}
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	p.acceptedBlocks.Add(1)
	txs, err := account.DecodeBlockTransactions(block.MetaData)
	if err == nil {
		p.mempoolLock.Lock()
		for _, tx := range txs {
			if _, exists := p.mempool[tx.ID]; exists {
				delete(p.mempool, tx.ID)
				logEvent("mempool_tx_included", map[string]any{"tx": tx.ID, "block": blockHexString(block), "slot": block.Slot})
			}
		}
		p.mempoolLock.Unlock()
	}
	p.pruneInvalidMempool("block_applied")
	return true
}

// Send a transaction across the network.
func (p *Peer) FloodTransaction(tx *account.SignedTransaction) {
	payload, err := json.Marshal(&tx)
	if err != nil {
		fmt.Println(err) // For testing. There should not be any way to make errors here in production code.
	}
	// We don't receive our own flood, so apply the transaction locally here too.
	if p.hasSeenMessage(tx.ID) { // Prevent replay independently from finality depth.
		return
	}
	msg := &Message{Type: helpers.TRANSACTION_MESSAGE_TYPE, MsgID: tx.ID, From: p.id, Payload: payload}
	if !p.handleTransaction(msg) {
		return
	}
	// We don't receive our own flood back on the network, so push one local event here.
	// This keeps demo/test counting symmetric across all peers.
	p.received <- *msg
	p.FloodNetwork(msg)
}

func (p *Peer) handlePoSTransaction(msg *Message) bool {
	var tx account.SignedTransaction
	if err := json.Unmarshal(msg.Payload, &tx); err != nil {
		return false
	}
	// Cheap local checks before inserting to mempool.
	if tx.Amount < 0 || (!tx.IsEVM() && !tx.IsNFT() && !tx.IsAMM() && tx.Amount < 1) {
		logEvent("mempool_tx_rejected", map[string]any{"tx": tx.ID, "from": tx.From, "kind": tx.Kind, "reason": "invalid_amount"})
		return false
	}
	if !tx.Verify(tx.From) {
		logEvent("mempool_tx_rejected", map[string]any{"tx": tx.ID, "from": tx.From, "kind": tx.Kind, "reason": "verify_failed"})
		return false
	}
	if tx.IsAMM() && !p.bestLedgerWouldAccept(&tx) {
		logEvent("mempool_tx_rejected", map[string]any{"tx": tx.ID, "from": tx.From, "kind": tx.Kind, "reason": "best_ledger_reject"})
		return false
	}

	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	if _, exists := p.msgHistory[msg.MsgID]; exists {
		return false
	}
	// Deduplicate mempool entries too. EVM wallets use same-nonce replacements
	// for speeding up and cancellation, so keep only the latest tx for a nonce.
	p.mempoolLock.Lock()
	_, exists := p.mempool[tx.ID]
	if !exists {
		if tx.IsEVM() {
			for id, pending := range p.mempool {
				if pending.IsEVM() && strings.EqualFold(pending.From, tx.From) && pending.Nonce == tx.Nonce {
					delete(p.mempool, id)
					logEvent("mempool_tx_replaced", map[string]any{"oldTx": id, "newTx": tx.ID, "from": tx.From, "nonce": tx.Nonce})
				}
			}
		}
		p.mempool[tx.ID] = tx
		logEvent("mempool_tx_added", map[string]any{"tx": tx.ID, "from": tx.From, "to": tx.To, "kind": tx.Kind, "nonce": tx.Nonce, "isEVM": tx.IsEVM(), "isAMM": tx.IsAMM(), "isNFT": tx.IsNFT()})
	}
	p.mempoolLock.Unlock()

	hist := *NewMessageHistory(msg)
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist
	return !exists
}

func (p *Peer) bestLedgerWouldAccept(tx *account.SignedTransaction) bool {
	if p.blockchain == nil {
		return false
	}
	ledger := account.LedgerFromBlockchain(p.blockchain, 0)
	return ledger.Transaction(tx)
}

func (p *Peer) pruneInvalidMempool(reason string) int {
	if p.blockchain == nil {
		return 0
	}
	ledger := account.LedgerFromBlockchain(p.blockchain, 0)
	p.mempoolLock.Lock()
	pending := make([]account.SignedTransaction, 0, len(p.mempool))
	for _, tx := range p.mempool {
		pending = append(pending, tx)
	}
	p.mempoolLock.Unlock()

	keep := make(map[string]bool, len(pending))
	progress := true
	for progress {
		progress = false
		for _, tx := range pending {
			if keep[tx.ID] {
				continue
			}
			candidate := tx
			if !ledger.Transaction(&candidate) {
				continue
			}
			keep[tx.ID] = true
			progress = true
		}
	}

	removed := 0
	for _, tx := range pending {
		if keep[tx.ID] {
			continue
		}
		p.mempoolLock.Lock()
		if _, exists := p.mempool[tx.ID]; exists {
			delete(p.mempool, tx.ID)
			removed++
			logEvent("mempool_tx_evicted", map[string]any{"tx": tx.ID, "from": tx.From, "kind": tx.Kind, "nonce": tx.Nonce, "reason": reason})
		}
		p.mempoolLock.Unlock()
	}
	return removed
}

func (p *Peer) handleBlock(msg *Message) bool {
	if p.blockchain == nil {
		return false
	}
	var block account.Block
	if err := json.Unmarshal(msg.Payload, &block); err != nil {
		return false
	}

	p.floodingLock.Lock()
	if _, exists := p.msgHistory[msg.MsgID]; exists {
		p.floodingLock.Unlock()
		return false
	}
	p.floodingLock.Unlock()

	if !p.blockchain.AddBlock(&block) {
		if !p.hasBlockHash(block.ParentHash) {
			local := p.chainStatus()
			if local.GenesisHash != "" {
				logEvent("block_parent_missing_sync_requested", map[string]any{
					"peerID":        msg.From,
					"missingParent": block.ParentHash,
					"localHeight":   local.Height,
				})
				p.requestChainSync(msg.From, chainSyncRequest{GenesisHash: local.GenesisHash, FromHeight: 0})
			}
		}
		return false
	}

	// Rebuild ledger from best chain (simple and deterministic).
	p.ledger = account.LedgerFromBlockchain(p.blockchain, p.finalityDepth)
	p.acceptedBlocks.Add(1)

	// Remove included txs from mempool.
	txs, err := account.DecodeBlockTransactions(block.MetaData)
	if err == nil {
		p.mempoolLock.Lock()
		for _, tx := range txs {
			if _, exists := p.mempool[tx.ID]; exists {
				delete(p.mempool, tx.ID)
				logEvent("mempool_tx_included", map[string]any{"tx": tx.ID, "block": blockHexString(&block), "slot": block.Slot})
			}
		}
		p.mempoolLock.Unlock()
	}
	p.pruneInvalidMempool("block_received")

	p.floodingLock.Lock()
	hist := *NewMessageHistory(msg)
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist
	p.floodingLock.Unlock()
	return true
}

func (p *Peer) hasBlockHash(hash string) bool {
	if hash == "" || p.blockchain == nil {
		return false
	}
	for _, block := range p.blockchain.BlocksSnapshot() {
		if blockHashString(block) == hash {
			return true
		}
	}
	return false
}

// Handle receiving a transaction from another peer on the network.
func (p *Peer) handleTransaction(msg *Message) bool {
	var tx account.SignedTransaction
	if err := json.Unmarshal(msg.Payload, &tx); err != nil {
		return false
	}
	// Check-and-mark is atomic under one lock:
	// without this, two concurrent duplicate deliveries could both pass "not seen yet"
	// and apply the same transaction twice, causing ledger divergence.
	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	_, exists := p.msgHistory[msg.MsgID]
	if exists {
		return false
	}
	if p.ledger.HasRecordedTransaction(&tx) { // Prevent replay attacks.
		return false
	}
	// Apply the transaction to our local ledger.
	if !p.ledger.Transaction(&tx) {
		return false
	}
	p.acceptedTxs.Add(1)
	hist := *NewMessageHistory(msg)
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist

	return true
}

func (p *Peer) logf(format string, a ...any) {
	if !p.verbose {
		// Default path in hand-in runs: keep output minimal and readable.
		return
	}
	p.output <- fmt.Sprintf(format, a...)
}

func (p *Peer) connection(id string) net.Conn {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.conns[id]
}

func (p *Peer) hasConnection(id string) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, exists := p.conns[id]
	return exists
}

func (p *Peer) connectionIDs() []string {
	p.lock.Lock()
	defer p.lock.Unlock()
	return slices.Collect(maps.Keys(p.conns))
}

func (p *Peer) connectPayload() connectPayload {
	p.lock.Lock()
	defer p.lock.Unlock()
	addresses := make([]PeerAddress, 0, len(p.peerAddresses)+1)
	for _, address := range p.peerAddresses {
		if address.ID != "" && address.Host != "" && address.Port > 0 {
			addresses = append(addresses, address)
		}
	}
	if p.advertiseHost != "" {
		self := PeerAddress{ID: p.id, Host: p.advertiseHost, Port: p.listenport}
		p.peerAddresses[p.id] = self
		addresses = append(addresses, self)
	}
	status := p.chainStatusLocked()
	return connectPayload{
		Peers:       slices.Collect(maps.Keys(p.conns)),
		Addresses:   uniquePeerAddresses(addresses),
		Validator:   p.validatorAccountLocked(),
		GenesisHash: status.GenesisHash,
		Height:      status.Height,
		BestHash:    status.BestHash,
	}
}

func (p *Peer) chainStatus() chainStatusSummary {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.chainStatusLocked()
}

func (p *Peer) chainStatusLocked() chainStatusSummary {
	if p.blockchain == nil {
		return chainStatusSummary{}
	}
	blocks := p.blockchain.CanonicalBlocks(0)
	if len(blocks) == 0 {
		return chainStatusSummary{}
	}
	return chainStatusSummary{
		GenesisHash: genesisContentHash(blocks[0]),
		Height:      len(blocks) - 1,
		BestHash:    blockHashString(blocks[len(blocks)-1]),
	}
}

func blockHashString(block account.Block) string {
	hash := block.GetBlockHash()
	return base64.StdEncoding.EncodeToString(hash[:])
}

func blockHexString(block *account.Block) string {
	if block == nil {
		return ""
	}
	hash := block.GetBlockHash()
	return "0x" + fmt.Sprintf("%x", hash[:])
}

func genesisContentHash(block account.Block) string {
	hash := sha256.Sum256([]byte(block.MetaData))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (p *Peer) knownPeerAddresses() []PeerAddress {
	p.lock.Lock()
	defer p.lock.Unlock()
	addresses := make([]PeerAddress, 0, len(p.peerAddresses))
	for _, address := range p.peerAddresses {
		if address.ID != "" && address.ID != p.id && address.Host != "" && address.Port > 0 {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

func (p *Peer) validatorAccount() string {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.validatorAccountLocked()
}

func (p *Peer) validatorAccountLocked() string {
	if p.miner == nil {
		return ""
	}
	return p.miner.SafeEncode()
}

func (p *Peer) recordPeerAddress(address PeerAddress) {
	if address.ID == "" || address.Host == "" || address.Port <= 0 {
		return
	}
	p.lock.Lock()
	p.peerAddresses[address.ID] = address
	p.lock.Unlock()
}

func (p *Peer) recordPeerValidator(peerID string, validator string) {
	validator = strings.TrimSpace(validator)
	if peerID == "" || validator == "" {
		return
	}
	p.lock.Lock()
	p.peerValidators[peerID] = validator
	p.lock.Unlock()
}

func peersFromConnectPayload(raw []byte) []string {
	var payload connectPayload
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Peers != nil {
		return payload.Peers
	}
	var peers []string
	_ = json.Unmarshal(raw, &peers)
	return peers
}

func (p *Peer) addressesFromConnectPayload(raw []byte) []PeerAddress {
	var payload connectPayload
	if err := json.Unmarshal(raw, &payload); err == nil {
		addresses := make([]PeerAddress, 0, len(payload.Addresses)+len(payload.Peers))
		addresses = append(addresses, payload.Addresses...)
		for _, peer := range payload.Peers {
			if _, exists := findPeerAddress(addresses, peer); exists {
				continue
			}
			if address, exists := p.peerAddress(peer); exists {
				addresses = append(addresses, address)
			}
		}
		return uniquePeerAddresses(addresses)
	}
	return nil
}

func (p *Peer) peerAddress(peerID string) (PeerAddress, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	address, exists := p.peerAddresses[peerID]
	return address, exists
}

func addressFromConnectPayload(peerID string, raw []byte, remote net.Addr) PeerAddress {
	var payload connectPayload
	if err := json.Unmarshal(raw, &payload); err == nil {
		for _, address := range payload.Addresses {
			if address.ID == peerID && address.Host != "" && address.Port > 0 {
				return address
			}
		}
		if payload.Address != "" {
			host, portText, err := net.SplitHostPort(payload.Address)
			if err == nil {
				if port, convErr := strconv.Atoi(portText); convErr == nil {
					return PeerAddress{ID: peerID, Host: host, Port: port}
				}
			}
		}
	}
	if tcp, ok := remote.(*net.TCPAddr); ok {
		return PeerAddress{ID: peerID, Host: tcp.IP.String(), Port: tcp.Port}
	}
	return PeerAddress{ID: peerID}
}

func uniquePeerAddresses(addresses []PeerAddress) []PeerAddress {
	seen := make(map[string]PeerAddress)
	for _, address := range addresses {
		if address.ID == "" || address.Host == "" || address.Port <= 0 {
			continue
		}
		seen[address.ID] = address
	}
	out := make([]PeerAddress, 0, len(seen))
	for _, address := range seen {
		out = append(out, address)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func findPeerAddress(addresses []PeerAddress, id string) (PeerAddress, bool) {
	for _, address := range addresses {
		if address.ID == id {
			return address, true
		}
	}
	return PeerAddress{}, false
}

func validatorFromConnectPayload(raw []byte) string {
	var payload connectPayload
	if err := json.Unmarshal(raw, &payload); err == nil {
		return strings.TrimSpace(payload.Validator)
	}
	return ""
}

type PeerAuthorization struct {
	PeerID     string `json:"peerId"`
	Validator  string `json:"validator"`
	Stake      int    `json:"stake"`
	Authorized bool   `json:"authorized"`
	Local      bool   `json:"local"`
	Connected  bool   `json:"connected"`
}

func (p *Peer) AuthorizedPeers() []PeerAuthorization {
	conns := p.connectionSnapshot()
	p.lock.Lock()
	validators := make(map[string]string, len(p.peerValidators))
	for peerID, validator := range p.peerValidators {
		validators[peerID] = validator
	}
	p.lock.Unlock()
	if p.miner != nil {
		validators[p.id] = p.miner.SafeEncode()
	}
	rows := make([]PeerAuthorization, 0, len(validators))
	for peerID, validator := range validators {
		stake := p.BalanceOf(validator)
		_, connected := conns[peerID]
		rows = append(rows, PeerAuthorization{
			PeerID:     peerID,
			Validator:  validator,
			Stake:      stake,
			Authorized: validator != "",
			Local:      peerID == p.id,
			Connected:  connected && peerID != p.id,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Authorized != rows[j].Authorized {
			return rows[i].Authorized
		}
		return rows[i].PeerID < rows[j].PeerID
	})
	return rows
}

func (p *Peer) ActiveAuthorizedPeers() []PeerAuthorization {
	rows := p.AuthorizedPeers()
	active := make([]PeerAuthorization, 0, len(rows))
	for _, row := range rows {
		if row.Local || row.Connected {
			active = append(active, row)
		}
	}
	return active
}

func (p *Peer) IsAuthorizedValidator(accountName string) bool {
	if p.blockchain == nil {
		return false
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).IsValidator(accountName)
}

func (p *Peer) connectionSnapshot() map[string]net.Conn {
	p.lock.Lock()
	defer p.lock.Unlock()
	out := make(map[string]net.Conn, len(p.conns))
	for peerID, conn := range p.conns {
		out[peerID] = conn
	}
	return out
}

func (p *Peer) PeerCount() int {
	conns := p.connectionSnapshot()
	if len(conns) == 0 {
		return 0
	}
	// Exclude self id, which is kept in the map as nil.
	return len(conns) - 1
}

func (p *Peer) MempoolSize() int {
	p.mempoolLock.Lock()
	defer p.mempoolLock.Unlock()
	return len(p.mempool)
}

type MempoolTxSummary struct {
	ID                   string `json:"id"`
	Kind                 string `json:"kind"`
	From                 string `json:"from"`
	To                   string `json:"to"`
	Amount               int    `json:"amount"`
	Nonce                uint64 `json:"nonce"`
	ChainID              int64  `json:"chainId,omitempty"`
	IsEVM                bool   `json:"isEvm"`
	IsAMM                bool   `json:"isAmm"`
	IsNFT                bool   `json:"isNft"`
	BestLedgerWouldApply bool   `json:"bestLedgerWouldApply"`
}

func (p *Peer) MempoolSummaries() []MempoolTxSummary {
	p.mempoolLock.Lock()
	pending := make([]account.SignedTransaction, 0, len(p.mempool))
	for _, tx := range p.mempool {
		pending = append(pending, tx)
	}
	p.mempoolLock.Unlock()
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].ID < pending[j].ID
	})
	out := make([]MempoolTxSummary, 0, len(pending))
	for _, tx := range pending {
		candidate := tx
		out = append(out, MempoolTxSummary{
			ID:                   tx.ID,
			Kind:                 tx.Kind,
			From:                 tx.From,
			To:                   tx.To,
			Amount:               tx.Amount,
			Nonce:                tx.Nonce,
			ChainID:              tx.ChainID,
			IsEVM:                tx.IsEVM(),
			IsAMM:                tx.IsAMM(),
			IsNFT:                tx.IsNFT(),
			BestLedgerWouldApply: p.bestLedgerWouldAccept(&candidate),
		})
	}
	return out
}

func (p *Peer) ChainHeight() int {
	if p.blockchain == nil {
		return 0
	}
	return p.blockchain.BestHeight()
}

func (p *Peer) AverageRecentBlockSlots(window int) int {
	if p.blockchain == nil {
		return 0
	}
	blocks := p.blockchain.CanonicalBlocks(0)
	if len(blocks) < 2 {
		return 0
	}
	if window < 1 {
		window = 1
	}
	start := len(blocks) - 1 - window
	if start < 0 {
		start = 0
	}
	total := 0
	count := 0
	for i := start + 1; i < len(blocks); i++ {
		delta := blocks[i].Slot - blocks[i-1].Slot
		if delta <= 0 {
			continue
		}
		total += delta
		count++
	}
	if count == 0 {
		return 0
	}
	return (total + count/2) / count
}

func (p *Peer) CurrentEffectiveHardness() int {
	if p.blockchain == nil || p.genesis == nil {
		return 0
	}
	return account.EffectiveHardnessForMining(p.blockchain.CanonicalBlocks(0), p.genesis, p.MempoolSize())
}

func (p *Peer) DynamicHardnessForkHeight() int {
	if p.genesis == nil {
		return 0
	}
	return p.genesis.DynamicHardnessForkHeight
}

func (p *Peer) DynamicHardnessActive() bool {
	forkHeight := p.DynamicHardnessForkHeight()
	return forkHeight > 0 && p.ChainHeight() >= forkHeight
}

func (p *Peer) UptimeSeconds() int64 {
	return int64(time.Since(p.startedAt).Seconds())
}

func (p *Peer) AcceptedBlocks() uint64 {
	return p.acceptedBlocks.Load()
}

func (p *Peer) MinedBlocks() uint64 {
	return p.minedBlocks.Load()
}

func (p *Peer) LotteryAttempts() uint64 {
	return p.lotteryAttempts.Load()
}

func (p *Peer) LotteryWins() uint64 {
	return p.lotteryWins.Load()
}

func (p *Peer) LotteryMisses() uint64 {
	return p.lotteryMisses.Load()
}

func (p *Peer) LotteryNoTicket() uint64 {
	return p.lotteryNoTicket.Load()
}

func (p *Peer) AcceptedTransactions() uint64 {
	return p.acceptedTxs.Load()
}

func (p *Peer) TxHistoryCount() int {
	if p.ledger == nil {
		return 0
	}
	return p.ledger.TxCount()
}

func (p *Peer) BalanceOf(accountName string) int {
	if p.ledger == nil {
		return 0
	}
	return p.ledger.GetBalance(accountName)
}

func (p *Peer) BestBalanceOf(accountName string) int {
	if p.blockchain == nil {
		return 0
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).GetBalance(accountName)
}

func (p *Peer) SupplySnapshot(reservedAddresses []string) SupplySnapshot {
	if p.ledger == nil {
		return SupplySnapshot{}
	}
	accounts := p.ledger.CopyAccounts()
	reservedSet := make(map[string]bool, len(reservedAddresses))
	for _, address := range reservedAddresses {
		reservedSet[strings.ToLower(strings.TrimSpace(address))] = true
	}
	var total int
	var reserved int
	for accountName, balance := range accounts {
		if balance <= 0 {
			continue
		}
		total += balance
		if reservedSet[strings.ToLower(accountName)] {
			reserved += balance
		}
	}
	circulating := total - reserved
	if circulating < 0 {
		circulating = 0
	}
	return SupplySnapshot{
		TotalSupply:       total,
		CirculatingSupply: circulating,
		ReservedSupply:    reserved,
		ReservedAddresses: reservedAddresses,
	}
}

func (p *Peer) ValidatorStake() int {
	if p.miner == nil || p.ledger == nil {
		return 0
	}
	return p.ledger.GetBalance(p.miner.SafeEncode())
}

func (p *Peer) NonceOf(accountName string) uint64 {
	if p.ledger == nil {
		return 0
	}
	return p.ledger.GetNonce(accountName)
}

func (p *Peer) BestNonceOf(accountName string) uint64 {
	if p.blockchain == nil {
		return 0
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).GetNonce(accountName)
}

func (p *Peer) BestNFTs() map[string]account.NFTToken {
	if p.blockchain == nil {
		return map[string]account.NFTToken{}
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).CopyNFTs()
}

func (p *Peer) BestNFT(collectionID string, tokenID string) (account.NFTToken, bool) {
	if p.blockchain == nil {
		return account.NFTToken{}, false
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).NFT(collectionID, tokenID)
}

func (p *Peer) BestNFTsByOwner(owner string) []account.NFTToken {
	if p.blockchain == nil {
		return []account.NFTToken{}
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).NFTsByOwner(owner)
}

func (p *Peer) BestAssetBalances(accountName string) map[string]int {
	if p.blockchain == nil {
		return map[string]int{}
	}
	ledger := account.LedgerFromBlockchain(p.blockchain, 0)
	balances := ledger.CopyAssetBalances()
	out := map[string]int{account.AssetPKN: ledger.GetBalance(strings.ToLower(strings.TrimSpace(accountName)))}
	for asset, byAccount := range balances {
		value := byAccount[strings.ToLower(strings.TrimSpace(accountName))]
		if value != 0 {
			out[asset] = value
		}
	}
	return out
}

func (p *Peer) BestAMMPools() map[string]account.AMMPool {
	if p.blockchain == nil {
		return map[string]account.AMMPool{}
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).CopyAMMPools()
}

func (p *Peer) BestAMMPool(poolID string) (account.AMMPool, bool) {
	if p.blockchain == nil {
		return account.AMMPool{}, false
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).AMMPool(poolID)
}

func (p *Peer) BestEVMCode(address string) []byte {
	if p.blockchain == nil {
		return nil
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).GetEVMCode(address)
}

func (p *Peer) BestEVMStorage(address string, slot string) string {
	if p.blockchain == nil {
		return "0x0000000000000000000000000000000000000000000000000000000000000000"
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).GetEVMStorage(address, slot)
}

func (p *Peer) BestEVMAccounts() map[string]account.EVMAccount {
	if p.blockchain == nil {
		return map[string]account.EVMAccount{}
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).CopyEVMAccounts()
}

func (p *Peer) BestEVMCall(from string, to string, input []byte) ([]byte, bool) {
	if p.blockchain == nil {
		return nil, false
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).CallEVM(from, to, input)
}

func (p *Peer) EVMReceipt(txHash string) (account.EVMReceipt, bool) {
	if p.blockchain == nil {
		return account.EVMReceipt{}, false
	}
	return account.LedgerFromBlockchain(p.blockchain, 0).EVMReceipt(txHash)
}

func (p *Peer) PendingNonceOf(accountName string) uint64 {
	nonce := p.BestNonceOf(accountName)
	p.mempoolLock.Lock()
	defer p.mempoolLock.Unlock()
	for _, tx := range p.mempool {
		if tx.IsEVM() && strings.EqualFold(tx.From, accountName) && tx.Nonce >= nonce {
			nonce = tx.Nonce + 1
		}
	}
	return nonce
}

func (p *Peer) TransactionByID(txID string) (account.SignedTransaction, bool) {
	if explorerTx, exists := BuildExplorerIndex(p).TxsByHash[strings.ToLower(txID)]; exists {
		return signedTransactionFromExplorer(explorerTx), true
	}
	p.mempoolLock.Lock()
	defer p.mempoolLock.Unlock()
	tx, exists := p.mempool[txID]
	return tx, exists
}

func (p *Peer) TransactionLocation(txID string) (account.SignedTransaction, string, int, uint64, bool) {
	if explorerTx, exists := BuildExplorerIndex(p).TxsByHash[strings.ToLower(txID)]; exists {
		return signedTransactionFromExplorer(explorerTx), explorerTx.BlockHash, explorerTx.BlockNumber, uint64(explorerTx.TransactionIndex), true
	}
	p.mempoolLock.Lock()
	defer p.mempoolLock.Unlock()
	tx, exists := p.mempool[txID]
	return tx, "", 0, 0, exists
}

func (p *Peer) BlockByNumber(height int) (account.Block, int, bool) {
	if p.blockchain == nil {
		return account.Block{}, 0, false
	}
	blocks := p.blockchain.CanonicalBlocks(0)
	if height < 0 || height >= len(blocks) {
		return account.Block{}, 0, false
	}
	return blocks[height], height, true
}

func (p *Peer) BlockByHash(hash string) (account.Block, int, bool) {
	if p.blockchain == nil {
		return account.Block{}, 0, false
	}
	hash = strings.TrimPrefix(strings.ToLower(hash), "0x")
	blocks := p.blockchain.CanonicalBlocks(0)
	for height, block := range blocks {
		blockHash := block.GetBlockHash()
		if fmt.Sprintf("%x", blockHash[:]) == hash {
			return block, height, true
		}
	}
	return account.Block{}, 0, false
}

func (p *Peer) Ready() bool {
	return p.blockchain != nil && p.genesis != nil && p.miner != nil
}

func (p *Peer) FinalityDepth() int {
	return p.finalityDepth
}

func (p *Peer) CommittedHeight() int {
	height := p.ChainHeight()
	if height <= p.finalityDepth {
		return 0
	}
	return height - p.finalityDepth
}

func (p *Peer) hasSeenMessage(msgID string) bool {
	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	_, exists := p.msgHistory[msgID]
	return exists
}
