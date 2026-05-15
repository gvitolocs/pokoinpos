package main

import (
	"fmt"
	"maps"
	"net"
	"peer/account"
	"peer/helpers"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestNewPeer(t *testing.T) {
	// Use free ports to avoid collisions on shared machines.
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	peer1 := NewPeer(port1)
	peer2 := NewPeer(port2)
	if peer1.listenport != port1 {
		t.Errorf("Should be %d", port1)
	}
	if peer2.listenport != port2 {
		t.Errorf("Should be %d", port2)
	}
}

func TestPeerStart(t *testing.T) {
	// Start peers and verify listeners accept connections.
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	peer1 := NewPeer(port1)
	peer1.Start()
	conn1, _ := net.Dial(helpers.PROTOCOL, fmt.Sprintf("127.0.0.1:%d", port1))
	// Defer ensures the connection is closed at test end.
	defer conn1.Close()
	if conn1.RemoteAddr().String() != fmt.Sprintf("127.0.0.1:%d", port1) {
		t.Errorf("Expects connection to be 127.0.0.1:%d", port1)
	}

	peer2 := NewPeer(port2)
	peer2.Start()
	conn2, _ := net.Dial(helpers.PROTOCOL, fmt.Sprintf("127.0.0.1:%d", port2))
	// Defer ensures the connection is closed at test end.
	defer conn2.Close()
	if conn2.RemoteAddr().String() != fmt.Sprintf("127.0.0.1:%d", port2) {
		t.Errorf("Expects connection to be 127.0.0.1:%d", port2)
	}
}

func TestPeerConnectAndSendMessages(t *testing.T) {
	// Connect two peers and verify k messages are received.
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	peer1 := NewPeer(port1)
	peer1.Start()

	peer2 := NewPeer(port2)
	peer2.Start()

	if _, err := peer1.Connect("127.0.0.1", peer2.listenport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !waitForConn(peer1, peer2.id, 2*time.Second) {
		t.Fatalf("Expected connection from %s in peer1", peer2.id)
	}
	if !waitForConn(peer2, peer1.id, 2*time.Second) {
		t.Fatalf("Expected connection from %s in peer2", peer1.id)
	}

	// Track MsgIDs to ensure we received all expected messages.
	wantIDs := map[string]bool{}
	k := 3
	// Cycle for sending k messages.
	for i := 0; i < k; i++ {
		msgID := fmt.Sprintf("msg-%d", i)
		wantIDs[msgID] = true
		if err := peer1.Send(peer2.id, &Message{
			Type:  helpers.PING_MESSAGE_TYPE,
			MsgID: msgID,
			From:  peer1.id,
		}); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	// Wait for all messages with a timeout to avoid hanging tests.
	got := 0
	timeout := time.After(2 * time.Second)
	// Cycle for waiting until all messages arrive.
	for got < k {
		select {
		case msg := <-peer2.received:
			if msg.Type == helpers.PING_MESSAGE_TYPE && wantIDs[msg.MsgID] {
				got++
				delete(wantIDs, msg.MsgID)
			}
		case <-timeout:
			t.Fatalf("Timed out waiting for messages, got %d/%d", got, k)
		}
	}
}

func TestConnectAndJoinNetwork(t *testing.T) {
	// Connect three peers and verify they all connect through flooding.
	// I.e., only attempt to connect 2->1 and 3->1 and check that they all are connected to three peers.
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	port3 := getFreePort(t)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)

	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)

	peer3 := NewPeer(port3)
	peer3.StartWithConnection("localhost", port1)
	if !waitForConn(peer2, peer3.id, 2*time.Second) { // Peer3 connects to peer1, but a flooded connection should establish to peer2 afterwards.
		t.Fatalf("Expected connection from %s in peer1", peer2.id)
	}

	conns1 := slices.Collect(maps.Keys(peer1.conns))
	slices.Sort(conns1)
	conns2 := slices.Collect(maps.Keys(peer2.conns))
	slices.Sort(conns2)
	conns3 := slices.Collect(maps.Keys(peer2.conns))
	slices.Sort(conns3)
	expected := []string{peer1.id, peer2.id, peer3.id}
	slices.Sort(expected)
	if !(reflect.DeepEqual(conns1, conns2) && reflect.DeepEqual(conns1, conns3) && reflect.DeepEqual(conns1, expected[:])) {
		t.Fatalf("Peer sets not equal")
	}
}

func TestFloodMessage(t *testing.T) {
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	port3 := getFreePort(t)
	//fmt.Println(port1, port2, port3)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)

	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)

	peer3 := NewPeer(port3)
	peer3.StartWithConnection("localhost", port1)
	if !waitForConn(peer2, peer3.id, 2*time.Second) { // Peer3 connects to peer1, but a flooded connection should establish to peer2 afterwards.
		t.Fatalf("Expected connection from %s in peer1", peer2.id)
	}

	peer1.FloodNetwork(&Message{MsgID: "flood-001", From: peer1.id, Type: "Test-flood-message"})
	timeout := time.After(2 * time.Second)
	// Wait for each peer to receive the flood message twice (one from each other peer).
	// NB: peer1 should not receive the message at all!
	// NB: It is technically possible that peer3 receives from peer1 and peer2 before sending, but the chances are minimal
	// when no order is implemented. Thus, this might need to be changed in the future.
	expected := [4]chan Message{peer2.received, peer2.received, peer3.received, peer3.received}
	for _, ch := range expected {
		select {
		case <-ch:
		case <-timeout:
			t.Errorf("Timed out waiting for message")
		}
	}
}

func TestTransaction(t *testing.T) {
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	port3 := getFreePort(t)
	//fmt.Println(port1, port2, port3)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)

	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)

	peer3 := NewPeer(port3)
	peer3.StartWithConnection("localhost", port1)
	if !waitForConn(peer2, peer3.id, 2*time.Second) { // Peer3 connects to peer1, but a flooded connection should establish to peer2 afterwards.
		t.Fatalf("Expected connection from %s in peer1", peer2.id)
	}

	//peer2.FloodTransaction(&account.Transaction{ID: "t-01", From: "User-01", To: "User-02", Amount: 42})

	// Do some testing...
}

type Msg struct {
	Content string
}

// TestLedgerConvergence checks that after flooding transactions, all peers end up with the same ledger.
func TestLedgerConvergence(t *testing.T) {
	port1 := getFreePort(t)
	port2 := getFreePort(t)
	port3 := getFreePort(t)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)
	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)
	peer3 := NewPeer(port3)
	peer3.StartWithConnection("localhost", port1)
	if !waitForConn(peer2, peer3.id, 2*time.Second) {
		t.Fatalf("Peers did not connect")
	}

	user1, _ := account.NewAccount()
	user2, _ := account.NewAccount()
	// Seed balances so valid transactions pass overdraft checks.
	for _, p := range []*Peer{peer1, peer2, peer3} {
		p.ledger.Accounts[user1.SafeEncode()] = 100
		p.ledger.Accounts[user2.SafeEncode()] = 100
	}
	// Each peer sends 2 transactions; total 6, all use accounts 1 and 2.
	peer1.FloodTransaction(account.NewSignedTransaction("lc-1", user1, user2.SafeEncode(), 10))
	peer2.FloodTransaction(account.NewSignedTransaction("lc-2", user2, user1.SafeEncode(), 3))
	peer3.FloodTransaction(account.NewSignedTransaction("lc-3", user1, user2.SafeEncode(), 1))
	peer1.FloodTransaction(account.NewSignedTransaction("lc-4", user2, user1.SafeEncode(), 2))
	peer2.FloodTransaction(account.NewSignedTransaction("lc-5", user1, user2.SafeEncode(), 5))
	peer3.FloodTransaction(account.NewSignedTransaction("lc-6", user2, user1.SafeEncode(), 1))

	totalTx := 6
	done := make(chan struct{})
	for _, p := range []*Peer{peer1, peer2, peer3} {
		go func() {
			count := 0
			for count < totalTx {
				msg := <-p.received
				if msg.Type == helpers.TRANSACTION_MESSAGE_TYPE {
					count++
				}
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for transactions")
		}
	}
	time.Sleep(200 * time.Millisecond)

	ref := peer1.ledger.CopyAccounts()
	for i, p := range []*Peer{peer2, peer3} {
		other := p.ledger.CopyAccounts()
		if !maps.Equal(ref, other) {
			t.Errorf("Ledger mismatch peer%d: got %v, want same as peer1 %v", i+2, other, ref)
		}
	}
}

// TestTransactionDeliveryCount checks that when one peer floods N transactions, the other peer receives all N (distinct MsgIDs).
func TestTransactionDeliveryCount(t *testing.T) {
	port1 := getFreePort(t)
	port2 := getFreePort(t)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)
	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)
	if !waitForConn(peer1, peer2.id, 2*time.Second) {
		t.Fatalf("Peers did not connect")
	}
	time.Sleep(100 * time.Millisecond)

	numTx := 4
	wantIDs := make(map[string]bool)
	for i := 0; i < numTx; i++ {
		wantIDs[fmt.Sprintf("tdc-%d", i)] = true
	}

	done := make(chan struct{})
	go func() {
		seen := make(map[string]bool)
		for len(seen) < numTx {
			msg := <-peer1.received
			if msg.Type == helpers.TRANSACTION_MESSAGE_TYPE && wantIDs[msg.MsgID] {
				seen[msg.MsgID] = true
			}
		}
		done <- struct{}{}
	}()

	sendUser, _ := account.NewAccount()
	receiverUser, _ := account.NewAccount()
	// Seed sender so transactions are valid under overdraft checks.
	for _, p := range []*Peer{peer1, peer2} {
		p.ledger.Accounts[sendUser.SafeEncode()] = 100
		p.ledger.Accounts[receiverUser.SafeEncode()] = 0
	}
	for i := 0; i < numTx; i++ {
		peer2.FloodTransaction(account.NewSignedTransaction(
			fmt.Sprintf("tdc-%d", i),
			sendUser,
			receiverUser.SafeEncode(),
			1,
		))
		time.Sleep(25 * time.Millisecond)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: peer1 did not receive all transactions from peer2")
	}
}

func TestMakeSignedTransaction(t *testing.T) {
	user1, _ := account.NewAccount()
	user2, _ := account.NewAccount()
	tx := account.NewSignedTransaction("test-1", user1, user2.SafeEncode(), 1)
	if !tx.Verify(user1.SafeEncode()) {
		t.Errorf("Transaction not verified for true sender.")
	}
	if tx.Verify(user2.SafeEncode()) {
		t.Errorf("Transaction verified for non-sender.")
	}
}

func TestFakeTransactionsRejected(t *testing.T) {
	port1 := getFreePort(t)
	port2 := getFreePort(t)

	peer1 := NewPeer(port1)
	peer1.StartWithConnection("localhost", -1)
	peer2 := NewPeer(port2)
	peer2.StartWithConnection("localhost", port1)
	if !waitForConn(peer1, peer2.id, 2*time.Second) {
		t.Fatalf("Peers did not connect")
	}

	user1, _ := account.NewAccount()
	user2, _ := account.NewAccount()
	// Seed balances so the one valid transaction can be accepted.
	for _, p := range []*Peer{peer1, peer2} {
		p.ledger.Accounts[user1.SafeEncode()] = 10
		p.ledger.Accounts[user2.SafeEncode()] = 0
	}
	// Each peer sends 2 transactions; total 6, all use accounts 1 and 2.
	tx1 := account.NewSignedTransaction("lc-1", user1, user2.SafeEncode(), 10)
	tx2 := account.NewSignedTransaction("lc-2", user2, user1.SafeEncode(), 10)
	// Faking signature.
	tx2.Signature = tx1.Signature
	tx3 := account.NewSignedTransaction("lc-3", user2, user1.SafeEncode(), 1)
	// Making tx3 into a message sent from user1 to user2, but signed under user2's key.
	tx3.From = user1.SafeEncode()
	tx3.To = user2.SafeEncode()
	peer1.FloodTransaction(tx2) // Should be rejected.
	peer1.FloodTransaction(tx3) // Should be rejected.
	peer1.FloodTransaction(tx3) // Replay attack. Should be rejected.
	peer1.FloodTransaction(tx1) // Should be accepted.

	totalTx := 1
	done := make(chan struct{})
	for _, p := range []*Peer{peer2} {
		go func() {
			count := 0
			for count < totalTx {
				msg := <-p.received
				if msg.Type == helpers.TRANSACTION_MESSAGE_TYPE {
					count++
				}
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 1; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for transactions")
		}
	}
	time.Sleep(200 * time.Millisecond)

	if peer1.ledger.Accounts[user1.SafeEncode()] != 0 {
		t.Errorf("Wrong account balance. Fake messages were accepted.")
	}
	if peer1.ledger.Accounts[user2.SafeEncode()] != 9 {
		t.Errorf("Wrong account balance. Fake messages were accepted.")
	}
}

func getFreePort(t *testing.T) int {
	// Ask the OS for an available port.
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	// Defer ensures the listener is closed after we read the port.
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForConn(peer *Peer, id string, timeout time.Duration) bool {
	// Poll until the connection appears or the timeout expires.
	deadline := time.Now().Add(timeout)
	// Cycle for polling the connection map.
	for time.Now().Before(deadline) {
		peer.lock.Lock()
		_, ok := peer.conns[id]
		peer.lock.Unlock()
		if ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
