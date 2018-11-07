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

package artifactmanager

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/stats"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/cryptohelpers/hash"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/instrumentation/insmetrics"
	"github.com/insolar/insolar/ledger/record"
	"github.com/insolar/insolar/ledger/storage"
)

const (
	getChildrenChunkSize = 10 * 1000
)

// LedgerArtifactManager provides concrete API to storage for processing module.
type LedgerArtifactManager struct {
	db         *storage.DB
	messageBus core.MessageBus

	getChildrenChunkSize int
}

// State returns hash state for artifact manager.
func (m *LedgerArtifactManager) State() ([]byte, error) {
	// This is a temporary stab to simulate real hash.
	return hash.SHA3Bytes256([]byte{1, 2, 3}), nil
}

// NewArtifactManger creates new manager instance.
func NewArtifactManger(db *storage.DB) (*LedgerArtifactManager, error) {
	return &LedgerArtifactManager{db: db, getChildrenChunkSize: getChildrenChunkSize}, nil
}

// Link links external components.
func (m *LedgerArtifactManager) Link(components core.Components) error {
	m.messageBus = components.MessageBus

	return nil
}

// GenesisRef returns the root record reference.
//
// Root record is the parent for all top-level records.
func (m *LedgerArtifactManager) GenesisRef() *core.RecordRef {
	return m.db.GenesisRef()
}

// RegisterRequest sends message for request registration,
// returns request record Ref if request successfully created or already exists.
func (m *LedgerArtifactManager) RegisterRequest(
	ctx context.Context, msg core.Message,
) (*core.RecordID, error) {
	defer instrumentation(ctx, "RegisterRequest", time.Now())

	return m.setRecord(
		ctx,
		&record.CallRequest{
			Payload: message.MustSerializeBytes(msg),
		},
		*msg.Target(),
	)
}

// GetCode returns code from code record by provided reference according to provided machine preference.
//
// This method is used by VM to fetch code for execution.
func (m *LedgerArtifactManager) GetCode(
	ctx context.Context, code core.RecordRef,
) (core.CodeDescriptor, error) {
	defer instrumentation(ctx, "GetCode", time.Now())

	genericReact, err := m.messageBus.Send(
		ctx,
		&message.GetCode{Code: code},
	)

	if err != nil {
		return nil, err
	}

	react, ok := genericReact.(*reply.Code)
	if !ok {
		return nil, ErrUnexpectedReply
	}

	desc := CodeDescriptor{
		ctx:         ctx,
		ref:         code,
		machineType: react.MachineType,
	}
	desc.cache.code = react.Code
	return &desc, nil
}

// GetObject returns descriptor for provided state.
//
// If provided state is nil, the latest state will be returned (with deactivation check). Returned descriptor will
// provide methods for fetching all related data.
func (m *LedgerArtifactManager) GetObject(
	ctx context.Context, head core.RecordRef, state *core.RecordID, approved bool,
) (core.ObjectDescriptor, error) {
	defer instrumentation(ctx, "GetObject", time.Now())

	genericReact, err := m.messageBus.Send(
		ctx,
		&message.GetObject{
			Head:     head,
			State:    state,
			Approved: approved,
		},
	)

	if err != nil {
		return nil, err
	}

	switch r := genericReact.(type) {
	case *reply.Object:
		desc := ObjectDescriptor{
			ctx:          ctx,
			am:           m,
			head:         r.Head,
			state:        r.State,
			prototype:    r.Prototype,
			isPrototype:  r.IsPrototype,
			childPointer: r.ChildPointer,
			memory:       r.Memory,
			parent:       r.Parent,
		}
		return &desc, nil
	case *reply.Error:
		return nil, r.Error()
	}

	return nil, ErrUnexpectedReply
}

// GetDelegate returns provided object's delegate reference for provided prototype.
//
// Object delegate should be previously created for this object. If object delegate does not exist, an error will
// be returned.
func (m *LedgerArtifactManager) GetDelegate(
	ctx context.Context, head, asType core.RecordRef,
) (*core.RecordRef, error) {
	defer instrumentation(ctx, "GetDelegate", time.Now())

	genericReact, err := m.messageBus.Send(
		ctx,
		&message.GetDelegate{
			Head:   head,
			AsType: asType,
		},
	)

	if err != nil {
		return nil, err
	}

	react, ok := genericReact.(*reply.Delegate)
	if !ok {
		return nil, ErrUnexpectedReply
	}
	return &react.Head, nil
}

// GetChildren returns children iterator.
//
// During iteration children refs will be fetched from remote source (parent object).
func (m *LedgerArtifactManager) GetChildren(
	ctx context.Context, parent core.RecordRef, pulse *core.PulseNumber,
) (core.RefIterator, error) {
	defer instrumentation(ctx, "GetChildren", time.Now())
	return NewChildIterator(ctx, m.messageBus, parent, pulse, m.getChildrenChunkSize)
}

// DeclareType creates new type record in storage.
//
// Type is a contract interface. It contains one method signature.
func (m *LedgerArtifactManager) DeclareType(
	ctx context.Context, domain, request core.RecordRef, typeDec []byte,
) (*core.RecordID, error) {
	defer instrumentation(ctx, "DeclareType", time.Now())

	return m.setRecord(
		ctx,
		&record.TypeRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain:  domain,
				Request: request,
			},
			TypeDeclaration: typeDec,
		},
		request,
	)
}

// DeployCode creates new code record in storage.
//
// CodeRef records are used to activate prototype or as migration code for an object.
func (m *LedgerArtifactManager) DeployCode(
	ctx context.Context,
	domain core.RecordRef,
	request core.RecordRef,
	code []byte,
	machineType core.MachineType,
) (*core.RecordID, error) {
	defer instrumentation(ctx, "DeployCode", time.Now())

	pulseNumber, err := m.db.GetLatestPulseNumber(ctx)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var setRecord *core.RecordID
	var setRecordErr error
	go func() {
		setRecord, setRecordErr = m.setRecord(
			ctx,
			&record.CodeRecord{
				SideEffectRecord: record.SideEffectRecord{
					Domain:  domain,
					Request: request,
				},
				Code:        record.CalculateIDForBlob(pulseNumber, code),
				MachineType: machineType,
			},
			request,
		)
		wg.Done()
	}()

	var setBlobErr error
	go func() {
		_, setBlobErr = m.setBlob(ctx, code, request)
		wg.Done()
	}()
	wg.Wait()

	if setRecordErr != nil {
		return nil, setRecordErr
	}
	if setBlobErr != nil {
		return nil, setBlobErr
	}

	return setRecord, nil
}

// ActivatePrototype creates activate object record in storage. Provided prototype reference will be used as objects prototype
// memory as memory of created object. If memory is not provided, the prototype default memory will be used.
//
// Request reference will be this object's identifier and referred as "object head".
func (m *LedgerArtifactManager) ActivatePrototype(
	ctx context.Context,
	domain, object, parent, code core.RecordRef,
	memory []byte,
) (core.ObjectDescriptor, error) {
	defer instrumentation(ctx, "ActivatePrototype", time.Now())
	return m.activateObject(ctx, domain, object, code, true, parent, false, memory)
}

// ActivateObject creates activate object record in storage. Provided prototype reference will be used as objects prototype
// memory as memory of created object. If memory is not provided, the prototype default memory will be used.
//
// Request reference will be this object's identifier and referred as "object head".
func (m *LedgerArtifactManager) ActivateObject(
	ctx context.Context,
	domain, object, parent, prototype core.RecordRef,
	asDelegate bool,
	memory []byte,
) (core.ObjectDescriptor, error) {
	defer instrumentation(ctx, "ActivateObject", time.Now())
	return m.activateObject(ctx, domain, object, prototype, false, parent, asDelegate, memory)
}

// DeactivateObject creates deactivate object record in storage. Provided reference should be a reference to the head
// of the object. If object is already deactivated, an error should be returned.
//
// Deactivated object cannot be changed.
func (m *LedgerArtifactManager) DeactivateObject(
	ctx context.Context, domain, request core.RecordRef, object core.ObjectDescriptor,
) (*core.RecordID, error) {
	defer instrumentation(ctx, "DeactivateObject", time.Now())

	desc, err := m.sendUpdateObject(
		ctx,
		&record.DeactivationRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain:  domain,
				Request: request,
			},
			PrevState: *object.StateID(),
		},
		*object.HeadRef(),
		nil,
	)
	if err != nil {
		return nil, err
	}
	return &desc.State, nil
}

// UpdatePrototype creates amend object record in storage. Provided reference should be a reference to the head of the
// prototype. Provided memory well be the new object memory.
//
// Returned reference will be the latest object state (exact) reference.
func (m *LedgerArtifactManager) UpdatePrototype(
	ctx context.Context,
	domain, request core.RecordRef,
	object core.ObjectDescriptor,
	memory []byte,
	code *core.RecordRef,
) (core.ObjectDescriptor, error) {
	if !object.IsPrototype() {
		return nil, errors.New("object is not a prototype")
	}
	defer instrumentation(ctx, "UpdatePrototype", time.Now())
	return m.updateObject(ctx, domain, request, object, code, memory)
}

// UpdateObject creates amend object record in storage. Provided reference should be a reference to the head of the
// object. Provided memory well be the new object memory.
//
// Returned reference will be the latest object state (exact) reference.
func (m *LedgerArtifactManager) UpdateObject(
	ctx context.Context,
	domain, request core.RecordRef,
	object core.ObjectDescriptor,
	memory []byte,
) (core.ObjectDescriptor, error) {
	if object.IsPrototype() {
		return nil, errors.New("object is not an instance")
	}
	defer instrumentation(ctx, "UpdateObject", time.Now())
	return m.updateObject(ctx, domain, request, object, nil, memory)
}

// RegisterValidation marks provided object state as approved or disapproved.
//
// When fetching object, validity can be specified.
func (m *LedgerArtifactManager) RegisterValidation(
	ctx context.Context, object core.RecordRef, state core.RecordID, isValid bool, validationMessages []core.Message,
) error {
	defer instrumentation(ctx, "RegisterValidation", time.Now())

	msg := message.ValidateRecord{
		Object:             object,
		State:              state,
		IsValid:            isValid,
		ValidationMessages: validationMessages,
	}
	_, err := m.messageBus.Send(ctx, &msg)
	if err != nil {
		return err
	}

	return nil
}

// RegisterResult saves VM method call result.
func (m *LedgerArtifactManager) RegisterResult(
	ctx context.Context, request core.RecordRef, payload []byte,
) (*core.RecordID, error) {
	defer instrumentation(ctx, "RegisterResult", time.Now())

	return m.setRecord(
		ctx,
		&record.ResultRecord{
			Request: request,
			Payload: payload,
		},
		request,
	)
}

func (m *LedgerArtifactManager) activateObject(
	ctx context.Context,
	domain core.RecordRef,
	object core.RecordRef,
	prototype core.RecordRef,
	isPrototype bool,
	parent core.RecordRef,
	asDelegate bool,
	memory []byte,
) (core.ObjectDescriptor, error) {
	parentDesc, err := m.GetObject(ctx, parent, nil, false)
	if err != nil {
		return nil, err
	}
	pulseNumber, err := m.db.GetLatestPulseNumber(ctx)
	if err != nil {
		return nil, err
	}

	obj, err := m.sendUpdateObject(
		ctx,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain:  domain,
				Request: object,
			},
			ObjectStateRecord: record.ObjectStateRecord{
				Memory:      record.CalculateIDForBlob(pulseNumber, memory),
				Image:       prototype,
				IsPrototype: isPrototype,
			},
			Parent:     parent,
			IsDelegate: asDelegate,
		},
		object,
		memory,
	)
	if err != nil {
		return nil, err
	}

	var (
		prevChild *core.RecordID
		asType    *core.RecordRef
	)
	if parentDesc.ChildPointer() != nil {
		prevChild = parentDesc.ChildPointer()
	}
	if asDelegate {
		asType = &prototype
	}
	_, err = m.registerChild(
		ctx,
		&record.ChildRecord{
			Ref:       object,
			PrevChild: prevChild,
		},
		parent,
		object,
		asType,
	)
	if err != nil {
		return nil, err
	}

	return &ObjectDescriptor{
		ctx:          ctx,
		am:           m,
		head:         obj.Head,
		state:        obj.State,
		prototype:    obj.Prototype,
		childPointer: obj.ChildPointer,
		memory:       memory,
		parent:       obj.Parent,
	}, nil
}

func (m *LedgerArtifactManager) updateObject(
	ctx context.Context,
	domain, request core.RecordRef,
	object core.ObjectDescriptor,
	code *core.RecordRef,
	memory []byte,
) (core.ObjectDescriptor, error) {
	var (
		image *core.RecordRef
		err   error
	)
	if object.IsPrototype() {
		if code != nil {
			image = code
		} else {
			image, err = object.Code()
		}
	} else {
		image, err = object.Prototype()
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to update object")
	}

	pulseNumber, err := m.db.GetLatestPulseNumber(ctx)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	obj, err := m.sendUpdateObject(
		ctx,
		&record.ObjectAmendRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain:  domain,
				Request: request,
			},
			ObjectStateRecord: record.ObjectStateRecord{
				Memory:      record.CalculateIDForBlob(pulseNumber, memory),
				Image:       *image,
				IsPrototype: object.IsPrototype(),
			},
			PrevState: *object.StateID(),
		},
		*object.HeadRef(),
		memory,
	)
	if err != nil {
		return nil, err
	}

	return &ObjectDescriptor{
		ctx:          ctx,
		am:           m,
		head:         obj.Head,
		state:        obj.State,
		prototype:    obj.Prototype,
		childPointer: obj.ChildPointer,
		memory:       memory,
		parent:       obj.Parent,
	}, nil
}

func (m *LedgerArtifactManager) setRecord(ctx context.Context, rec record.Record, target core.RecordRef) (*core.RecordID, error) {
	genericReact, err := m.messageBus.Send(
		ctx,
		&message.SetRecord{
			Record:    record.SerializeRecord(rec),
			TargetRef: target,
		},
	)

	if err != nil {
		return nil, err
	}

	react, ok := genericReact.(*reply.ID)
	if !ok {
		return nil, ErrUnexpectedReply
	}

	return &react.ID, nil
}

func (m *LedgerArtifactManager) setBlob(ctx context.Context, blob []byte, target core.RecordRef) (*core.RecordID, error) {
	genericReact, err := m.messageBus.Send(
		ctx,
		&message.SetBlob{
			Memory:    blob,
			TargetRef: target,
		},
	)

	if err != nil {
		return nil, err
	}

	react, ok := genericReact.(*reply.ID)
	if !ok {
		return nil, ErrUnexpectedReply
	}

	return &react.ID, nil
}

func (m *LedgerArtifactManager) sendUpdateObject(
	ctx context.Context,
	rec record.Record,
	object core.RecordRef,
	memory []byte,
) (*reply.Object, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var genericReact core.Reply
	var genericError error
	go func() {
		genericReact, genericError = m.messageBus.Send(
			ctx,
			&message.UpdateObject{
				Record: record.SerializeRecord(rec),
				Object: object,
			},
		)
		wg.Done()
	}()

	var blobReact core.Reply
	var blobError error
	go func() {
		blobReact, blobError = m.messageBus.Send(
			ctx,
			&message.SetBlob{
				TargetRef: object,
				Memory:    memory,
			},
		)
		wg.Done()
	}()

	wg.Wait()

	if genericError != nil {
		return nil, genericError
	}
	if blobError != nil {
		return nil, blobError
	}

	rep, ok := genericReact.(*reply.Object)
	if !ok {
		return nil, ErrUnexpectedReply
	}
	_, ok = blobReact.(*reply.ID)
	if !ok {
		return nil, ErrUnexpectedReply
	}

	return rep, nil
}

func (m *LedgerArtifactManager) registerChild(
	ctx context.Context,
	rec record.Record,
	parent core.RecordRef,
	child core.RecordRef,
	asType *core.RecordRef,
) (*core.RecordID, error) {
	genericReact, err := m.messageBus.Send(
		ctx,
		&message.RegisterChild{
			Record: record.SerializeRecord(rec),
			Parent: parent,
			Child:  child,
			AsType: asType,
		},
	)

	if err != nil {
		return nil, err
	}

	react, ok := genericReact.(*reply.ID)
	if !ok {
		return nil, ErrUnexpectedReply
	}

	return &react.ID, nil
}

func instrumentation(ctx context.Context, name string, start time.Time) {
	ctx = insmetrics.InsertTag(ctx, tagMethod, name)
	latency := time.Since(start)
	stats.Record(ctx, statCalls.M(1), statLatency.M(latency.Nanoseconds()/1e6))
	inslogger.FromContext(ctx).Debug("measured time is ", latency)
}

// GetHistory returns history iterator.
//
// During iteration history will be fetched from remote source.
func (m *LedgerArtifactManager) GetHistory(
	ctx context.Context, parent core.RecordRef, pulse *core.PulseNumber,
) (core.RefIterator, error) {
	defer instrumentation(ctx, "GetHistory", time.Now())
	return NewHistoryIterator(ctx, m.messageBus, parent, pulse, m.getChildrenChunkSize)
}
