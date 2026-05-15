package main

import (
	"bufio"
	"bytes"
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
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Peer struct {
	listenport int
	id         string
	conns      map[string]net.Conn
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
	startedAt      time.Time
	acceptedBlocks atomic.Uint64
	minedBlocks    atomic.Uint64
	acceptedTxs    atomic.Uint64
}

type MessageHistory struct {
	content      *Message // The actual message.
	receivedFrom []string // List of IDs from all peers this peer has received this message from.
	isSent       bool     // Has this peer already sent this message.
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

// ConfigurePoS initializes blockchain state for Exercise 16.2.
// All peers must receive the same genesis metadata.
func (p *Peer) ConfigurePoS(genesis *account.GenesisMetaData, miner *account.Account) {
	p.genesis = genesis
	p.miner = miner
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
func (p *Peer) prepareConnection(conn net.Conn, fromAccept bool) ([]string, error) {
	reader := bufio.NewReader(conn)
	// Handshake: announce our id, then learn the peer id.
	// Send a connect message. The other peer will receivee it in their prepareConnection at the readMessage line.
	// Message payload is the list of known peers, this knows about (self, if this is a new peer, otherwise the entire network).
	payload, _ := json.Marshal(p.connectionIDs()) // Convert connection keys to []string and marshal it.
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
	go p.handleDecode(msg.From, conn, reader)
	// Catch-up: when we are the server (we accepted), send the new peer all messages we already have
	// so they get the same ledger and message history as the rest of the network.
	if fromAccept {
		p.sendCatchUp(conn)
	}
	// Unmarshal the received list of peers the connection knew about.
	var peers []string
	json.Unmarshal(msg.Payload, &peers)
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
	conn, err := net.Dial(helpers.PROTOCOL, addr+":"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}
	// We are the client (joiner), so fromAccept is false (no catch-up from our side).
	peers, err := p.prepareConnection(conn, false)
	if err != nil {
		return nil, err
	}
	return peers, nil
}

// Connect to a list of peers (may be known, in which case, it does notrhing).
func (p *Peer) joinNetwork(addr string, peers []string) {
	for _, peer := range peers {
		if !p.hasConnection(peer) {
			port, _ := strconv.Atoi(peer)
			p.Connect(addr, port)
		}
	}
}

// floodJoin announces to the network that we joined, so others can add us to their peer set.
func (p *Peer) floodJoin() {
	joinMsg := &Message{
		Type:    helpers.JOIN_MESSAGE_TYPE,
		MsgID:   "join-" + p.id,
		From:    p.id,
		Payload: []byte(p.id),
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

// handleJoin adds the new peer from a Join message to our peer set and connects if we didn't know them.
func (p *Peer) handleJoin(msg *Message) {
	newPeerID := msg.From
	if p.hasConnection(newPeerID) {
		return
	}
	// Connect so we have an open TCP connection (fully connected network).
	port, err := strconv.Atoi(newPeerID)
	if err != nil {
		return
	}
	// Use same host as we use elsewhere (e.g. localhost in tests).
	_, _ = p.Connect("localhost", port)
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

// MineOneSlot tries to produce and flood one block for the given slot.
// Returns the mined block when successful.
func (p *Peer) MineOneSlot(slot int) *account.Block {
	if p.blockchain == nil || p.miner == nil || p.genesis == nil {
		return nil
	}
	accountName := p.miner.SafeEncode()
	tickets := p.genesis.InitialBalances[accountName]
	if tickets <= 0 {
		return nil
	}
	draw := account.ComputeLotteryDraw(p.genesis.Seed, slot, accountName)
	if !account.WinsLottery(tickets, p.genesis.Hardness, draw) {
		return nil
	}

	parentHash := p.blockchain.BestLeafHash()
	// Build a block from txs that are valid on top of current best chain state.
	// This prevents one invalid tx from making the whole candidate block invalid.
	tempLedger := account.LedgerFromBlockchain(p.blockchain, 0)
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

	block := account.NewCandidateBlock(p.miner, slot, parentHash, selected, p.genesis.Seed)
	payload, err := json.Marshal(block)
	if err != nil {
		return nil
	}
	hash := block.GetBlockHash()
	msgID := base64.StdEncoding.EncodeToString(hash[:])
	msg := &Message{Type: helpers.BLOCK_MESSAGE_TYPE, MsgID: msgID, From: p.id, Payload: payload}

	if !p.handleBlock(msg) {
		return nil
	}
	p.minedBlocks.Add(1)
	p.FloodNetwork(msg)
	return block
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
			delete(p.mempool, tx.ID)
		}
		p.mempoolLock.Unlock()
	}
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
	if tx.Amount < 1 {
		return false
	}
	if !tx.Verify(tx.From) {
		return false
	}

	p.floodingLock.Lock()
	defer p.floodingLock.Unlock()
	if _, exists := p.msgHistory[msg.MsgID]; exists {
		return false
	}
	// Deduplicate mempool entries too.
	p.mempoolLock.Lock()
	_, exists := p.mempool[tx.ID]
	if !exists {
		p.mempool[tx.ID] = tx
	}
	p.mempoolLock.Unlock()

	hist := *NewMessageHistory(msg)
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist
	return !exists
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
			delete(p.mempool, tx.ID)
		}
		p.mempoolLock.Unlock()
	}

	p.floodingLock.Lock()
	hist := *NewMessageHistory(msg)
	hist.receivedFrom = append(hist.receivedFrom, msg.From)
	p.msgHistory[msg.MsgID] = hist
	p.floodingLock.Unlock()
	return true
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

func (p *Peer) ChainHeight() int {
	if p.blockchain == nil {
		return 0
	}
	return p.blockchain.BestHeight()
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

func (p *Peer) AcceptedTransactions() uint64 {
	return p.acceptedTxs.Load()
}

func (p *Peer) TxHistoryCount() int {
	if p.ledger == nil {
		return 0
	}
	return p.ledger.TxCount()
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
