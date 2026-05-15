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
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{miner, from, to}, 1_000_000, 1_000_000_000, 42)
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
	if got, want := ledger.Accounts[to.SafeEncode()], 1_000_000+10; got != want {
		t.Fatalf("wrong receiver balance: got %d want %d", got, want)
	}
	if got, want := ledger.Accounts[miner.SafeEncode()], 1_000_000+10; got != want {
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
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{miner, from, to}, 1_000_000, 1_000_000_000, 42)
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

func TestMintTransactionCreditsReceiverWithoutDebitingMiner(t *testing.T) {
	miner, _ := NewAccount()
	to, _ := NewAccount()
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{miner}, 1_000_000, 1_000_000_000, 42)
	chain := NewBlockchainWithGenesis(genesis)

	tx := NewMintTransaction("mint-1", miner, to.SafeEncode(), 2_000_000)
	parent := chain.Blocks[0].GetBlockHash()
	block := NewCandidateBlock(miner, 1, encode(parent[:]), []SignedTransaction{*tx}, genesis.Seed)
	if ok := chain.AddBlock(block); !ok {
		t.Fatalf("expected mint block to be valid")
	}

	ledger := LedgerFromBlockchain(chain, 0)
	if got := ledger.Accounts[to.SafeEncode()]; got != 2_000_000 {
		t.Fatalf("wrong minted balance: got %d", got)
	}
	if got := ledger.Accounts[miner.SafeEncode()]; got != 1_000_000+MINER_BLOCK_REWARD {
		t.Fatalf("wrong miner balance: got %d", got)
	}
}

func TestValidatorCanMineWithoutGenesisStakeOrBalance(t *testing.T) {
	genesisMiner, _ := NewAccount()
	newValidator, _ := NewAccount()

	genesis := MakeGenesisMetaDataFromAccounts([]*Account{genesisMiner}, 1_000_000, 1_000_000_000, 42)
	chain := NewBlockchainWithGenesis(genesis)

	candidate := NewCandidateBlock(newValidator, 1, chain.BestLeafHash(), nil, genesis.Seed)
	if ok := chain.AddBlock(candidate); !ok {
		t.Fatal("expected P2P validator to mine without genesis stake or balance")
	}
}

func TestValidatorCanWithdrawAllAndRemainAuthorized(t *testing.T) {
	genesisMiner, _ := NewAccount()
	newValidator, _ := NewAccount()

	genesis := MakeGenesisMetaDataFromAccounts([]*Account{genesisMiner}, 2_000_000, 1_000_000_000, 42)
	chain := NewBlockchainWithGenesis(genesis)

	fundingTx := NewMintTransaction("fund-validator", genesisMiner, newValidator.SafeEncode(), 1_000_000)
	fundingBlock := NewCandidateBlock(genesisMiner, 1, chain.BestLeafHash(), []SignedTransaction{*fundingTx}, genesis.Seed)
	if ok := chain.AddBlock(fundingBlock); !ok {
		t.Fatal("expected funding block to be accepted")
	}

	withdrawTx := NewSignedTransaction("withdraw-all", newValidator, "0x1111111111111111111111111111111111111111", 1_000_000)
	withdrawBlock := NewCandidateBlock(newValidator, 2, chain.BestLeafHash(), []SignedTransaction{*withdrawTx}, genesis.Seed)
	if ok := chain.AddBlock(withdrawBlock); !ok {
		t.Fatal("expected full withdraw block to be accepted")
	}

	ledger := LedgerFromBlockchain(chain, 0)
	if got := ledger.GetBalance(newValidator.SafeEncode()); got != MINER_BLOCK_REWARD {
		t.Fatalf("validator should retain only block reward after full withdrawal, got %d", got)
	}
	if !ledger.IsValidator(newValidator.SafeEncode()) {
		t.Fatal("validator should remain authorized after withdrawing spendable balance")
	}
}

func TestLotteryTicketsCapsWhaleAtNinetySevenPercent(t *testing.T) {
	whale, _ := NewAccount()
	zeroValidator, _ := NewAccount()
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{whale}, 1_000_000, 1_000_000, 42)
	ledger := MakeLedger()
	ledger.InitializeFromGenesis(genesis)
	ledger.MarkValidator(zeroValidator.SafeEncode())

	whaleTickets := ledger.LotteryTickets(whale.SafeEncode())
	zeroTickets := ledger.LotteryTickets(zeroValidator.SafeEncode())
	total := whaleTickets + zeroTickets

	if got := whaleTickets * 100 / total; got != 97 {
		t.Fatalf("whale share got %d%% want 97%%", got)
	}
	if got := zeroTickets * 100 / total; got != 3 {
		t.Fatalf("zero-validator share got %d%% want 3%%", got)
	}
}

func TestLotteryTicketsSharesZeroValidatorPool(t *testing.T) {
	whale, _ := NewAccount()
	zeroA, _ := NewAccount()
	zeroB, _ := NewAccount()
	genesis := MakeGenesisMetaDataFromAccounts([]*Account{whale}, 1_000_000, 1_000_000, 42)
	ledger := MakeLedger()
	ledger.InitializeFromGenesis(genesis)
	ledger.MarkValidator(zeroA.SafeEncode())
	ledger.MarkValidator(zeroB.SafeEncode())

	if gotA, gotB := ledger.LotteryTickets(zeroA.SafeEncode()), ledger.LotteryTickets(zeroB.SafeEncode()); gotA != gotB {
		t.Fatalf("zero-validator tickets should split evenly: %d vs %d", gotA, gotB)
	}
}
