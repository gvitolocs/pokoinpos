package main

type Message struct {
	Type    string // The type is an indicator for receiver on how to respond.
	MsgID   string // ID for the message.
	From    string // Name of sender.
	Payload []byte // Any data passed with the message.
}
