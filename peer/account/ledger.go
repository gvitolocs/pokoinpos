package account

import (
	"sync"
)

type Ledger struct {
	Accounts map[string]int
	lock     sync.Mutex
	// Array of transactions perfomed on this ledger.
	TxHistory map[string]SignedTransaction
}

func MakeLedger() *Ledger {
	ledger := new(Ledger)
	ledger.Accounts = make(map[string]int)
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

func (l *Ledger) TxCount() int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return len(l.TxHistory)
}
