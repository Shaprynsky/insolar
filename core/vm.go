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

package core

import (
	"context"
	"encoding/gob"
	"time"
)

// MachineType is a type of virtual machine
type MachineType int

// Real constants of MachineType
const (
	MachineTypeNotExist             = 0
	MachineTypeBuiltin  MachineType = iota + 1
	MachineTypeGoPlugin

	MachineTypesLastID
)

// MachineLogicExecutor is an interface for implementers of one particular machine type
type MachineLogicExecutor interface {
	CallMethod(
		ctx context.Context, callContext *LogicCallContext,
		code RecordRef, data []byte,
		method string, args Arguments,
	) (
		newObjectState []byte, methodResults Arguments, err error,
	)
	CallConstructor(
		ctx context.Context, callContext *LogicCallContext,
		code RecordRef, name string, args Arguments,
	) (
		objectState []byte, err error,
	)
	Stop() error
}

// LogicRunner is an interface that should satisfy logic executor
//go:generate minimock -i github.com/insolar/insolar/core.LogicRunner -o ../testutils -s _mock.go
type LogicRunner interface {
	Execute(context.Context, Parcel) (res Reply, err error)
	ValidateCaseBind(context.Context, Parcel) (res Reply, err error)
	ProcessValidationResults(context.Context, Parcel) (res Reply, err error)
	ExecutorResults(context.Context, Parcel) (res Reply, err error)
	Validate(ctx context.Context, ref RecordRef, p Pulse, cb CaseBind) (int, error) // TODO hide?
	OnPulse(context.Context, Pulse) error
}

// LogicCallContext is a context of contract execution
type LogicCallContext struct {
	Callee          *RecordRef // Contract that was called
	Request         *RecordRef // ref of request
	Prototype       *RecordRef // Image of the callee
	Code            *RecordRef // ref of contract code
	CallerPrototype *RecordRef // Image of the caller
	Parent          *RecordRef // Parent of the callee
	Caller          *RecordRef // Contract that made the call
	Time            time.Time  // Time when call was made
	Pulse           Pulse      // Number of the pulse
	TraceID         string
}

// CaseRecordType is a type of caserecord
type CaseRecordType int

// Types of records
const (
	caseRecordTypeUnexistent CaseRecordType = iota
	CaseRecordTypeStart
	CaseRecordTypeTraceID
	CaseRecordTypeResult
	CaseRecordTypeRequest
	CaseRecordTypeGetObject
	CaseRecordTypeSignObject
	CaseRecordTypeRouteCall
	CaseRecordTypeSaveAsChild
	CaseRecordTypeGetObjChildren
	CaseRecordTypeSaveAsDelegate
	CaseRecordTypeGetDelegate
	CaseRecordTypeDeactivateObject
)

// CaseRecord is one record of validateable object calling history
type CaseRecord struct {
	Type   CaseRecordType
	ReqSig []byte
	Resp   interface{}
}

type CaseRequest struct {
	Request interface{}
	Records []CaseRecord
}

// CaseBinder is a whole result of executor efforts on every object it seen on this pulse
type CaseBind struct {
	Requests []CaseRequest
}

type CaseBindReplay struct {
	Pulse    Pulse
	CaseBind CaseBind
	Request  int
	Record   int
	Steps    int
	Fail     int
}

func (r *CaseBindReplay) NextStep() (*CaseRecord, int) {
	if r.Request >= len(r.CaseBind.Requests) {
		return nil, r.Steps
	}

	request := r.CaseBind.Requests[r.Request]

	if r.Record < 0 {
		r.Record = 0
		r.Steps++
		res := request.Request.(CaseRecord)
		return &res, r.Steps
	}

	if r.Record >= len(request.Records) {
		r.Record = -1
		r.Request++
		if r.Request >= len(r.CaseBind.Requests) {
			return nil, r.Steps
		}
		r.Record = 0
		r.Steps++
		res := r.CaseBind.Requests[r.Request].Request.(CaseRecord)
		return &res, r.Steps
	}
	res := request.Records[r.Record]
	r.Record++
	r.Steps++
	return &res, r.Steps
}

func init() {
	gob.Register(&CaseRecord{})
	gob.Register(&CaseRequest{})
	gob.Register(&CaseBind{})
}
