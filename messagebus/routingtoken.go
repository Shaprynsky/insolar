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

package messagebus

import (
	"bytes"
	"crypto"
	"encoding/gob"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/pkg/errors"
)

type routingTokenFactory struct {
	Cryptography core.CryptographyService `inject:""`
}

func NewRoutingTokenFactory() message.RoutingTokenFactory {
	return &routingTokenFactory{}
}

func (rtf *routingTokenFactory) Create(to *core.RecordRef, from *core.RecordRef, pulseNumber core.PulseNumber, msgHash []byte) *message.RoutingToken {
	token := &message.RoutingToken{
		To:    to,
		From:  from,
		Pulse: pulseNumber,
	}

	var tokenBuffer bytes.Buffer
	enc := gob.NewEncoder(&tokenBuffer)
	err := enc.Encode(to)
	if err != nil {
		panic(err)
	}
	err = enc.Encode(from)
	if err != nil {
		panic(err)
	}
	err = enc.Encode(pulseNumber)
	if err != nil {
		panic(err)
	}
	tokenBuffer.Write(msgHash)

	signature, err := rtf.Cryptography.Sign(tokenBuffer.Bytes())
	if err != nil {
		panic(err)
	}
	token.Sign = signature.Bytes()

	return token
}

func (rtf *routingTokenFactory) Validate(publicKey crypto.PublicKey, token core.RoutingToken, msgHash []byte) error {
	var tokenBuffer bytes.Buffer
	enc := gob.NewEncoder(&tokenBuffer)
	err := enc.Encode(token.GetTo())
	if err != nil {
		panic(err)
	}
	err = enc.Encode(token.GetFrom())
	if err != nil {
		panic(err)
	}
	err = enc.Encode(token.GetPulse())
	if err != nil {
		panic(err)
	}
	tokenBuffer.Write(msgHash)

	ok := rtf.Cryptography.Verify(publicKey, core.SignatureFromBytes(tokenBuffer.Bytes()), token.GetSign())
	if !ok {
		return errors.New("token isn't valid")
	}
	return nil
}
