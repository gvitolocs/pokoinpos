// handin.go — Demo for Exercise 4.6 (Hand-in 2): simple P2P ledger.
package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"peer/account"
	"sync"
	"time"
)

// main is the entry point for the hand-in demo (traccia: "program handin.go").
// Run: go run .
func main() {
	cfg, err := LoadNodeConfigFromEnv()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if cfg.RunMode == "node" {
		runNode(cfg)
		return
	}
	runHandin()
}

// runHandin starts peers, floods transactions, and checks ledger convergence.
func runHandin() {
	fmt.Println("Starting Exercise 16.2 demo.")
	const numPeers = 10
	const slotLength = 1 * time.Second
	const numSlots = 30

	// Create the 10 staking accounts requested by 16.2.
	stakeAccounts := make([]*account.Account, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		acc, _ := account.NewAccount()
		stakeAccounts = append(stakeAccounts, acc)
	}

	// Genesis: static stake map (10 accounts, 10^6 PKN each).
	// Hardness is tuned so we get a low winner probability per slot.
	genesis := account.MakeGenesisMetaDataFromAccounts(stakeAccounts, 1_000_000, 10_000, 42)

	// Use three existing stake accounts for transactions.
	acc1 := stakeAccounts[0]
	acc2 := stakeAccounts[1]
	acc3 := stakeAccounts[2]

	accountNames := map[string]string{
		acc1.SafeEncode(): "A",
		acc2.SafeEncode(): "B",
		acc3.SafeEncode(): "C",
	}

	// Make transactions to send.
	validTransactions := []*account.SignedTransaction{
		account.NewSignedTransaction("val-01", acc1, acc2.SafeEncode(), 3),
		account.NewSignedTransaction("val-02", acc3, acc2.SafeEncode(), 5),
		account.NewSignedTransaction("val-03", acc1, acc2.SafeEncode(), 7),
		account.NewSignedTransaction("val-04", acc2, acc1.SafeEncode(), 11),
		account.NewSignedTransaction("val-05", acc3, acc1.SafeEncode(), 13),
		account.NewSignedTransaction("val-06", acc1, acc3.SafeEncode(), 17),
	}

	inv2 := account.NewSignedTransaction("inv-02", acc3, acc2.SafeEncode(), 5)
	inv2.Signature = validTransactions[2].Signature
	inv3 := account.NewSignedTransaction("inv-03", acc1, acc2.SafeEncode(), 7)
	inv3.From = acc3.SafeEncode()
	inv4 := account.NewSignedTransaction("inv-04", acc1, acc2.SafeEncode(), 0)  // Zero amount.
	inv5 := account.NewSignedTransaction("inv-05", acc1, acc2.SafeEncode(), -3) // Negative amount.
	inv6 := account.NewSignedTransaction("inv-06", acc1, acc2.SafeEncode(), 100_000_000)
	invalidTransactions := []*account.SignedTransaction{
		validTransactions[0], // Replay.
		inv2,                 // Fake signature.
		inv3,                 // Changing sender.
		inv4,                 // Zero amount.
		inv5,                 // Negative amount.
		inv6,                 // Overdraft.
	}

	// Connect peers.
	basePort := 43000
	peers := make([]*Peer, numPeers)

	// 1) Start first peer with no existing network (it starts its own).
	peers[0] = NewPeer(basePort)
	peers[0].ConfigurePoS(genesis, stakeAccounts[0])
	peers[0].StartWithConnection("localhost", -1)

	// 2) Start the rest and point them to the first peer so they join the same network.
	for i := 1; i < numPeers; i++ {
		peers[i] = NewPeer(basePort + i)
		peers[i].ConfigurePoS(genesis, stakeAccounts[i])
		peers[i].StartWithConnection("localhost", basePort)
	}

	// Give the network time to form (Join messages and connections).
	time.Sleep(5 * time.Second)

	// 3) Broadcast transactions (valid + invalid) to mempools.
	var wgSend sync.WaitGroup
	for i, tx := range append(invalidTransactions, validTransactions...) {
		sender := peers[i%numPeers]
		txToSend := tx
		wgSend.Go(func() { sender.FloodPoSTransaction(txToSend) })
	}
	wgSend.Wait()
	fmt.Printf("Submitted %d valid, %d invalid transactions.\n", len(validTransactions), len(invalidTransactions))
	time.Sleep(2 * time.Second)

	// 4) Run slot-based block production (1s slots).
	start := time.Now()
	minedBlocks := 0
	for slot := 1; slot <= numSlots; slot++ {
		for i := 0; i < numPeers; i++ {
			mineDone := make(chan *account.Block, 1)
			go func(peer *Peer, s int) {
				mineDone <- peer.MineOneSlot(s)
			}(peers[i], slot)

			var mined *account.Block
			select {
			case mined = <-mineDone:
			case <-time.After(6 * time.Second):
				fmt.Printf("Timeout in MineOneSlot at slot=%d peer=%s\n", slot, peers[i].id)
				return
			}
			if mined != nil {
				minedBlocks++
				// Keep one producer per slot in this simple demo.
				// This avoids excessive forks and makes convergence easier to inspect.
				break
			}
		}
		time.Sleep(slotLength)
	}
	elapsed := time.Since(start)

	// Give handlers time to process the latest block floods.
	time.Sleep(1 * time.Second)

	// 5) Check that all peers converged to the same longest-chain ledger.
	ref := peers[0].ledger.CopyAccountsPretty(accountNames)
	fmt.Println("Final ledgers:")
	fmt.Printf("  Peer %s: %v\n", peers[0].id, ref)
	for i := 1; i < numPeers; i++ {
		other := peers[i].ledger.CopyAccountsPretty(accountNames)
		fmt.Printf("  Peer %s: %v\n", peers[i].id, other)
		if !maps.Equal(ref, other) {
			fmt.Printf("Ledger mismatch: peer %s differs from peer %s\n", peers[i].id, peers[0].id)
			return
		}
	}

	// 6) Check that only valid transactions were included.
	for _, p := range peers {
		if len(p.ledger.TxHistory) != len(validTransactions) {
			fmt.Printf("Peer %s has an incorrect amount of transactions (%d)\n", p.id, len(p.ledger.TxHistory))
			return
		}
	}

	// 7) Throughput metric requested by 16.2.
	tps := float64(len(peers[0].ledger.TxHistory)) / elapsed.Seconds()
	fmt.Printf("Mined blocks: %d in %s\n", minedBlocks, elapsed.Round(time.Millisecond))
	fmt.Printf("Throughput (valid tx/s on best chain): %.3f\n", tps)

	// 8) Rollback observation: compare best-chain ledger with rollback=1.
	noRollback := account.LedgerFromBlockchain(peers[0].blockchain, 0)
	withRollback := account.LedgerFromBlockchain(peers[0].blockchain, 1)
	fmt.Printf("Tx count no rollback: %d | rollback=1: %d\n", len(noRollback.TxHistory), len(withRollback.TxHistory))

	fmt.Println("Demo complete: PoS ordering, rewards, throughput and rollback check done.")
}

func runNode(cfg NodeConfig) {
	peer := NewPeer(cfg.ListenPort)
	peer.SetFinalityDepth(cfg.FinalityDepth)
	slot := 1
	lastPayoutUnix := int64(0)
	if restored, err := loadNodeState(cfg.StateDir, cfg.ListenPort); err == nil {
		miner, minerErr := account.AccountFromKeyMaterial(restored.Miner)
		if minerErr != nil {
			fmt.Println(minerErr)
			os.Exit(1)
		}
		peer.ConfigurePoS(&restored.Genesis, miner)
		peer.blockchain.ReplaceBlocks(restored.Blocks)
		peer.ledger = account.LedgerFromBlockchain(peer.blockchain, cfg.FinalityDepth)
		if restored.LastSlot > 0 {
			slot = restored.LastSlot + 1
		}
		lastPayoutUnix = restored.LastPayoutUnix
		logEvent("node_state_restored", map[string]any{
			"stateDir": cfg.StateDir,
			"lastSlot": restored.LastSlot,
			"blocks":   len(restored.Blocks),
		})
	} else {
		miner, minerErr := account.NewAccount()
		if minerErr != nil {
			fmt.Println(minerErr)
			os.Exit(1)
		}
		genesis := account.MakeGenesisMetaDataFromAccounts(
			[]*account.Account{miner},
			cfg.InitialBalance,
			cfg.GenesisHardness,
			cfg.GenesisSeed,
		)
		for address, amount := range cfg.EVMGenesisAlloc {
			genesis.InitialBalances[address] = amount
		}
		peer.ConfigurePoS(genesis, miner)
	}
	peer.SetAdvertiseHost(cfg.AdvertiseHost)
	bootstrapRegistry := NewBootstrapRegistry(cfg.BootstrapManifestURL, cfg.BootstrapRefreshHours, cfg.BootstrapPeers)
	bootstrapPeers := mergeBootstrapPeers(bootstrapRegistry.Peers(context.Background()), cfg.BootstrapPeers)
	bootstrapRegistry.Start(context.Background())
	startedFromBootstrap := false
	if err := peer.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	go func() {
		if err := StartOpsServer(cfg.OpsAddr, peer, cfg.AdminToken, cfg.EVMChainID, cfg.EVMNetworkID, bootstrapRegistry); err != nil {
			logEvent("ops_server_stopped", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
	}()

	for _, bootstrap := range bootstrapPeers {
		peers, err := peer.ConnectAddress(bootstrap)
		if err != nil {
			continue
		}
		peer.joinKnownPeers(peers)
		peer.floodJoin()
		startedFromBootstrap = true
		logEvent("bootstrap_join_succeeded", map[string]any{
			"host": bootstrap.Host,
			"port": bootstrap.Port,
		})
		break
	}
	if !startedFromBootstrap && cfg.JoinPort > 0 {
		peers, err := peer.Connect(cfg.JoinHost, cfg.JoinPort)
		if err == nil {
			peer.joinNetwork(cfg.JoinHost, peers)
			peer.joinKnownPeers(peer.knownPeerAddresses())
			peer.floodJoin()
			startedFromBootstrap = true
		}
	}

	go func() {
		if cfg.JoinPort < 0 && len(cfg.BootstrapPeers) == 0 && len(bootstrapPeers) == 0 {
			return
		}
		reconnectTicker := time.NewTicker(time.Duration(cfg.ReconnectIntervalSeconds) * time.Second)
		defer reconnectTicker.Stop()
		for range reconnectTicker.C {
			if peer.PeerCount() > 0 {
				continue
			}
			reconnected := false
			for _, address := range peer.knownPeerAddresses() {
				peers, err := peer.ConnectAddress(address)
				if err != nil {
					continue
				}
				peer.joinKnownPeers(peers)
				peer.floodJoin()
				reconnected = true
				logEvent("known_peer_reconnect_succeeded", map[string]any{
					"peerId": address.ID,
					"host":   address.Host,
					"port":   address.Port,
				})
				break
			}
			if reconnected {
				continue
			}
			manifestPeers := bootstrapRegistry.CachedPeers()
			for _, bootstrap := range mergeBootstrapPeers(manifestPeers, cfg.BootstrapPeers) {
				peers, err := peer.ConnectAddress(bootstrap)
				if err != nil {
					continue
				}
				peer.joinKnownPeers(peers)
				peer.floodJoin()
				reconnected = true
				logEvent("bootstrap_reconnect_succeeded", map[string]any{
					"host": bootstrap.Host,
					"port": bootstrap.Port,
				})
				break
			}
			if reconnected {
				continue
			}
			if cfg.JoinPort < 0 {
				continue
			}
			peers, err := peer.Connect(cfg.JoinHost, cfg.JoinPort)
			if err != nil {
				continue
			}
			peer.joinNetwork(cfg.JoinHost, peers)
			peer.joinKnownPeers(peer.knownPeerAddresses())
			peer.floodJoin()
			logEvent("seed_reconnect_succeeded", map[string]any{
				"joinHost": cfg.JoinHost,
				"joinPort": cfg.JoinPort,
			})
		}
	}()

	logEvent("node_started", map[string]any{
		"listenPort":               cfg.ListenPort,
		"advertiseHost":            cfg.AdvertiseHost,
		"opsAddr":                  cfg.OpsAddr,
		"slotSeconds":              cfg.SlotSeconds,
		"idleSlotInterval":         cfg.IdleSlotInterval,
		"baseMineAttempts":         cfg.BaseMineAttemptsPerTick,
		"maxMineAttempts":          cfg.MaxMineAttemptsPerTick,
		"mineAttemptsPerPendingTx": cfg.MineAttemptsPerPendingTx,
		"genesisHardness":          cfg.GenesisHardness,
		"genesisSeed":              cfg.GenesisSeed,
		"finalityDepth":            cfg.FinalityDepth,
		"evmChainID":               cfg.EVMChainID,
		"evmNetworkID":             cfg.EVMNetworkID,
		"evmAllocCount":            len(cfg.EVMGenesisAlloc),
		"rewardPayoutConfigured":   cfg.RewardPayoutAddress != "",
		"joinHost":                 cfg.JoinHost,
		"joinPort":                 cfg.JoinPort,
		"bootstrapManifestURL":     cfg.BootstrapManifestURL,
		"bootstrapPeerCount":       len(bootstrapPeers),
		"stateDir":                 cfg.StateDir,
	})

	stateTicker := time.NewTicker(time.Duration(cfg.StateSaveIntervalSeconds) * time.Second)
	defer stateTicker.Stop()
	ticker := time.NewTicker(time.Duration(cfg.SlotSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if tx, paid := maybeQueueMonthlyPayout(peer, cfg, &lastPayoutUnix); paid {
				logEvent("validator_monthly_payout_queued", map[string]any{
					"to":     tx.To,
					"amount": tx.Amount,
					"hash":   tx.ID,
				})
			}
			mempoolDepth := peer.MempoolSize()
			if !shouldMineSlot(slot, mempoolDepth, cfg) {
				slot++
				continue
			}
			attempts := mineAttemptsForNetwork(mempoolDepth, peer.PeerCount()+1, cfg)
			for attempt := 0; attempt < attempts; attempt++ {
				mined := peer.MineOneSlot(slot)
				slot++
				if mined != nil {
					break
				}
			}
			if slot == int(^uint(0)>>1) {
				// Slot rollover guard for long-running process.
				slot = 1
			}
		case <-stateTicker.C:
			if err := saveNodeState(cfg.StateDir, cfg.ListenPort, peer, slot, lastPayoutUnix); err != nil {
				logEvent("node_state_save_failed", map[string]any{"error": err.Error()})
			}
		}
	}
}

const monthlyPayoutIntervalSeconds = int64(30 * 24 * 60 * 60)

func maybeQueueMonthlyPayout(peer *Peer, cfg NodeConfig, lastPayoutUnix *int64) (*account.SignedTransaction, bool) {
	if cfg.RewardPayoutAddress == "" || peer == nil || peer.miner == nil {
		return nil, false
	}
	now := time.Now().Unix()
	if *lastPayoutUnix != 0 && now-*lastPayoutUnix < monthlyPayoutIntervalSeconds {
		return nil, false
	}
	balance := peer.ValidatorStake()
	if balance <= 0 {
		return nil, false
	}
	tx, ok := peer.FloodValidatorWithdrawTransaction(
		fmt.Sprintf("monthly-payout-%d-%s-%d", now, cfg.RewardPayoutAddress, balance),
		cfg.RewardPayoutAddress,
		balance,
	)
	if !ok {
		return nil, false
	}
	*lastPayoutUnix = now
	return tx, true
}

func shouldMineSlot(slot int, mempoolDepth int, cfg NodeConfig) bool {
	if mempoolDepth > 0 {
		return true
	}
	if cfg.IdleSlotInterval == 0 {
		return true
	}
	return slot == 1 || slot%cfg.IdleSlotInterval == 0
}

func mineAttemptsForNetwork(mempoolDepth int, availableNodes int, cfg NodeConfig) int {
	if availableNodes < 1 {
		availableNodes = 1
	}
	networkAttempts := cfg.BaseMineAttemptsPerTick + mempoolDepth*cfg.MineAttemptsPerPendingTx
	attempts := (networkAttempts + availableNodes - 1) / availableNodes
	if attempts > cfg.MaxMineAttemptsPerTick {
		return cfg.MaxMineAttemptsPerTick
	}
	if attempts < cfg.BaseMineAttemptsPerTick {
		return cfg.BaseMineAttemptsPerTick
	}
	return attempts
}
