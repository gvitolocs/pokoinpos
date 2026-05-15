package main

import (
	"fmt"
	"peer/account"
	"sort"
	"strings"
)

type ExplorerTx struct {
	Hash             string `json:"hash"`
	From             string `json:"from"`
	To               string `json:"to"`
	Amount           int    `json:"amount"`
	Kind             string `json:"kind,omitempty"`
	Nonce            uint64 `json:"nonce"`
	ChainID          int64  `json:"chainId,omitempty"`
	BlockHash        string `json:"blockHash"`
	BlockNumber      int    `json:"blockNumber"`
	TransactionIndex int    `json:"transactionIndex"`
	Finalized        bool   `json:"finalized"`
}

type ExplorerBlock struct {
	Number           int          `json:"number"`
	Hash             string       `json:"hash"`
	ParentHash       string       `json:"parentHash"`
	Slot             int          `json:"slot"`
	Draw             int          `json:"draw"`
	Miner            string       `json:"miner"`
	TransactionCount int          `json:"transactionCount"`
	Transactions     []ExplorerTx `json:"transactions,omitempty"`
	Finalized        bool         `json:"finalized"`
}

type ExplorerAddress struct {
	Address          string       `json:"address"`
	Balance          int          `json:"balance"`
	Nonce            uint64       `json:"nonce"`
	TransactionCount int          `json:"transactionCount"`
	Transactions     []ExplorerTx `json:"transactions,omitempty"`
}

type ExplorerIndex struct {
	BlocksByNumber  map[int]ExplorerBlock
	BlocksByHash    map[string]ExplorerBlock
	TxsByHash       map[string]ExplorerTx
	TxsByAddress    map[string][]ExplorerTx
	LatestHeight    int
	FinalizedHeight int
}

func BuildExplorerIndex(peer *Peer) ExplorerIndex {
	idx := ExplorerIndex{
		BlocksByNumber:  make(map[int]ExplorerBlock),
		BlocksByHash:    make(map[string]ExplorerBlock),
		TxsByHash:       make(map[string]ExplorerTx),
		TxsByAddress:    make(map[string][]ExplorerTx),
		LatestHeight:    0,
		FinalizedHeight: 0,
	}
	if peer == nil || peer.blockchain == nil {
		return idx
	}
	blocks := peer.blockchain.CanonicalBlocks(0)
	if len(blocks) == 0 {
		return idx
	}
	idx.LatestHeight = len(blocks) - 1
	idx.FinalizedHeight = idx.LatestHeight - peer.finalityDepth
	if idx.FinalizedHeight < 0 {
		idx.FinalizedHeight = 0
	}
	for height, block := range blocks {
		blockHash := blockHexHash(block)
		explorerBlock := ExplorerBlock{
			Number:     height,
			Hash:       blockHash,
			ParentHash: explorerParentHash(block.ParentHash),
			Slot:       block.Slot,
			Draw:       block.Draw,
			Miner:      block.VerificationKey,
			Finalized:  height <= idx.FinalizedHeight,
		}
		txs, err := account.DecodeBlockTransactions(block.MetaData)
		if err == nil {
			explorerBlock.TransactionCount = len(txs)
			explorerBlock.Transactions = make([]ExplorerTx, 0, len(txs))
			for txIndex, tx := range txs {
				explorerTx := explorerTxFromSigned(tx, blockHash, height, txIndex, explorerBlock.Finalized)
				explorerBlock.Transactions = append(explorerBlock.Transactions, explorerTx)
				idx.TxsByHash[strings.ToLower(explorerTx.Hash)] = explorerTx
				indexAddressTx(idx.TxsByAddress, explorerTx.From, explorerTx)
				indexAddressTx(idx.TxsByAddress, explorerTx.To, explorerTx)
			}
		}
		idx.BlocksByNumber[height] = explorerBlock
		idx.BlocksByHash[strings.ToLower(blockHash)] = explorerBlock
	}
	return idx
}

func explorerTxFromSigned(tx account.SignedTransaction, blockHash string, blockNumber int, txIndex int, finalized bool) ExplorerTx {
	return ExplorerTx{
		Hash:             tx.ID,
		From:             tx.From,
		To:               tx.To,
		Amount:           tx.Amount,
		Kind:             tx.Kind,
		Nonce:            tx.Nonce,
		ChainID:          tx.ChainID,
		BlockHash:        blockHash,
		BlockNumber:      blockNumber,
		TransactionIndex: txIndex,
		Finalized:        finalized,
	}
}

func signedTransactionFromExplorer(tx ExplorerTx) account.SignedTransaction {
	return account.SignedTransaction{
		ID:      tx.Hash,
		From:    tx.From,
		To:      tx.To,
		Amount:  tx.Amount,
		Kind:    tx.Kind,
		Nonce:   tx.Nonce,
		ChainID: tx.ChainID,
	}
}

func indexAddressTx(index map[string][]ExplorerTx, address string, tx ExplorerTx) {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return
	}
	index[address] = append(index[address], tx)
}

func blockHexHash(block account.Block) string {
	hash := block.GetBlockHash()
	return "0x" + fmt.Sprintf("%x", hash[:])
}

func explorerParentHash(parentHash string) string {
	if parentHash == "" {
		return "0x0000000000000000000000000000000000000000000000000000000000000000"
	}
	return evmParentHash(parentHash)
}

func (idx ExplorerIndex) BlockList() []ExplorerBlock {
	blocks := make([]ExplorerBlock, 0, len(idx.BlocksByNumber))
	for _, block := range idx.BlocksByNumber {
		blocks = append(blocks, block)
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number > blocks[j].Number
	})
	return blocks
}
