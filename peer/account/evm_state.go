package account

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/bal"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

type evmStateAdapter struct {
	ledger    *Ledger
	snapshots []map[string]EVMAccount
	refund    uint64
	logs      []*types.Log
	access    *state.AccessEvents
}

func newEVMStateAdapter(ledger *Ledger) *evmStateAdapter {
	return &evmStateAdapter{ledger: ledger, access: state.NewAccessEvents()}
}

func newLedgerEVM(adapter *evmStateAdapter, origin common.Address) *vm.EVM {
	blockCtx := vm.BlockContext{
		CanTransfer: func(db vm.StateDB, addr common.Address, amount *uint256.Int) bool {
			return db.GetBalance(addr).Cmp(amount) >= 0
		},
		Transfer: func(db vm.StateDB, from common.Address, to common.Address, amount *uint256.Int, _ *params.Rules) {
			db.SubBalance(from, amount, tracing.BalanceChangeTransfer)
			db.AddBalance(to, amount, tracing.BalanceChangeTransfer)
		},
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		GasLimit:    30_000_000,
		BlockNumber: big.NewInt(0),
		Time:        0,
		Difficulty:  big.NewInt(0),
		BaseFee:     big.NewInt(0),
		BlobBaseFee: big.NewInt(0),
	}
	evm := vm.NewEVM(blockCtx, adapter, params.AllEthashProtocolChanges, vm.Config{NoBaseFee: true})
	evm.SetTxContext(vm.TxContext{
		Origin:       origin,
		GasPrice:     uint256.NewInt(0),
		AccessEvents: state.NewAccessEvents(),
	})
	return evm
}

func (s *evmStateAdapter) account(addr common.Address) EVMAccount {
	key := evmAddressKey(addr)
	account := s.ledger.EVMAccounts[key]
	if account.Storage == nil {
		account.Storage = map[string]string{}
	}
	return account
}

func (s *evmStateAdapter) putAccount(addr common.Address, account EVMAccount) {
	key := evmAddressKey(addr)
	if account.Storage == nil {
		account.Storage = map[string]string{}
	}
	s.ledger.EVMAccounts[key] = account
}

func (s *evmStateAdapter) CreateAccount(addr common.Address) {
	if !s.Exist(addr) {
		s.putAccount(addr, EVMAccount{Storage: map[string]string{}})
	}
}

func (s *evmStateAdapter) CreateContract(addr common.Address) {
	s.CreateAccount(addr)
}

func (s *evmStateAdapter) SubBalance(addr common.Address, amount *uint256.Int, _ tracing.BalanceChangeReason) uint256.Int {
	account := s.account(addr)
	balance := evmBalanceFromString(account.Balance)
	if balance.Cmp(amount) < 0 {
		balance = uint256.NewInt(0)
	} else {
		balance.Sub(balance, amount)
	}
	account.Balance = balance.ToBig().String()
	s.putAccount(addr, account)
	return *balance
}

func (s *evmStateAdapter) AddBalance(addr common.Address, amount *uint256.Int, _ tracing.BalanceChangeReason) uint256.Int {
	account := s.account(addr)
	balance := evmBalanceFromString(account.Balance)
	balance.Add(balance, amount)
	account.Balance = balance.ToBig().String()
	s.putAccount(addr, account)
	return *balance
}

func (s *evmStateAdapter) GetBalance(addr common.Address) *uint256.Int {
	return evmBalanceFromString(s.account(addr).Balance)
}

func (s *evmStateAdapter) GetNonce(addr common.Address) uint64 {
	return s.account(addr).Nonce
}

func (s *evmStateAdapter) SetNonce(addr common.Address, nonce uint64, _ tracing.NonceChangeReason) {
	account := s.account(addr)
	account.Nonce = nonce
	s.putAccount(addr, account)
}

func (s *evmStateAdapter) GetCodeHash(addr common.Address) common.Hash {
	code := s.GetCode(addr)
	if len(code) == 0 {
		return common.Hash{}
	}
	return crypto.Keccak256Hash(code)
}

func (s *evmStateAdapter) GetCode(addr common.Address) []byte {
	return s.ledger.evmCodeLocked(addr.Hex())
}

func (s *evmStateAdapter) SetCode(addr common.Address, code []byte, _ tracing.CodeChangeReason) []byte {
	account := s.account(addr)
	previous := s.GetCode(addr)
	account.Code = "0x" + hex.EncodeToString(code)
	s.putAccount(addr, account)
	return previous
}

func (s *evmStateAdapter) GetCodeSize(addr common.Address) int {
	return len(s.GetCode(addr))
}

func (s *evmStateAdapter) AddRefund(gas uint64) { s.refund += gas }
func (s *evmStateAdapter) SubRefund(gas uint64) {
	if gas > s.refund {
		s.refund = 0
		return
	}
	s.refund -= gas
}
func (s *evmStateAdapter) GetRefund() uint64 { return s.refund }

func (s *evmStateAdapter) GetStateAndCommittedState(addr common.Address, key common.Hash) (common.Hash, common.Hash) {
	value := s.GetState(addr, key)
	return value, value
}

func (s *evmStateAdapter) GetState(addr common.Address, key common.Hash) common.Hash {
	value := s.account(addr).Storage[strings.ToLower(key.Hex())]
	return common.HexToHash(value)
}

func (s *evmStateAdapter) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
	account := s.account(addr)
	old := common.HexToHash(account.Storage[strings.ToLower(key.Hex())])
	account.Storage[strings.ToLower(key.Hex())] = strings.ToLower(value.Hex())
	s.putAccount(addr, account)
	return old
}

func (s *evmStateAdapter) GetTransientState(common.Address, common.Hash) common.Hash {
	return common.Hash{}
}
func (s *evmStateAdapter) SetTransientState(common.Address, common.Hash, common.Hash) {}
func (s *evmStateAdapter) SelfDestruct(addr common.Address) {
	account := s.account(addr)
	account.Code = ""
	account.Storage = map[string]string{}
	s.putAccount(addr, account)
}
func (s *evmStateAdapter) HasSelfDestructed(common.Address) bool { return false }

func (s *evmStateAdapter) Exist(addr common.Address) bool {
	_, exists := s.ledger.EVMAccounts[evmAddressKey(addr)]
	return exists
}

func (s *evmStateAdapter) Touch(addr common.Address) { s.CreateAccount(addr) }

func (s *evmStateAdapter) IsNewContract(addr common.Address) bool {
	return s.Exist(addr) && len(s.GetCode(addr)) > 0
}

func (s *evmStateAdapter) Empty(addr common.Address) bool {
	account := s.account(addr)
	return account.Nonce == 0 && account.Balance == "" && account.Code == "" && len(account.Storage) == 0
}

func (s *evmStateAdapter) AddressInAccessList(common.Address) bool { return true }
func (s *evmStateAdapter) SlotInAccessList(common.Address, common.Hash) (bool, bool) {
	return true, true
}
func (s *evmStateAdapter) AddAddressToAccessList(common.Address)           {}
func (s *evmStateAdapter) AddSlotToAccessList(common.Address, common.Hash) {}

func (s *evmStateAdapter) Prepare(params.Rules, common.Address, common.Address, *common.Address, []common.Address, types.AccessList) {
}

func (s *evmStateAdapter) RevertToSnapshot(id int) {
	if id < 0 || id >= len(s.snapshots) {
		return
	}
	s.ledger.EVMAccounts = copyEVMAccountMap(s.snapshots[id])
	s.snapshots = s.snapshots[:id]
}

func (s *evmStateAdapter) Snapshot() int {
	s.snapshots = append(s.snapshots, copyEVMAccountMap(s.ledger.EVMAccounts))
	return len(s.snapshots) - 1
}

func (s *evmStateAdapter) AddLog(log *types.Log) {
	if log != nil {
		log.Index = uint(len(s.logs))
	}
	s.logs = append(s.logs, log)
}

func (s *evmStateAdapter) LogsForBurnAccounts() []*types.Log { return nil }
func (s *evmStateAdapter) AddPreimage(common.Hash, []byte)   {}
func (s *evmStateAdapter) Witness() *stateless.Witness       { return nil }
func (s *evmStateAdapter) AccessEvents() *state.AccessEvents { return s.access }
func (s *evmStateAdapter) Finalise(bool) *bal.StateAccessList {
	return nil
}

func (l *Ledger) evmCodeLocked(address string) []byte {
	account := l.EVMAccounts[strings.ToLower(address)]
	codeHex := strings.TrimPrefix(account.Code, "0x")
	if codeHex == "" {
		return nil
	}
	code, err := hex.DecodeString(codeHex)
	if err != nil {
		return nil
	}
	return code
}

func copyEVMAccountMap(in map[string]EVMAccount) map[string]EVMAccount {
	out := make(map[string]EVMAccount, len(in))
	for address, account := range in {
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

func evmLogsFromTypes(logs []*types.Log) []EVMLog {
	out := make([]EVMLog, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		topics := make([]string, 0, len(log.Topics))
		for _, topic := range log.Topics {
			topics = append(topics, strings.ToLower(topic.Hex()))
		}
		out = append(out, EVMLog{
			Address: strings.ToLower(log.Address.Hex()),
			Topics:  topics,
			Data:    "0x" + hex.EncodeToString(log.Data),
		})
	}
	return out
}

func evmAddressKey(addr common.Address) string {
	return strings.ToLower(addr.Hex())
}

func evmBalanceFromString(value string) *uint256.Int {
	if value == "" {
		return uint256.NewInt(0)
	}
	bigValue, ok := new(big.Int).SetString(value, 10)
	if !ok || bigValue.Sign() < 0 {
		return uint256.NewInt(0)
	}
	out, overflow := uint256.FromBig(bigValue)
	if overflow {
		return uint256.NewInt(0)
	}
	return out
}
