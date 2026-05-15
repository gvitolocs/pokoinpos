---
title: "Hand-in 10 - Exercise 16.2 Report"
author: "Hand-in 10"
date: "\\today"
documentclass: article
geometry:
  - margin=2.5cm
  - a4paper
fontsize: 11pt
numbersections: true
---

# Goal

This report explains how Exercise 16.2 is implemented in a simple way, using the current project style.

The requested points are covered:

- total order with a tree-based blockchain and longest chain rule,
- static Proof-of-Stake block production by slots,
- block rewards (+10 per block, +1 per transaction in the block),
- block validity checks,
- throughput measurement,
- rollback observation.

# Design

## Genesis and static stake

Genesis uses:

- 10 initial accounts,
- 1,000,000 AU each,
- fixed `Hardness` and `Seed`.

All peers use the same genesis metadata to start from the same state.

## Slot-based PoS

Each slot has length 1 second.

For each peer and each slot:

1. a deterministic draw is computed from `(Seed, Slot, VerificationKey)`,
2. winning is checked with static tickets (`InitialBalances`),
3. if the peer wins, it creates one candidate block.

In the handin demo, we keep one producer per slot (first winner found) to keep behavior simple and stable.

## Total order by blockchain tree

Blocks form a tree (`ParentHash`).

To get the ordered ledger state, peers:

1. compute depths in the tree,
2. pick the longest branch leaf,
3. rebuild ledger from genesis to that leaf.

So total order is the transaction order of the current best branch.

# Block Validation Rules

When a block is received, the peer accepts it only if all checks pass:

1. parent exists in the known tree,
2. block signature is valid,
3. lottery draw is valid for winner/slot/seed/hardness,
4. transaction list format is valid and block size limit is respected,
5. all transactions are valid in sequence on parent state:
   - positive amount,
   - valid signature,
   - no overdraft.

If one transaction is invalid, the whole block is rejected.

# Rewards

After transaction validation, miner reward is applied:

- `+10 AU` per accepted block,
- `+1 AU` per transaction in that block.

Transaction fee logic remains:

- sender pays full amount,
- receiver gets `amount - 1`,
- the `1 AU` difference is the fee.

# Throughput Measurement

Throughput is measured in the demo as:

`valid_tx_on_best_chain / elapsed_seconds`

Example run output:

- Mined blocks: 4 in about 30.1 s
- Throughput: about 0.199 valid tx/s

The value changes per run because winners are probabilistic.

# Rollback Observation

Rollback is observed by recomputing ledger from blockchain with:

- `rollback = 0` (full best branch),
- `rollback = 1` (ignore latest block).

The demo prints both transaction counts.
When latest block has transactions, `rollback=1` gives fewer transactions.
When latest block has no transactions, counts can be equal.

# How to Run

From `peer/`:

```bash
go test ./...
go run .
```

The demo prints:

- final ledgers for all peers,
- mined block count,
- throughput,
- rollback comparison.
