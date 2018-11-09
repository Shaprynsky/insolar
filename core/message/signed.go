/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package message

import (
	"context"
	"crypto/ecdsa"

	"github.com/pkg/errors"

	"github.com/insolar/insolar/core"
	ecdsa2 "github.com/insolar/insolar/cryptohelpers/ecdsa"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/instrumentation/instracer"
	"github.com/insolar/insolar/log"
)

// SignedMessage is a message signed by senders private key.
type SignedMessage struct {
	Sender        core.RecordRef
	Msg           core.Message
	Signature     []byte
	LogTraceID    string
	TraceSpanData []byte
	PulseNumber   core.PulseNumber
}

// Pulse returns pulse when message was sent.
func (sm *SignedMessage) Pulse() core.PulseNumber {
	return sm.PulseNumber
}

func (sm *SignedMessage) Message() core.Message {
	return sm.Msg
}

// Context returns initialized context with propagated data with ctx as parent.
func (sm *SignedMessage) Context(ctx context.Context) context.Context {
	ctx = inslogger.ContextWithTrace(ctx, sm.LogTraceID)
	parentspan := instracer.MustDeserialize(sm.TraceSpanData)
	return instracer.WithParentSpan(ctx, parentspan)
}

// NewSignedMessage creates and return a signed message.
func NewSignedMessage(
	ctx context.Context,
	msg core.Message,
	sender core.RecordRef,
	key *ecdsa.PrivateKey,
	pulse core.PulseNumber,
) (*SignedMessage, error) {
	if key == nil {
		return nil, errors.New("failed to sign a message: private key == nil")
	}
	if msg == nil {
		return nil, errors.New("failed to sign a nil message")
	}
	sign, err := signMessage(msg, key)
	if err != nil {
		return nil, err
	}
	return &SignedMessage{
		Sender:        sender,
		Msg:           msg,
		Signature:     sign,
		LogTraceID:    inslogger.TraceID(ctx),
		TraceSpanData: instracer.MustSerialize(ctx),
		PulseNumber:   pulse,
	}, nil
}

// SignMessage tries to sign a core.Message.
func signMessage(msg core.Message, key *ecdsa.PrivateKey) ([]byte, error) {
	serialized := ToBytes(msg)
	sign, err := ecdsa2.Sign(serialized, key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign a message")
	}
	return sign, nil
}

// IsValid checks if a sign is correct.
func (sm *SignedMessage) IsValid(key *ecdsa.PublicKey) bool {
	exportedKey, err := ecdsa2.ExportPublicKey(key)
	if err != nil {
		log.Error("failed to export a public key")
		return false
	}
	verified, err := ecdsa2.Verify(ToBytes(sm.Msg), sm.Signature, exportedKey)
	if err != nil {
		log.Error(err, "failed to verify a message")
		return false
	}
	return verified
}

// Type returns message type.
func (sm *SignedMessage) Type() core.MessageType {
	return sm.Msg.Type()
}

// Target returns target for this message. If nil, Message will be sent for all actors for the role returned by
// Role method.
func (sm *SignedMessage) Target() *core.RecordRef {
	return sm.Msg.Target()
}

// TargetRole returns jet role to actors of which Message should be sent.
func (sm *SignedMessage) TargetRole() core.JetRole {
	return sm.Msg.TargetRole()
}

// GetCaller returns initiator of this event.
func (sm *SignedMessage) GetCaller() *core.RecordRef {
	return sm.Msg.GetCaller()
}

func (sm *SignedMessage) GetSign() []byte {
	return sm.Signature
}

func (sm *SignedMessage) GetSender() core.RecordRef {
	return sm.Sender
}
