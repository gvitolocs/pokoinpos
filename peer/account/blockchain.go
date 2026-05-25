package account

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"peer/dissycrypto"
	"strconv"
	"sync"
)

const BLOCK_SIZE = 16 // The maximum number of transactions in a block.
const BLOCK_TYPE = "block"
const NO_PARENT_HASH = "" // The hahs of the genesis block's parent (i.e., none as it has no parent).
const MINER_BLOCK_REWARD = 10
const LOTTERY_DRAW_RANGE = 1_000_000_000_000 // 10^12
const VALIDATOR_TICKET_WEIGHT = 1_000_000

type Block struct {
	Type            string // Tuple type: The text "block".
	VerificationKey string // Verification key for block winner.
	Slot            int    // Slot number.
	Draw            int    // The draw used to win the lottery.
	MetaData        string // The block metadata. For Genesis, this is settings. Otherwise, transactions.
	ParentHash      string // The hash of the parent block.
	Signature       string // A signature on the block for authentication from the winner.
	LotteryProof    string // A signature over (seed, slot) used to derive the lottery draw.
}

func NewBlock(winner *Account, slot int, draw int, parentHash string, metaData string) *Block {
	b := new(Block)
	b.Type = BLOCK_TYPE
	b.Slot = slot
	b.Draw = draw
	b.MetaData = metaData
	b.ParentHash = parentHash
	if winner != nil { // Useful for Genesis. Otherwise, this will make the block invalid.
		b.VerificationKey = winner.SafeEncode()
		b.Signature = b.Sign(winner)
	}
	return b
}

func (b *Block) marshalForSignature() []byte {
	fields := []string{b.Type, b.VerificationKey, strconv.Itoa(b.Slot), strconv.Itoa(b.Draw), b.ParentHash}
	if b.LotteryProof != "" {
		fields = append(fields, b.LotteryProof)
	}
	msg, err := json.Marshal(fields)
	if err != nil {
		return []byte{}
	}
	return msg
}

func (b *Block) MarshalBlock() []byte {
	msg := b.marshalForSignature()
	signature, err := decode(b.Signature)
	if err != nil {
		return msg
	}
	msg = append(msg, signature...)
	return msg
}

func (b *Block) GetBlockHash() [32]byte {
	return sha256.Sum256(b.MarshalBlock())
}

func (b *Block) Sign(signer *Account) string {
	signature := dissycrypto.RSASign(b.marshalForSignature(), signer.d, signer.n)
	return encode(signature.Bytes())
}

func lotteryProofMessage(seed int, slot int) []byte {
	msg, err := json.Marshal([]string{"pokoin-lottery-v1", strconv.Itoa(seed), strconv.Itoa(slot)})
	if err != nil {
		return []byte{}
	}
	return msg
}

func SignLotteryProof(seed int, slot int, signer *Account) string {
	if signer == nil {
		return ""
	}
	signature := dissycrypto.RSASign(lotteryProofMessage(seed, slot), signer.d, signer.n)
	return encode(signature.Bytes())
}

func ComputeLotteryDrawFromProof(proof string) (int, bool) {
	raw, err := decode(proof)
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	hash := sha256.Sum256(raw)
	v := binary.BigEndian.Uint64(hash[:8])
	return int(v % LOTTERY_DRAW_RANGE), true
}

func VerifyLotteryProof(seed int, slot int, accountName string, proof string) bool {
	signature, err := decode(proof)
	if err != nil {
		return false
	}
	publicKey, err := decode(accountName)
	if err != nil {
		return false
	}
	return dissycrypto.VerifySignature(lotteryProofMessage(seed, slot), signature, string(publicKey))
}

type Blockchain struct {
	Blocks []Block
	lock   sync.Mutex
}

func (b *Blockchain) BlocksSnapshot() []Block {
	b.lock.Lock()
	defer b.lock.Unlock()
	out := make([]Block, len(b.Blocks))
	copy(out, b.Blocks)
	return out
}

func (b *Blockchain) CanonicalBlocks(rollback int) []Block {
	b.lock.Lock()
	defer b.lock.Unlock()
	depths, nodes := b.calculateDepths()
	leaf := getLeafHash(depths)
	blocks := getBlocksFromLongestBranch(nodes, leaf)
	if rollback > 0 && rollback < len(blocks) {
		blocks = blocks[:len(blocks)-rollback]
	} else if rollback >= len(blocks) {
		if len(blocks) > 0 {
			blocks = blocks[:1]
		}
	}
	out := make([]Block, len(blocks))
	for i, block := range blocks {
		out[i] = *block
	}
	return out
}

func (b *Blockchain) ReplaceBlocks(blocks []Block) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if len(blocks) == 0 {
		return
	}
	b.Blocks = make([]Block, len(blocks))
	copy(b.Blocks, blocks)
}

func NewBlockchain() *Blockchain {
	return NewBlockchainWithGenesis(makeGenesisMetaData())
}

func NewBlockchainWithGenesis(genesis *GenesisMetaData) *Blockchain {
	b := new(Blockchain)
	meta, err := marshalGenesisMetaData(genesis)
	if err != nil {
		meta = []byte("{}")
	}
	b.Blocks = append(b.Blocks, Block{
		Type:       BLOCK_TYPE,
		Slot:       0,
		Draw:       0,
		MetaData:   encode(meta),
		ParentHash: NO_PARENT_HASH,
	})
	return b
}

// EncodeBlockTransactions serializes transactions for block metadata.
func EncodeBlockTransactions(txs []SignedTransaction) string {
	raw, err := json.Marshal(txs)
	if err != nil {
		raw = []byte("[]")
	}
	return encode(raw)
}

// DecodeBlockTransactions deserializes block metadata into transactions.
func DecodeBlockTransactions(meta string) ([]SignedTransaction, error) {
	if meta == "" {
		return []SignedTransaction{}, nil
	}
	decoded, err := decode(meta)
	if err != nil {
		// Allow plain JSON too for robustness.
		decoded = []byte(meta)
	}
	var txs []SignedTransaction
	if err := json.Unmarshal(decoded, &txs); err != nil {
		return nil, err
	}
	return txs, nil
}

// ComputeLotteryDraw computes a deterministic draw for (seed, slot, account).
func ComputeLotteryDraw(seed int, slot int, accountName string) int {
	raw := []byte(strconv.Itoa(seed) + "|" + strconv.Itoa(slot) + "|" + accountName)
	hash := sha256.Sum256(raw)
	v := binary.BigEndian.Uint64(hash[:8])
	return int(v % LOTTERY_DRAW_RANGE)
}

// WinsLottery checks static PoS winning condition.
func WinsLottery(tickets int, hardness int, draw int) bool {
	// More tickets => higher threshold => higher chance to win.
	threshold := uint64(hardness) * uint64(tickets)
	return uint64(draw) < threshold
}

func EffectiveHardnessForMining(blocks []Block, genesis *GenesisMetaData, candidateTxCount int) int {
	if genesis == nil {
		return 0
	}
	applyDefaultDynamicHardnessConfig(genesis)
	if len(blocks) == 0 || genesis.DynamicHardnessForkHeight <= DYNAMIC_HARDNESS_DISABLED || len(blocks) < genesis.DynamicHardnessForkHeight {
		return clampHardness(genesis.Hardness, genesis.DynamicHardnessMin, genesis.DynamicHardnessMax)
	}
	return effectiveDynamicHardness(blocks, genesis, candidateTxCount)
}

func effectiveHardnessForBlock(nodes map[string]*Block, genesis *GenesisMetaData, block *Block) int {
	if genesis == nil || block == nil {
		return 0
	}
	parentBlocks := getBlocksFromLongestBranch(nodes, block.ParentHash)
	txs, err := DecodeBlockTransactions(block.MetaData)
	if err != nil {
		txs = nil
	}
	return EffectiveHardnessForMining(blockPointersToValues(parentBlocks), genesis, len(txs))
}

func effectiveDynamicHardness(parentBlocks []Block, genesis *GenesisMetaData, candidateTxCount int) int {
	target := genesis.IdleBlockTargetSlots
	if candidateTxCount > 0 {
		target = genesis.FastBlockTargetSlots
	}
	if target <= 0 {
		target = 1
	}
	window := genesis.DynamicHardnessWindow
	if window <= 0 {
		window = DYNAMIC_HARDNESS_DEFAULT_WINDOW
	}
	avgInterval := averageRecentSlotInterval(parentBlocks, window)
	if avgInterval <= 0 {
		return clampHardness(genesis.Hardness, genesis.DynamicHardnessMin, genesis.DynamicHardnessMax)
	}
	adjusted := int((int64(genesis.Hardness) * int64(avgInterval)) / int64(target))
	return clampHardness(adjusted, genesis.DynamicHardnessMin, genesis.DynamicHardnessMax)
}

func averageRecentSlotInterval(blocks []Block, window int) int {
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

func clampHardness(value int, min int, max int) int {
	if min <= 0 {
		min = 1
	}
	if max < min {
		max = min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// BestLeafHash returns hash of current best/longest branch leaf.
func (b *Blockchain) BestLeafHash() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	depths, _ := b.calculateDepths()
	return getLeafHash(depths)
}

// BestHeight returns the height of the longest branch.
func (b *Blockchain) BestHeight() int {
	b.lock.Lock()
	defer b.lock.Unlock()
	depths, _ := b.calculateDepths()
	maxDepth := 0
	for _, depth := range depths {
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// AddBlock validates a block and appends it if valid.
func (b *Blockchain) AddBlock(block *Block) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	depths, nodes := b.calculateDepths()
	if _, parentExists := depths[block.ParentHash]; !parentExists {
		return false
	}

	// Block signature must be valid.
	signature, err := decode(block.Signature)
	if err != nil {
		return false
	}
	publicKey, err := decode(block.VerificationKey)
	if err != nil {
		return false
	}
	if !dissycrypto.VerifySignature(block.marshalForSignature(), signature, string(publicKey)) {
		return false
	}

	// Extract genesis settings to validate lottery.
	genesis, err := decodeGenesisFromMeta(nodes[getGenesisHash(nodes)].MetaData)
	if err != nil {
		return false
	}
	// Any P2P node with a valid miner key can validate. Once a miner appears in
	// the chain, it stays in the validator registry independently of balance.
	parentLedger := buildLedgerAtHash(nodes, block.ParentHash)
	tickets := parentLedger.LotteryTickets(block.VerificationKey)
	if tickets <= 0 {
		return false
	}
	if block.LotteryProof != "" {
		if !VerifyLotteryProof(genesis.Seed, block.Slot, block.VerificationKey, block.LotteryProof) {
			return false
		}
		expectedDraw, ok := ComputeLotteryDrawFromProof(block.LotteryProof)
		if !ok || block.Draw != expectedDraw {
			return false
		}
	} else {
		expectedDraw := ComputeLotteryDraw(genesis.Seed, block.Slot, block.VerificationKey)
		if block.Draw != expectedDraw {
			return false
		}
	}
	hardness := effectiveHardnessForBlock(nodes, genesis, block)
	if !parentLedger.IsValidator(block.VerificationKey) && parentLedger.GetBalance(block.VerificationKey) == 0 && genesis.Hardness > hardness {
		hardness = genesis.Hardness
	}
	if !WinsLottery(tickets, hardness, block.Draw) {
		return false
	}

	// Validate transactions against parent state.
	ledger := parentLedger
	txs, err := DecodeBlockTransactions(block.MetaData)
	if err != nil {
		return false
	}
	if len(txs) > BLOCK_SIZE {
		return false
	}
	for i := range txs {
		if !ledger.Transaction(&txs[i]) {
			return false
		}
	}
	// Apply miner reward after all txs are valid.
	ledger.MarkValidator(block.VerificationKey)
	ledger.Accounts[block.VerificationKey] += MINER_BLOCK_REWARD

	b.Blocks = append(b.Blocks, *block)
	return true
}

// Get all transactions in the current longest branch (except those in the <rollback> latest blocks).
func (b *Blockchain) GetCurrentTransactions(rollback int) []string {
	b.lock.Lock()
	defer b.lock.Unlock()
	depths, nodes := b.calculateDepths()
	leaf := getLeafHash(depths)
	meta := getMetaFromLongestBranch(nodes, leaf)
	if rollback <= 0 {
		return meta
	}
	if rollback >= len(meta) {
		return []string{}
	}
	return meta[:len(meta)-rollback]
}

// Get the depths of all nodes starting from the genesis.
func (b *Blockchain) calculateDepths() (map[string]int, map[string]*Block) {
	// Maps from encoded block hash to depth/block ptr, respectively.
	depths := make(map[string]int)
	nodes := make(map[string]*Block)
	// Initialize with the genesis node.
	genesisHash := b.Blocks[0].GetBlockHash()
	depths[encode(genesisHash[:])] = 0
	nodes[encode(genesisHash[:])] = &b.Blocks[0]
	// Go over all blocks and determine their depths, if they are valid.
	for i := 1; i < len(b.Blocks); i++ {
		block := &b.Blocks[i]
		// Verify that the block is valid before applying.
		signature, err := decode(block.Signature)
		if err != nil {
			continue
		}
		// Verification key is stored base64-encoded in Block; dissycrypto expects raw bytes as string.
		publicKey, err := decode(block.VerificationKey)
		if err != nil {
			continue
		}
		if !dissycrypto.VerifySignature(block.marshalForSignature(), signature, string(publicKey)) {
			continue
		}
		parentDepth, exists := depths[block.ParentHash]
		if !exists {
			continue // Parent is an invalid node (most likely because it failed verification).
		}
		hash := block.GetBlockHash()
		depths[encode(hash[:])] = parentDepth + 1
		nodes[encode(hash[:])] = block
	}
	return depths, nodes
}

// Find which node is the leaf of the longest path.
func getLeafHash(depths map[string]int) string {
	maxDepth := -1
	var leaf string
	for hash, depth := range depths {
		if depth > maxDepth {
			maxDepth = depth
			leaf = hash
		} else if depth == maxDepth && hash < leaf { // Tiebreaker: Use the lexicographically smallest hash.
			leaf = hash
		}
	}
	return leaf
}

// Extract the metadata from the blocks along the longest branch.
func getMetaFromLongestBranch(nodes map[string]*Block, leaf string) []string {
	var meta []string
	next := leaf
	// Start from the leaf and work backwards through the tree until the genesis node is encountered.
	for next != NO_PARENT_HASH {
		node, exists := nodes[next]
		if !exists {
			break
		}
		meta = append(meta, node.MetaData)
		next = node.ParentHash
	}
	// Reverse so output is genesis -> leaf order.
	for left, right := 0, len(meta)-1; left < right; left, right = left+1, right-1 {
		meta[left], meta[right] = meta[right], meta[left]
	}
	return meta
}

// Given a list of transactions in Total Order, create a ledger.
func LedgerFromTransactions(transactions []string) *Ledger {
	ledger := MakeLedger()
	if len(transactions) == 0 {
		return ledger
	}

	var genesis GenesisMetaData
	if err := json.Unmarshal([]byte(transactions[0]), &genesis); err != nil {
		// Genesis metadata is currently stored base64-encoded in block metadata.
		decoded, decodeErr := decode(transactions[0])
		if decodeErr == nil {
			_ = json.Unmarshal(decoded, &genesis)
		}
	}
	ledger.InitializeFromGenesis(&genesis)
	if len(transactions) == 1 {
		return ledger
	}
	// Apply transactions.
	for _, txString := range transactions[1:] {
		var tx SignedTransaction
		if err := json.Unmarshal([]byte(txString), &tx); err != nil {
			decoded, decodeErr := decode(txString)
			if decodeErr != nil {
				continue
			}
			if err = json.Unmarshal(decoded, &tx); err != nil {
				continue
			}
		}
		_ = ledger.Transaction(&tx)
	}
	return ledger
}

// LedgerFromBlockchain reconstructs balances from best path.
func LedgerFromBlockchain(b *Blockchain, rollback int) *Ledger {
	b.lock.Lock()
	defer b.lock.Unlock()
	depths, nodes := b.calculateDepths()
	leaf := getLeafHash(depths)
	blocks := getBlocksFromLongestBranch(nodes, leaf)
	if rollback > 0 && rollback < len(blocks) {
		blocks = blocks[:len(blocks)-rollback]
	} else if rollback >= len(blocks) {
		if len(blocks) > 0 {
			blocks = blocks[:1]
		}
	}
	return buildLedgerFromBlocks(blocks)
}

func getBlocksFromLongestBranch(nodes map[string]*Block, leaf string) []*Block {
	blocks := make([]*Block, 0)
	next := leaf
	for next != NO_PARENT_HASH {
		node, exists := nodes[next]
		if !exists {
			break
		}
		blocks = append(blocks, node)
		next = node.ParentHash
	}
	// Reverse to genesis -> leaf.
	for left, right := 0, len(blocks)-1; left < right; left, right = left+1, right-1 {
		blocks[left], blocks[right] = blocks[right], blocks[left]
	}
	return blocks
}

func blockPointersToValues(blocks []*Block) []Block {
	out := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		if block != nil {
			out = append(out, *block)
		}
	}
	return out
}

func getGenesisHash(nodes map[string]*Block) string {
	for hash, block := range nodes {
		if block.ParentHash == NO_PARENT_HASH {
			return hash
		}
	}
	return ""
}

func decodeGenesisFromMeta(meta string) (*GenesisMetaData, error) {
	var genesis GenesisMetaData
	if err := json.Unmarshal([]byte(meta), &genesis); err == nil {
		return &genesis, nil
	}
	decoded, err := decode(meta)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(decoded, &genesis); err != nil {
		return nil, err
	}
	return &genesis, nil
}

func DecodeGenesisFromBlock(block Block) (*GenesisMetaData, error) {
	return decodeGenesisFromMeta(block.MetaData)
}

func buildLedgerAtHash(nodes map[string]*Block, hash string) *Ledger {
	chain := make([]*Block, 0)
	next := hash
	for next != NO_PARENT_HASH {
		node, exists := nodes[next]
		if !exists {
			break
		}
		chain = append(chain, node)
		next = node.ParentHash
	}
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return buildLedgerFromBlocks(chain)
}

func buildLedgerFromBlocks(blocks []*Block) *Ledger {
	ledger := MakeLedger()
	if len(blocks) == 0 {
		return ledger
	}

	genesis, err := decodeGenesisFromMeta(blocks[0].MetaData)
	if err != nil {
		return ledger
	}
	ledger.InitializeFromGenesis(genesis)

	for i := 1; i < len(blocks); i++ {
		block := blocks[i]
		ledger.MarkValidator(block.VerificationKey)
		txs, err := DecodeBlockTransactions(block.MetaData)
		if err != nil {
			continue
		}
		for j := range txs {
			_ = ledger.Transaction(&txs[j])
		}
		// Miner reward.
		ledger.Accounts[block.VerificationKey] += MINER_BLOCK_REWARD
	}
	return ledger
}

// NewCandidateBlock builds a block for a winner from parent hash and tx list.
func NewCandidateBlock(winner *Account, slot int, parentHash string, txs []SignedTransaction, seed int) *Block {
	proof := SignLotteryProof(seed, slot, winner)
	draw, ok := ComputeLotteryDrawFromProof(proof)
	if !ok {
		draw = ComputeLotteryDraw(seed, slot, winner.SafeEncode())
	}
	meta := EncodeBlockTransactions(txs)
	block := NewBlock(winner, slot, draw, parentHash, meta)
	block.LotteryProof = proof
	block.Signature = block.Sign(winner)
	return block
}

func (b *Block) String() string {
	return fmt.Sprintf("Block(slot=%d, winner=%s, parent=%s)", b.Slot, b.VerificationKey, b.ParentHash)
}
