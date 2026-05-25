package account

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/big"
	"peer/dissycrypto"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

type SignedTransaction struct {
	ID        string
	From      string
	To        string
	Amount    int
	Signature string
	Kind      string
	Nonce     uint64
	Raw       string
	ChainID   int64
	NFT       *NFTPayload
	AMM       *AMMPayload
}

const (
	AssetPKN                 = "PKN"
	AssetWPKN                = "WPKN"
	AMMFeeBps                = 30
	AMMDenominatorBps        = 10000
	PokoinSwapRouterAddress  = "0x0000000000000000000000000000000000002606"
	PokoinSwapCalldataPrefix = "pokoinswap:v1:"
)

type NFTPayload struct {
	CollectionID string `json:"collectionId"`
	TokenID      string `json:"tokenId"`
	Owner        string `json:"owner,omitempty"`
	MetadataURI  string `json:"metadataUri,omitempty"`
	MetadataHash string `json:"metadataHash,omitempty"`
	ImageURI     string `json:"imageUri,omitempty"`
	Burned       bool   `json:"burned,omitempty"`
}

type NFTToken struct {
	CollectionID string `json:"collectionId"`
	TokenID      string `json:"tokenId"`
	Owner        string `json:"owner"`
	Minter       string `json:"minter"`
	MetadataURI  string `json:"metadataUri,omitempty"`
	MetadataHash string `json:"metadataHash,omitempty"`
	ImageURI     string `json:"imageUri,omitempty"`
	MintTx       string `json:"mintTx"`
	LastTx       string `json:"lastTx"`
	Burned       bool   `json:"burned,omitempty"`
}

type AMMPayload struct {
	Action       string `json:"action"`
	PoolID       string `json:"poolId,omitempty"`
	Asset        string `json:"asset,omitempty"`
	AssetA       string `json:"assetA,omitempty"`
	AssetB       string `json:"assetB,omitempty"`
	AssetIn      string `json:"assetIn,omitempty"`
	AssetOut     string `json:"assetOut,omitempty"`
	Amount       int    `json:"amount,omitempty"`
	AmountA      int    `json:"amountA,omitempty"`
	AmountB      int    `json:"amountB,omitempty"`
	AmountIn     int    `json:"amountIn,omitempty"`
	MinAmountOut int    `json:"minAmountOut,omitempty"`
	AmountOut    int    `json:"amountOut,omitempty"`
	FeeBps       int    `json:"feeBps,omitempty"`
}

type AMMPool struct {
	ID        string `json:"id"`
	AssetA    string `json:"assetA"`
	AssetB    string `json:"assetB"`
	ReserveA  int    `json:"reserveA"`
	ReserveB  int    `json:"reserveB"`
	FeeBps    int    `json:"feeBps"`
	CreatedTx string `json:"createdTx,omitempty"`
	LastTx    string `json:"lastTx,omitempty"`
}

type AMMQuote struct {
	PoolID           string `json:"poolId"`
	AssetIn          string `json:"assetIn"`
	AssetOut         string `json:"assetOut"`
	AmountIn         int    `json:"amountIn"`
	AmountOut        int    `json:"amountOut"`
	FeeBps           int    `json:"feeBps"`
	ReserveIn        int    `json:"reserveIn"`
	ReserveOut       int    `json:"reserveOut"`
	PriceNumerator   int    `json:"priceNumerator"`
	PriceDenominator int    `json:"priceDenominator"`
}

var EVMUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// Perform transactions. Checks whether transaction is authentic before applying.
func (l *Ledger) Transaction(t *SignedTransaction) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	if t.IsAMM() {
		return l.applyAMMTransactionLocked(t)
	}
	if t.IsNFTMint() {
		if !t.validNFTMint() || !t.Verify(t.From) {
			return false
		}
		key := NFTKey(t.NFT.CollectionID, t.NFT.TokenID)
		if key == "" {
			return false
		}
		if _, exists := l.NFTs[key]; exists {
			return false
		}
		owner := normalizeNFTAccount(t.NFT.Owner)
		if owner == "" {
			owner = normalizeNFTAccount(t.To)
		}
		if owner == "" {
			return false
		}
		l.TxHistory[t.ID] = *t
		l.NFTs[key] = NFTToken{
			CollectionID: t.NFT.CollectionID,
			TokenID:      t.NFT.TokenID,
			Owner:        owner,
			Minter:       t.From,
			MetadataURI:  t.NFT.MetadataURI,
			MetadataHash: t.NFT.MetadataHash,
			ImageURI:     t.NFT.ImageURI,
			MintTx:       t.ID,
			LastTx:       t.ID,
		}
		return true
	}
	if t.IsNFTTransfer() {
		if !t.validNFTTransfer() || !t.Verify(t.From) {
			return false
		}
		key := NFTKey(t.NFT.CollectionID, t.NFT.TokenID)
		token, exists := l.NFTs[key]
		if !exists || token.Burned || !sameNFTAccount(token.Owner, t.From) {
			return false
		}
		token.Owner = normalizeNFTAccount(t.To)
		token.LastTx = t.ID
		l.NFTs[key] = token
		l.TxHistory[t.ID] = *t
		return true
	}
	if t.IsMint() {
		if t.Amount < 1 || l.Accounts[t.From] <= 0 || !t.Verify(t.From) {
			return false
		}
		l.TxHistory[t.ID] = *t
		l.Accounts[t.To] += t.Amount
		if isNativeValidatorAccount(t.To) {
			l.Validators[t.To] = true
		}
		return true
	}
	// Native RSA transactions must transfer a positive amount. EVM zero-value
	// transactions are allowed so wallets can cancel/replace pending nonces.
	if t.Amount < 0 || (!t.IsEVM() && t.Amount < 1) {
		return false
	}
	if !t.Verify(t.From) {
		return false
	}
	if t.IsEVM() {
		if l.Nonces[t.From] != t.Nonce {
			return false
		}
		if t.Raw != "" {
			if applied := l.applyEVMContractTransactionLocked(t); applied != nil {
				return *applied
			}
		}
	}
	// Exercise 16.2: reject overdrafts (no negative balances).
	if l.Accounts[t.From] < t.Amount {
		return false
	}

	l.TxHistory[t.ID] = *t

	l.Accounts[t.From] -= t.Amount
	l.Accounts[t.To] += t.Amount
	if t.IsEVM() {
		l.Nonces[t.From] = t.Nonce + 1
	}
	return true
}

func (l *Ledger) applyAMMTransactionLocked(t *SignedTransaction) bool {
	if t.AMM == nil || !t.validAMM() || !t.Verify(t.From) {
		return false
	}
	if t.IsEVM() {
		if l.Nonces[t.From] != t.Nonce {
			return false
		}
	}
	advanceNonce := func() {
		if t.IsEVM() {
			l.Nonces[t.From] = t.Nonce + 1
		}
	}
	switch t.AMM.Action {
	case "asset_credit":
		if !l.isOperatorLocked(t.From) || t.To == "" || t.AMM.Asset == AssetPKN || t.AMM.Amount < 1 {
			return false
		}
		l.creditAssetLocked(t.AMM.Asset, t.To, t.AMM.Amount)
	case "asset_debit":
		if !l.isOperatorLocked(t.From) || t.To == "" || t.AMM.Asset == AssetPKN || t.AMM.Amount < 1 {
			return false
		}
		if !l.debitAssetLocked(t.AMM.Asset, t.To, t.AMM.Amount) {
			return false
		}
	case "amm_create_pool":
		if !l.isOperatorLocked(t.From) {
			return false
		}
		poolID := PoolID(t.AMM.AssetA, t.AMM.AssetB)
		if poolID == "" {
			return false
		}
		if _, exists := l.AMMPools[poolID]; exists {
			return false
		}
		l.AMMPools[poolID] = AMMPool{ID: poolID, AssetA: normalizeAsset(t.AMM.AssetA), AssetB: normalizeAsset(t.AMM.AssetB), FeeBps: AMMFeeBps, CreatedTx: t.ID, LastTx: t.ID}
	case "amm_add_liquidity":
		if !l.isOperatorLocked(t.From) || t.AMM.AmountA < 1 || t.AMM.AmountB < 1 {
			return false
		}
		pool, exists := l.AMMPools[normalizePoolID(t.AMM.PoolID)]
		if !exists {
			return false
		}
		if !l.canDebitAssetLocked(pool.AssetA, t.From, t.AMM.AmountA) || !l.canDebitAssetLocked(pool.AssetB, t.From, t.AMM.AmountB) {
			return false
		}
		_ = l.debitAssetOrPKNLocked(pool.AssetA, t.From, t.AMM.AmountA)
		_ = l.debitAssetOrPKNLocked(pool.AssetB, t.From, t.AMM.AmountB)
		pool.ReserveA += t.AMM.AmountA
		pool.ReserveB += t.AMM.AmountB
		pool.LastTx = t.ID
		l.AMMPools[pool.ID] = pool
	case "amm_swap":
		if t.AMM.AmountIn < 1 || t.AMM.MinAmountOut < 0 {
			return false
		}
		pool, exists := l.AMMPools[normalizePoolID(t.AMM.PoolID)]
		if !exists {
			return false
		}
		amountOut, ok := QuoteSwap(pool, t.AMM.AssetIn, t.AMM.AmountIn)
		if !ok || amountOut < t.AMM.MinAmountOut || amountOut < 1 {
			return false
		}
		if !l.canDebitAssetLocked(t.AMM.AssetIn, t.From, t.AMM.AmountIn) {
			return false
		}
		_ = l.debitAssetOrPKNLocked(t.AMM.AssetIn, t.From, t.AMM.AmountIn)
		if !pool.applySwap(t.AMM.AssetIn, t.AMM.AmountIn, amountOut) {
			return false
		}
		l.creditAssetOrPKNLocked(t.AMM.AssetOut, t.From, amountOut)
		t.AMM.AmountOut = amountOut
		pool.LastTx = t.ID
		l.AMMPools[pool.ID] = pool
	default:
		return false
	}
	l.TxHistory[t.ID] = *t
	advanceNonce()
	return true
}

func (l *Ledger) applyEVMContractTransactionLocked(t *SignedTransaction) *bool {
	tx, err := t.decodeEVM()
	if err != nil {
		out := false
		return &out
	}
	if tx.To() != nil && len(tx.Data()) == 0 {
		return nil
	}
	if t.Amount > 0 && l.Accounts[t.From] < t.Amount {
		out := false
		return &out
	}
	state := newEVMStateAdapter(l)
	origin := common.HexToAddress(t.From)
	evm := newLedgerEVM(state, origin)
	gasLimit := tx.Gas()
	if gasLimit == 0 {
		gasLimit = 5_000_000
	}
	gas := vm.NewGasBudget(gasLimit)
	value, overflow := uint256.FromBig(tx.Value())
	if overflow {
		out := false
		return &out
	}
	var callErr error
	contractAddress := ""
	if tx.To() == nil {
		_, created, left, err := evm.Create(origin, tx.Data(), gas, value)
		gas = left
		contractAddress = strings.ToLower(created.Hex())
		callErr = err
	} else {
		_, left, err := evm.Call(origin, *tx.To(), tx.Data(), gas, value)
		gas = left
		callErr = err
	}
	if callErr != nil {
		out := false
		return &out
	}
	if t.Amount > 0 {
		l.Accounts[t.From] -= t.Amount
		if tx.To() != nil {
			l.Accounts[strings.ToLower(tx.To().Hex())] += t.Amount
		}
	}
	l.Nonces[t.From] = t.Nonce + 1
	l.TxHistory[t.ID] = *t
	l.EVMReceipts[strings.ToLower(t.ID)] = EVMReceipt{
		TransactionHash: strings.ToLower(t.ID),
		ContractAddress: contractAddress,
		GasUsed:         gas.Used(vm.NewGasBudget(gasLimit)),
		Logs:            evmLogsFromTypes(state.logs),
		Status:          1,
	}
	out := true
	return &out
}

func (l *Ledger) HasRecordedTransaction(tx *SignedTransaction) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	_, exists := l.TxHistory[tx.ID]
	return exists
}

func NewSignedTransaction(ID string, From *Account, To string, Amount int) *SignedTransaction {
	s := new(SignedTransaction)
	s.ID = ID
	s.From = From.SafeEncode()
	s.To = To
	s.Amount = Amount
	s.signTransaction(From.d, From.n)
	return s
}

func NewMintTransaction(ID string, From *Account, To string, Amount int) *SignedTransaction {
	s := NewSignedTransaction(ID, From, To, Amount)
	s.Kind = "mint"
	s.signTransaction(From.d, From.n)
	return s
}

func NewNFTMintTransaction(id string, from *Account, collectionID string, tokenID string, owner string, metadataURI string, metadataHash string, imageURI string) *SignedTransaction {
	s := &SignedTransaction{
		ID:   id,
		From: from.SafeEncode(),
		To:   normalizeNFTAccount(owner),
		Kind: "nft_mint",
		NFT: &NFTPayload{
			CollectionID: normalizeNFTID(collectionID),
			TokenID:      normalizeNFTID(tokenID),
			Owner:        normalizeNFTAccount(owner),
			MetadataURI:  strings.TrimSpace(metadataURI),
			MetadataHash: strings.TrimSpace(metadataHash),
			ImageURI:     strings.TrimSpace(imageURI),
		},
	}
	s.signTransaction(from.d, from.n)
	return s
}

func NewNFTTransferTransaction(id string, from *Account, collectionID string, tokenID string, to string) *SignedTransaction {
	s := &SignedTransaction{
		ID:   id,
		From: from.SafeEncode(),
		To:   normalizeNFTAccount(to),
		Kind: "nft_transfer",
		NFT: &NFTPayload{
			CollectionID: normalizeNFTID(collectionID),
			TokenID:      normalizeNFTID(tokenID),
		},
	}
	s.signTransaction(from.d, from.n)
	return s
}

func NewAssetCreditTransaction(id string, from *Account, asset string, to string, amount int) *SignedTransaction {
	return newOperatorAMMTransaction(id, from, to, &AMMPayload{Action: "asset_credit", Asset: asset, Amount: amount})
}

func NewAssetDebitTransaction(id string, from *Account, asset string, accountName string, amount int) *SignedTransaction {
	return newOperatorAMMTransaction(id, from, accountName, &AMMPayload{Action: "asset_debit", Asset: asset, Amount: amount})
}

func NewAMMCreatePoolTransaction(id string, from *Account, assetA string, assetB string) *SignedTransaction {
	amm := &AMMPayload{Action: "amm_create_pool", AssetA: assetA, AssetB: assetB, PoolID: PoolID(assetA, assetB)}
	return newOperatorAMMTransaction(id, from, "", amm)
}

func NewAMMAddLiquidityTransaction(id string, from *Account, poolID string, amountA int, amountB int) *SignedTransaction {
	return newOperatorAMMTransaction(id, from, "", &AMMPayload{Action: "amm_add_liquidity", PoolID: poolID, AmountA: amountA, AmountB: amountB})
}

func newOperatorAMMTransaction(id string, from *Account, to string, amm *AMMPayload) *SignedTransaction {
	s := &SignedTransaction{ID: id, From: from.SafeEncode(), To: normalizeAssetAccount(to), Kind: amm.Action, AMM: amm}
	s.normalizeAMM()
	s.signTransaction(from.d, from.n)
	return s
}

func NewEVMTransaction(raw string, chainID int64) (*SignedTransaction, error) {
	normalized := strings.TrimPrefix(strings.TrimSpace(raw), "0x")
	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex("0x" + normalized)); err != nil {
		return nil, err
	}
	signer := types.LatestSignerForChainID(big.NewInt(chainID))
	from, err := types.Sender(signer, &tx)
	if err != nil {
		return nil, err
	}
	to := ""
	if tx.To() != nil {
		to = strings.ToLower(tx.To().Hex())
	}
	value := tx.Value()
	if value == nil || value.Sign() < 0 {
		return nil, fmt.Errorf("evm value must be non-negative")
	}
	amount, remainder := new(big.Int).QuoRem(value, EVMUnit, new(big.Int))
	if remainder.Sign() != 0 {
		return nil, fmt.Errorf("evm value must be a whole PKN amount")
	}
	if !amount.IsInt64() || amount.Int64() > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("evm value must fit native amount")
	}
	s := &SignedTransaction{
		ID:        strings.ToLower(tx.Hash().Hex()),
		From:      strings.ToLower(from.Hex()),
		To:        to,
		Amount:    int(amount.Int64()),
		Signature: "evm:" + strings.TrimPrefix(strings.TrimSpace(raw), "0x"),
		Kind:      "evm",
		Nonce:     tx.Nonce(),
		Raw:       "0x" + normalized,
		ChainID:   chainID,
	}
	if amm, ok := DecodePokoinSwapCalldata(tx.Data()); ok {
		s.Kind = amm.Action
		s.AMM = &amm
		s.Amount = 0
	}
	return s, nil
}

func (s *SignedTransaction) IsEVM() bool {
	return s.Kind == "evm" || strings.HasPrefix(s.Signature, "evm:")
}

func (s *SignedTransaction) IsMint() bool {
	return s.Kind == "mint"
}

func (s *SignedTransaction) IsNFT() bool {
	return s.IsNFTMint() || s.IsNFTTransfer()
}

func (s *SignedTransaction) IsNFTMint() bool {
	return s.Kind == "nft_mint"
}

func (s *SignedTransaction) IsNFTTransfer() bool {
	return s.Kind == "nft_transfer"
}

func (s *SignedTransaction) IsAMM() bool {
	return s.AMM != nil || strings.HasPrefix(s.Kind, "amm_") || strings.HasPrefix(s.Kind, "asset_")
}

func (s *SignedTransaction) normalizeAMM() {
	if s.AMM == nil {
		return
	}
	s.AMM.Action = strings.ToLower(strings.TrimSpace(s.AMM.Action))
	s.AMM.Asset = normalizeAsset(s.AMM.Asset)
	s.AMM.AssetA = normalizeAsset(s.AMM.AssetA)
	s.AMM.AssetB = normalizeAsset(s.AMM.AssetB)
	s.AMM.AssetIn = normalizeAsset(s.AMM.AssetIn)
	s.AMM.AssetOut = normalizeAsset(s.AMM.AssetOut)
	s.AMM.PoolID = normalizePoolID(s.AMM.PoolID)
	if s.AMM.PoolID == "" && s.AMM.AssetA != "" && s.AMM.AssetB != "" {
		s.AMM.PoolID = PoolID(s.AMM.AssetA, s.AMM.AssetB)
	}
	if s.AMM.FeeBps == 0 {
		s.AMM.FeeBps = AMMFeeBps
	}
	s.Kind = s.AMM.Action
	s.To = normalizeAssetAccount(s.To)
}

func (s *SignedTransaction) validAMM() bool {
	if s.AMM == nil {
		return false
	}
	s.normalizeAMM()
	if s.ID == "" || s.From == "" || s.AMM.Action == "" {
		return false
	}
	switch s.AMM.Action {
	case "asset_credit", "asset_debit":
		return s.To != "" && s.AMM.Asset != "" && s.AMM.Asset != AssetPKN && s.AMM.Amount > 0
	case "amm_create_pool":
		return s.AMM.PoolID != "" && s.AMM.AssetA != "" && s.AMM.AssetB != "" && s.AMM.AssetA != s.AMM.AssetB
	case "amm_add_liquidity":
		return s.AMM.PoolID != "" && s.AMM.AmountA > 0 && s.AMM.AmountB > 0
	case "amm_swap":
		return s.AMM.PoolID != "" && s.AMM.AssetIn != "" && s.AMM.AssetOut != "" && s.AMM.AssetIn != s.AMM.AssetOut && s.AMM.AmountIn > 0 && s.AMM.MinAmountOut >= 0
	default:
		return false
	}
}

func (s *SignedTransaction) validNFTMint() bool {
	if s.NFT == nil {
		return false
	}
	s.NFT.CollectionID = normalizeNFTID(s.NFT.CollectionID)
	s.NFT.TokenID = normalizeNFTID(s.NFT.TokenID)
	s.NFT.Owner = normalizeNFTAccount(s.NFT.Owner)
	s.To = normalizeNFTAccount(s.To)
	return s.ID != "" && s.From != "" && s.NFT.CollectionID != "" && s.NFT.TokenID != "" && (s.NFT.Owner != "" || s.To != "")
}

func (s *SignedTransaction) validNFTTransfer() bool {
	if s.NFT == nil {
		return false
	}
	s.NFT.CollectionID = normalizeNFTID(s.NFT.CollectionID)
	s.NFT.TokenID = normalizeNFTID(s.NFT.TokenID)
	s.To = normalizeNFTAccount(s.To)
	return s.ID != "" && s.From != "" && s.To != "" && s.NFT.CollectionID != "" && s.NFT.TokenID != ""
}

func (s *SignedTransaction) marshalContentForSignature() ([]byte, error) {
	if s.IsAMM() {
		amm := AMMPayload{}
		if s.AMM != nil {
			amm = *s.AMM
		}
		amm.Action = strings.ToLower(strings.TrimSpace(amm.Action))
		amm.Asset = normalizeAsset(amm.Asset)
		amm.AssetA = normalizeAsset(amm.AssetA)
		amm.AssetB = normalizeAsset(amm.AssetB)
		amm.AssetIn = normalizeAsset(amm.AssetIn)
		amm.AssetOut = normalizeAsset(amm.AssetOut)
		amm.PoolID = normalizePoolID(amm.PoolID)
		return json.Marshal([]string{s.ID, s.From, normalizeAssetAccount(s.To), amm.Action, amm.PoolID, amm.Asset, amm.AssetA, amm.AssetB, amm.AssetIn, amm.AssetOut, strconv.Itoa(amm.Amount), strconv.Itoa(amm.AmountA), strconv.Itoa(amm.AmountB), strconv.Itoa(amm.AmountIn), strconv.Itoa(amm.MinAmountOut)})
	}
	if s.IsNFT() {
		nft := NFTPayload{}
		if s.NFT != nil {
			nft = *s.NFT
		}
		nft.CollectionID = normalizeNFTID(nft.CollectionID)
		nft.TokenID = normalizeNFTID(nft.TokenID)
		nft.Owner = normalizeNFTAccount(nft.Owner)
		return json.Marshal([]string{s.ID, s.From, normalizeNFTAccount(s.To), s.Kind, nft.CollectionID, nft.TokenID, nft.Owner, nft.MetadataURI, nft.MetadataHash, nft.ImageURI})
	}
	return json.Marshal([]string{s.ID, s.From, s.To, strconv.Itoa(s.Amount), s.Kind})
}

func (s *SignedTransaction) signTransaction(d, n *big.Int) {
	data, err := s.marshalContentForSignature()
	if err != nil {
		return
	}
	s.Signature = encode(dissycrypto.RSASign(data, d, n).Bytes())
}

func encode(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

func decode(content string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(content)
}

func (s *SignedTransaction) Verify(accountName string) bool {
	if s.IsEVM() {
		tx, err := s.decodeEVM()
		if err != nil {
			return false
		}
		signer := types.LatestSignerForChainID(big.NewInt(s.ChainID))
		from, err := types.Sender(signer, tx)
		if err != nil {
			return false
		}
		if !strings.EqualFold(from.Hex(), accountName) {
			return false
		}
		if !strings.EqualFold(tx.Hash().Hex(), s.ID) {
			return false
		}
		if s.IsAMM() {
			if tx.To() == nil || !strings.EqualFold(tx.To().Hex(), s.To) || !strings.EqualFold(s.To, PokoinSwapRouterAddress) {
				return false
			}
			amm, ok := DecodePokoinSwapCalldata(tx.Data())
			if !ok || s.AMM == nil {
				return false
			}
			s.normalizeAMM()
			if !amm.equal(*s.AMM) {
				return false
			}
			return tx.Nonce() == s.Nonce
		}
		if tx.To() == nil {
			return s.To == "" && len(tx.Data()) > 0 && tx.Nonce() == s.Nonce
		}
		if tx.To() == nil || !strings.EqualFold(tx.To().Hex(), s.To) {
			return false
		}
		expectedValue := new(big.Int).Mul(big.NewInt(int64(s.Amount)), EVMUnit)
		if tx.Value() == nil || tx.Value().Cmp(expectedValue) != 0 {
			return false
		}
		return tx.Nonce() == s.Nonce
	}
	data, err := s.marshalContentForSignature()
	if err != nil {
		return false
	}
	signature, err := decode(s.Signature)
	if err != nil {
		return false
	}
	pk, err := PublicKeyFromAccountName(accountName)
	if err != nil {
		return false
	}
	return dissycrypto.VerifySignature(data, signature, string(pk))
}

func (s *SignedTransaction) decodeEVM() (*types.Transaction, error) {
	raw := s.Raw
	if raw == "" {
		raw = strings.TrimPrefix(s.Signature, "evm:")
	}
	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex("0x" + strings.TrimPrefix(raw, "0x"))); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (s *SignedTransaction) DecodeEVMForRPC() (*types.Transaction, error) {
	return s.decodeEVM()
}

func DecodePokoinSwapCalldata(data []byte) (AMMPayload, bool) {
	if !bytes.HasPrefix(data, []byte(PokoinSwapCalldataPrefix)) {
		return AMMPayload{}, false
	}
	var payload AMMPayload
	if err := json.Unmarshal(data[len(PokoinSwapCalldataPrefix):], &payload); err != nil {
		return AMMPayload{}, false
	}
	payload.Action = strings.ToLower(strings.TrimSpace(payload.Action))
	payload.Asset = normalizeAsset(payload.Asset)
	payload.AssetA = normalizeAsset(payload.AssetA)
	payload.AssetB = normalizeAsset(payload.AssetB)
	payload.AssetIn = normalizeAsset(payload.AssetIn)
	payload.AssetOut = normalizeAsset(payload.AssetOut)
	payload.PoolID = normalizePoolID(payload.PoolID)
	if payload.PoolID == "" && payload.AssetA != "" && payload.AssetB != "" {
		payload.PoolID = PoolID(payload.AssetA, payload.AssetB)
	}
	if payload.FeeBps == 0 {
		payload.FeeBps = AMMFeeBps
	}
	return payload, true
}

func EncodePokoinSwapCalldata(payload AMMPayload) []byte {
	payload.Action = strings.ToLower(strings.TrimSpace(payload.Action))
	payload.Asset = normalizeAsset(payload.Asset)
	payload.AssetA = normalizeAsset(payload.AssetA)
	payload.AssetB = normalizeAsset(payload.AssetB)
	payload.AssetIn = normalizeAsset(payload.AssetIn)
	payload.AssetOut = normalizeAsset(payload.AssetOut)
	payload.PoolID = normalizePoolID(payload.PoolID)
	raw, _ := json.Marshal(payload)
	return append([]byte(PokoinSwapCalldataPrefix), raw...)
}

func (a AMMPayload) equal(b AMMPayload) bool {
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	b.Action = strings.ToLower(strings.TrimSpace(b.Action))
	return a.Action == b.Action &&
		normalizePoolID(a.PoolID) == normalizePoolID(b.PoolID) &&
		normalizeAsset(a.Asset) == normalizeAsset(b.Asset) &&
		normalizeAsset(a.AssetA) == normalizeAsset(b.AssetA) &&
		normalizeAsset(a.AssetB) == normalizeAsset(b.AssetB) &&
		normalizeAsset(a.AssetIn) == normalizeAsset(b.AssetIn) &&
		normalizeAsset(a.AssetOut) == normalizeAsset(b.AssetOut) &&
		a.Amount == b.Amount &&
		a.AmountA == b.AmountA &&
		a.AmountB == b.AmountB &&
		a.AmountIn == b.AmountIn &&
		a.MinAmountOut == b.MinAmountOut
}

func normalizeAsset(asset string) string {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	switch asset {
	case "", AssetPKN:
		return asset
	case "WPKN", "W-PKN":
		return AssetWPKN
	default:
		return asset
	}
}

func normalizePoolID(poolID string) string {
	poolID = strings.TrimSpace(poolID)
	poolID = strings.ReplaceAll(poolID, "_", "-")
	poolID = strings.ReplaceAll(poolID, "/", "-")
	parts := strings.Split(poolID, "-")
	if len(parts) == 2 {
		return PoolID(parts[0], parts[1])
	}
	return strings.ToUpper(poolID)
}

func PoolID(assetA string, assetB string) string {
	assetA = normalizeAsset(assetA)
	assetB = normalizeAsset(assetB)
	if assetA == "" || assetB == "" || assetA == assetB {
		return ""
	}
	if assetA < assetB {
		return assetA + "-" + assetB
	}
	return assetB + "-" + assetA
}

func normalizeAssetAccount(account string) string {
	account = strings.TrimSpace(account)
	if strings.HasPrefix(account, "0x") || strings.HasPrefix(account, "0X") {
		return strings.ToLower(account)
	}
	return account
}

func (l *Ledger) isOperatorLocked(accountName string) bool {
	return isNativeValidatorAccount(accountName) && l.Validators[accountName]
}

func (l *Ledger) assetBalanceLocked(asset string, accountName string) int {
	asset = normalizeAsset(asset)
	accountName = normalizeAssetAccount(accountName)
	if asset == AssetPKN {
		return l.Accounts[accountName]
	}
	return l.AssetBalances[asset][accountName]
}

func (l *Ledger) creditAssetLocked(asset string, accountName string, amount int) {
	asset = normalizeAsset(asset)
	accountName = normalizeAssetAccount(accountName)
	if amount <= 0 || accountName == "" {
		return
	}
	if asset == AssetPKN {
		l.Accounts[accountName] += amount
		return
	}
	if l.AssetBalances[asset] == nil {
		l.AssetBalances[asset] = make(map[string]int)
	}
	l.AssetBalances[asset][accountName] += amount
}

func (l *Ledger) debitAssetLocked(asset string, accountName string, amount int) bool {
	asset = normalizeAsset(asset)
	accountName = normalizeAssetAccount(accountName)
	if amount <= 0 || accountName == "" {
		return false
	}
	if asset == AssetPKN {
		if l.Accounts[accountName] < amount {
			return false
		}
		l.Accounts[accountName] -= amount
		return true
	}
	if l.AssetBalances[asset] == nil || l.AssetBalances[asset][accountName] < amount {
		return false
	}
	l.AssetBalances[asset][accountName] -= amount
	return true
}

func (l *Ledger) canDebitAssetLocked(asset string, accountName string, amount int) bool {
	asset = normalizeAsset(asset)
	accountName = normalizeAssetAccount(accountName)
	if amount <= 0 || accountName == "" {
		return false
	}
	if asset == AssetPKN {
		return l.Accounts[accountName] >= amount
	}
	return l.AssetBalances[asset] != nil && l.AssetBalances[asset][accountName] >= amount
}

func (l *Ledger) debitAssetOrPKNLocked(asset string, accountName string, amount int) bool {
	return l.debitAssetLocked(asset, accountName, amount)
}

func (l *Ledger) creditAssetOrPKNLocked(asset string, accountName string, amount int) {
	l.creditAssetLocked(asset, accountName, amount)
}

func QuoteSwap(pool AMMPool, assetIn string, amountIn int) (int, bool) {
	assetIn = normalizeAsset(assetIn)
	if amountIn <= 0 || pool.ReserveA <= 0 || pool.ReserveB <= 0 {
		return 0, false
	}
	reserveIn, reserveOut := pool.reservesFor(assetIn)
	if reserveIn <= 0 || reserveOut <= 0 {
		return 0, false
	}
	feeBps := pool.FeeBps
	if feeBps <= 0 {
		feeBps = AMMFeeBps
	}
	amountInAfterFee := (amountIn * (AMMDenominatorBps - feeBps)) / AMMDenominatorBps
	if amountInAfterFee <= 0 {
		return 0, false
	}
	out := (reserveOut * amountInAfterFee) / (reserveIn + amountInAfterFee)
	if out <= 0 || out >= reserveOut {
		return 0, false
	}
	return out, true
}

func (pool AMMPool) reservesFor(assetIn string) (int, int) {
	assetIn = normalizeAsset(assetIn)
	if assetIn == pool.AssetA {
		return pool.ReserveA, pool.ReserveB
	}
	if assetIn == pool.AssetB {
		return pool.ReserveB, pool.ReserveA
	}
	return 0, 0
}

func (pool *AMMPool) applySwap(assetIn string, amountIn int, amountOut int) bool {
	assetIn = normalizeAsset(assetIn)
	if assetIn == pool.AssetA {
		if pool.ReserveB <= amountOut {
			return false
		}
		pool.ReserveA += amountIn
		pool.ReserveB -= amountOut
		return true
	}
	if assetIn == pool.AssetB {
		if pool.ReserveA <= amountOut {
			return false
		}
		pool.ReserveB += amountIn
		pool.ReserveA -= amountOut
		return true
	}
	return false
}

func NFTKey(collectionID string, tokenID string) string {
	collectionID = normalizeNFTID(collectionID)
	tokenID = normalizeNFTID(tokenID)
	if collectionID == "" || tokenID == "" {
		return ""
	}
	return collectionID + "/" + tokenID
}

func normalizeNFTID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "-")
	return strings.ToLower(value)
}

func normalizeNFTAccount(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		return strings.ToLower(value)
	}
	return value
}

func sameNFTAccount(left string, right string) bool {
	left = normalizeNFTAccount(left)
	right = normalizeNFTAccount(right)
	if strings.HasPrefix(left, "0x") || strings.HasPrefix(right, "0x") {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func StableNFTTxID(parts ...string) string {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("nft-%x", h.Sum64())
}
