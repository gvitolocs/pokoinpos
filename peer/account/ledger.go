package account

import (
	"encoding/base64"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type Ledger struct {
	Accounts      map[string]int
	Nonces        map[string]uint64
	Validators    map[string]bool
	NFTs          map[string]NFTToken
	AssetBalances map[string]map[string]int
	AMMPools      map[string]AMMPool
	EVMAccounts   map[string]EVMAccount
	EVMReceipts   map[string]EVMReceipt
	lock          sync.Mutex
	// Array of transactions perfomed on this ledger.
	TxHistory map[string]SignedTransaction
}

type EVMAccount struct {
	Nonce   uint64            `json:"nonce,omitempty"`
	Balance string            `json:"balance,omitempty"`
	Code    string            `json:"code,omitempty"`
	Storage map[string]string `json:"storage,omitempty"`
}

type EVMReceipt struct {
	TransactionHash string   `json:"transactionHash"`
	ContractAddress string   `json:"contractAddress,omitempty"`
	GasUsed         uint64   `json:"gasUsed"`
	Logs            []EVMLog `json:"logs,omitempty"`
	Status          int      `json:"status"`
}

type EVMLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

func MakeLedger() *Ledger {
	ledger := new(Ledger)
	ledger.Accounts = make(map[string]int)
	ledger.Nonces = make(map[string]uint64)
	ledger.Validators = make(map[string]bool)
	ledger.NFTs = make(map[string]NFTToken)
	ledger.AssetBalances = make(map[string]map[string]int)
	ledger.AMMPools = make(map[string]AMMPool)
	ledger.EVMAccounts = make(map[string]EVMAccount)
	ledger.EVMReceipts = make(map[string]EVMReceipt)
	ledger.TxHistory = make(map[string]SignedTransaction)
	return ledger
}

// CopyAccounts returns a copy of the accounts map so we can compare ledgers without holding the lock.
func (l *Ledger) CopyAccounts() map[string]int {
	return l.CopyAccountsPretty(map[string]string{})
}

func (l *Ledger) CopyAccountsPretty(simpleNames map[string]string) map[string]int {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]int, len(l.Accounts))
	for k, v := range l.Accounts {
		name, exists := simpleNames[k]
		if exists {
			out[name] = v
		} else {
			out[k] = v
		}
	}
	return out
}

func (l *Ledger) CopyValidators() map[string]bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]bool, len(l.Validators))
	for k, v := range l.Validators {
		out[k] = v
	}
	return out
}

func (l *Ledger) IsValidator(account string) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.Validators[account]
}

func (l *Ledger) MarkValidator(account string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.Validators[account] = true
}

func isNativeValidatorAccount(account string) bool {
	if account == "" {
		return false
	}
	if _, err := base64.StdEncoding.DecodeString(account); err != nil {
		return false
	}
	return true
}

func (l *Ledger) SetBalance(account string, amount int) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.Accounts[account] = amount
}

func (l *Ledger) GetBalance(account string) int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.Accounts[account]
}

func (l *Ledger) LotteryTickets(account string) int {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !isNativeValidatorAccount(account) {
		return 0
	}
	balance := l.Accounts[account]
	totalPositive := 0
	zeroValidators := 0
	known := l.Validators[account] || balance > 0
	for validator := range l.Validators {
		validatorBalance := l.Accounts[validator]
		if validatorBalance > 0 {
			totalPositive += validatorBalance
		} else {
			zeroValidators++
		}
	}
	if !known && balance == 0 {
		zeroValidators++
	}
	const scale = 1000000
	if balance > 0 {
		if totalPositive <= 0 {
			return scale
		}
		tickets := (balance * 97 * scale) / (100 * totalPositive)
		if tickets < 1 {
			return 1
		}
		return tickets
	}
	if zeroValidators <= 0 {
		return 0
	}
	tickets := (3 * scale) / (100 * zeroValidators)
	if tickets < 1 {
		return 1
	}
	return tickets
}

func (l *Ledger) GetNonce(account string) uint64 {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.Nonces[account]
}

func (l *Ledger) GetAssetBalance(asset string, account string) int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.assetBalanceLocked(normalizeAsset(asset), normalizeAssetAccount(account))
}

func (l *Ledger) CopyAssetBalances() map[string]map[string]int {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]map[string]int, len(l.AssetBalances))
	for asset, balances := range l.AssetBalances {
		out[asset] = make(map[string]int, len(balances))
		for accountName, amount := range balances {
			out[asset][accountName] = amount
		}
	}
	return out
}

func (l *Ledger) CopyAMMPools() map[string]AMMPool {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]AMMPool, len(l.AMMPools))
	for id, pool := range l.AMMPools {
		out[id] = pool
	}
	return out
}

func (l *Ledger) AMMPool(poolID string) (AMMPool, bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	pool, exists := l.AMMPools[normalizePoolID(poolID)]
	return pool, exists
}

func (l *Ledger) GetEVMCode(address string) []byte {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.evmCodeLocked(address)
}

func (l *Ledger) GetEVMStorage(address string, slot string) string {
	l.lock.Lock()
	defer l.lock.Unlock()
	account := l.EVMAccounts[strings.ToLower(address)]
	value := account.Storage[strings.ToLower(slot)]
	if value == "" {
		return "0x0000000000000000000000000000000000000000000000000000000000000000"
	}
	return value
}

func (l *Ledger) CallEVM(from string, to string, input []byte) ([]byte, bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	state := newEVMStateAdapter(l)
	snapshot := state.Snapshot()
	ret, _, err := newLedgerEVM(state, common.HexToAddress(from)).StaticCall(
		common.HexToAddress(from),
		common.HexToAddress(to),
		input,
		vm.NewGasBudget(5_000_000),
	)
	state.RevertToSnapshot(snapshot)
	if err != nil {
		return nil, false
	}
	return ret, true
}

func (l *Ledger) CopyEVMAccounts() map[string]EVMAccount {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]EVMAccount, len(l.EVMAccounts))
	for address, account := range l.EVMAccounts {
		copied := account
		if account.Storage != nil {
			copied.Storage = make(map[string]string, len(account.Storage))
			for key, value := range account.Storage {
				copied.Storage[key] = value
			}
		}
		out[address] = copied
	}
	return out
}

func (l *Ledger) EVMReceipt(txHash string) (EVMReceipt, bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	receipt, exists := l.EVMReceipts[strings.ToLower(txHash)]
	return receipt, exists
}

func (l *Ledger) CopyTransactions() map[string]SignedTransaction {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]SignedTransaction, len(l.TxHistory))
	for id, tx := range l.TxHistory {
		out[id] = tx
	}
	return out
}

func (l *Ledger) CopyNFTs() map[string]NFTToken {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]NFTToken, len(l.NFTs))
	for id, token := range l.NFTs {
		out[id] = token
	}
	return out
}

func (l *Ledger) NFT(collectionID string, tokenID string) (NFTToken, bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	token, exists := l.NFTs[NFTKey(collectionID, tokenID)]
	return token, exists
}

func (l *Ledger) NFTsByOwner(owner string) []NFTToken {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := []NFTToken{}
	for _, token := range l.NFTs {
		if sameNFTAccount(token.Owner, owner) && !token.Burned {
			out = append(out, token)
		}
	}
	return out
}

func (l *Ledger) TxCount() int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return len(l.TxHistory)
}
