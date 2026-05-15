package account

import (
	"encoding/base64"
	"sync"
)

type Ledger struct {
	Accounts   map[string]int
	Nonces     map[string]uint64
	Validators map[string]bool
	lock       sync.Mutex
	// Array of transactions perfomed on this ledger.
	TxHistory map[string]SignedTransaction
}

func MakeLedger() *Ledger {
	ledger := new(Ledger)
	ledger.Accounts = make(map[string]int)
	ledger.Nonces = make(map[string]uint64)
	ledger.Validators = make(map[string]bool)
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
	if len(account) < 80 {
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

func (l *Ledger) CopyTransactions() map[string]SignedTransaction {
	l.lock.Lock()
	defer l.lock.Unlock()
	out := make(map[string]SignedTransaction, len(l.TxHistory))
	for id, tx := range l.TxHistory {
		out[id] = tx
	}
	return out
}

func (l *Ledger) TxCount() int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return len(l.TxHistory)
}
