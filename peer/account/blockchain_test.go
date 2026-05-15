package account

import (
	"reflect"
	"testing"
)

func TestGetCurrentTransactionsLongestBranch(t *testing.T) {
	chain := NewBlockchain()
	winnerA, _ := NewAccount()
	winnerB, _ := NewAccount()

	genesisHash := chain.Blocks[0].GetBlockHash()
	genesisHashEncoded := encode(genesisHash[:])

	// Create two competing children of genesis, then extend only one of them.
	left := NewBlock(winnerA, 1, 10, genesisHashEncoded, "left")
	right := NewBlock(winnerB, 1, 20, genesisHashEncoded, "right")
	rightHash := right.GetBlockHash()
	right2 := NewBlock(winnerB, 2, 30, encode(rightHash[:]), "right-2")

	chain.Blocks = append(chain.Blocks, *left, *right, *right2)

	got := chain.GetCurrentTransactions(0)
	want := []string{chain.Blocks[0].MetaData, "right", "right-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong longest branch metadata:\n got  %v\n want %v", got, want)
	}

	gotRollback := chain.GetCurrentTransactions(1)
	wantRollback := []string{chain.Blocks[0].MetaData, "right"}
	if !reflect.DeepEqual(gotRollback, wantRollback) {
		t.Fatalf("wrong rollback slice:\n got  %v\n want %v", gotRollback, wantRollback)
	}

	if got := chain.GetCurrentTransactions(3); len(got) != 0 {
		t.Fatalf("rollback >= length should return empty slice, got %v", got)
	}
}

func TestLedgerFromTransactionsReadsGenesisMetadata(t *testing.T) {
	chain := NewBlockchain()
	transactions := chain.GetCurrentTransactions(0)
	ledger := LedgerFromTransactions(transactions)

	if len(ledger.Accounts) != NUMBER_OF_PRIME_ACCOUNTS+NUMBER_OF_SECONDARY_ACCOUNTS {
		t.Fatalf("wrong number of genesis accounts: got %d", len(ledger.Accounts))
	}

	primeAccounts := 0
	secondaryAccounts := 0
	for _, balance := range ledger.Accounts {
		if balance == 1_000_000 {
			primeAccounts++
		}
		if balance == 0 {
			secondaryAccounts++
		}
	}

	if primeAccounts != NUMBER_OF_PRIME_ACCOUNTS {
		t.Fatalf("wrong number of prime accounts: got %d", primeAccounts)
	}
	if secondaryAccounts != NUMBER_OF_SECONDARY_ACCOUNTS {
		t.Fatalf("wrong number of secondary accounts: got %d", secondaryAccounts)
	}
}

func TestAddBlockAppliesTransactionsAndRewards(t *testing.T) {
	miner, _ := NewAccount()
	from, _ := NewAccount()
	to, _ := NewAccount()
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{miner, from, to}, 1_000_000, 1_000_000, 42)
	chain := NewBlockchainWithGenesis(genesis)

	tx := NewSignedTransaction("tx-1", from, to.SafeEncode(), 10)
	parent := chain.Blocks[0].GetBlockHash()
	block := NewCandidateBlock(miner, 1, encode(parent[:]), []SignedTransaction{*tx}, genesis.Seed)

	if ok := chain.AddBlock(block); !ok {
		t.Fatalf("expected block to be valid")
	}

	ledger := LedgerFromBlockchain(chain, 0)
	if got, want := ledger.Accounts[from.SafeEncode()], 1_000_000-10; got != want {
		t.Fatalf("wrong sender balance: got %d want %d", got, want)
	}
	if got, want := ledger.Accounts[to.SafeEncode()], 1_000_000+9; got != want {
		t.Fatalf("wrong receiver balance: got %d want %d", got, want)
	}
	if got, want := ledger.Accounts[miner.SafeEncode()], 1_000_000+11; got != want {
		t.Fatalf("wrong miner balance: got %d want %d", got, want)
	}
	if len(ledger.TxHistory) != 1 {
		t.Fatalf("expected exactly 1 tx in history, got %d", len(ledger.TxHistory))
	}
}

func TestAddBlockRejectsInvalidTransaction(t *testing.T) {
	miner, _ := NewAccount()
	from, _ := NewAccount()
	to, _ := NewAccount()
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{miner, from, to}, 1_000_000, 1_000_000, 42)
	chain := NewBlockchainWithGenesis(genesis)

	// Invalid due to overdraft.
	tx := NewSignedTransaction("tx-overspend", from, to.SafeEncode(), 2_000_000)
	parent := chain.Blocks[0].GetBlockHash()
	block := NewCandidateBlock(miner, 1, encode(parent[:]), []SignedTransaction{*tx}, genesis.Seed)

	if ok := chain.AddBlock(block); ok {
		t.Fatalf("expected block to be rejected")
	}

	ledger := LedgerFromBlockchain(chain, 0)
	if len(ledger.TxHistory) != 0 {
		t.Fatalf("expected empty tx history after rejected block")
	}
	if got := ledger.Accounts[from.SafeEncode()]; got != 1_000_000 {
		t.Fatalf("sender balance changed unexpectedly: %d", got)
	}
}
