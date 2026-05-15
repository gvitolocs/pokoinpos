package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

const BLOCKCHAIN_DEFAULT_HARDNESS = 10 // TODO: Change this.
const BLOCKCHAIN_SEED = 42
const NUMBER_OF_PRIME_ACCOUNTS = 10
const NUMBER_OF_SECONDARY_ACCOUNTS = 0

type GenesisMetaData struct {
	Hardness        int
	Seed            int
	InitialBalances map[string]int
}

func MakeGenesis() *Block {
	g := makeGenesisMetaData()
	data, err := marshalGenesisMetaData(g)
	if err != nil {
		data = []byte("{}")
	}
	strData := encode(data)
	return NewBlock(nil, 0, 0, "", strData)
}

func makeGenesisMetaData() *GenesisMetaData {
	g := new(GenesisMetaData)
	g.Hardness = BLOCKCHAIN_DEFAULT_HARDNESS
	g.Seed = BLOCKCHAIN_SEED
	// Initialize map before createAccounts writes into it.
	g.InitialBalances = make(map[string]int)
	// Exercise 16.2: exactly ten initial accounts with 10^6 AU each.
	createAccounts(g, NUMBER_OF_PRIME_ACCOUNTS, 1_000_000)
	createAccounts(g, NUMBER_OF_SECONDARY_ACCOUNTS, 0)
	return g
}

// MakeGenesisMetaDataFromAccounts creates a deterministic genesis from provided staking accounts.
// This is used in the 16.2 demo to ensure all peers share the exact same initial stake map.
func MakeGenesisMetaDataFromAccounts(accounts []*Account, initialBalance int, hardness int, seed int) *GenesisMetaData {
	g := new(GenesisMetaData)
	g.Hardness = hardness
	g.Seed = seed
	g.InitialBalances = make(map[string]int)
	for _, acc := range accounts {
		g.InitialBalances[acc.SafeEncode()] = initialBalance
	}
	return g
}

func createAccounts(genesis *GenesisMetaData, count int, initialBalance int) {
	for i := 0; i < count; i++ {
		account, err := NewAccount()
		if err != nil {
			continue
		}
		genesis.InitialBalances[account.SafeEncode()] = initialBalance
	}
}

// marshalGenesisMetaData serializes genesis deterministically so all peers get the same genesis hash.
func marshalGenesisMetaData(genesis *GenesisMetaData) ([]byte, error) {
	keys := make([]string, 0, len(genesis.InitialBalances))
	for key := range genesis.InitialBalances {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out bytes.Buffer
	_, err := fmt.Fprintf(&out, "{\"Hardness\":%d,\"Seed\":%d,\"InitialBalances\":{", genesis.Hardness, genesis.Seed)
	if err != nil {
		return nil, err
	}
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		_, err = fmt.Fprintf(&out, "%s:%d", keyJSON, genesis.InitialBalances[key])
		if err != nil {
			return nil, err
		}
	}
	out.WriteString("}}")
	return out.Bytes(), nil
}

func (l *Ledger) InitializeFromGenesis(genesis *GenesisMetaData) {
	l.lock.Lock()
	defer l.lock.Unlock()
	for account, amount := range genesis.InitialBalances {
		l.Accounts[account] = amount
	}
}
