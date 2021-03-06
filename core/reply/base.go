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

// Package reply represents responses to messages of the messagebus
package reply

import (
	"bytes"
	"encoding/gob"
	"io"
	"io/ioutil"

	"github.com/insolar/insolar/core"
	"github.com/pkg/errors"
)

const (
	// Generic

	// TypeError is reply with error.
	TypeError = core.ReplyType(iota + 1)
	// TypeOK is a generic reply for success calls without returned value.
	TypeOK
	
	TypeGetObjectRedirect

	// Logicrunner

	// TypeCallMethod - two binary fields: data and results.
	TypeCallMethod
	// TypeCallConstructor - reference on created object
	TypeCallConstructor

	// Ledger

	// TypeCode is code from storage.
	TypeCode
	// TypeObject is object from storage.
	TypeObject
	// TypeDelegate is delegate reference from storage.
	TypeDelegate
	// TypeID is common reply for methods returning id to lifeline states.
	TypeID
	// TypeChildren is a reply for fetching objects children in chunks.
	TypeChildren
)

// ErrType is used to determine and compare reply errors.
type ErrType int

const (
	// ErrDeactivated returned when requested object is deactivated.
	ErrDeactivated = iota + 1
	ErrStateNotAvailable
)

func getEmptyReply(t core.ReplyType) (core.Reply, error) {
	switch t {
	case TypeCallMethod:
		return &CallMethod{}, nil
	case TypeCallConstructor:
		return &CallConstructor{}, nil
	case TypeCode:
		return &Code{}, nil
	case TypeObject:
		return &Object{}, nil
	case TypeDelegate:
		return &Delegate{}, nil
	case TypeID:
		return &ID{}, nil
	case TypeChildren:
		return &Children{}, nil
	case TypeError:
		return &Error{}, nil
	case TypeOK:
		return &OK{}, nil
	default:
		return nil, errors.Errorf("unimplemented reply type: '%d'", t)
	}
}

// Serialize returns encoded reply.
func Serialize(reply core.Reply) (io.Reader, error) {
	buff := &bytes.Buffer{}
	_, err := buff.Write([]byte{byte(reply.Type())})
	if err != nil {
		return nil, err
	}

	enc := gob.NewEncoder(buff)
	err = enc.Encode(reply)
	return buff, err
}

// Deserialize returns decoded reply.
func Deserialize(buff io.Reader) (core.Reply, error) {
	b := make([]byte, 1)
	_, err := buff.Read(b)
	if err != nil {
		return nil, errors.New("too short input to deserialize a message reply")
	}

	reply, err := getEmptyReply(core.ReplyType(b[0]))
	if err != nil {
		return nil, err
	}
	enc := gob.NewDecoder(buff)
	err = enc.Decode(reply)
	return reply, err
}

// ToBytes deserializes reply to bytes.
func ToBytes(rep core.Reply) []byte {
	repBuff, err := Serialize(rep)
	if err != nil {
		panic("failed to serialize reply")
	}
	buff, err := ioutil.ReadAll(repBuff)
	if err != nil {
		panic("failed to serialize reply")
	}
	return buff
}

func init() {
	gob.Register(&CallMethod{})
	gob.Register(&CallConstructor{})
	gob.Register(&Code{})
	gob.Register(&Object{})
	gob.Register(&Delegate{})
	gob.Register(&ID{})
	gob.Register(&Children{})
	gob.Register(&Error{})
	gob.Register(&OK{})
}
