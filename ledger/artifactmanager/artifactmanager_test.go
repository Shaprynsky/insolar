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
	"github.com/insolar/insolar/ledger/blockexplorer"
	"math/rand"
	"testing"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/cryptohelpers/hash"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/ledger/index"
	"github.com/insolar/insolar/ledger/record"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/insolar/insolar/ledger/storage/storagetest"
	"github.com/insolar/insolar/testutils/testmessagebus"
	"github.com/stretchr/testify/assert"
)

var (
	domainID   = *genRandomID(0)
	domainRef  = *core.NewRecordRef(domainID, domainID)
	requestRef = *genRandomRef(0)
)

func genRandomID(pulse core.PulseNumber) *core.RecordID {
	buff := [core.RecordIDSize - core.PulseNumberSize]byte{}
	_, err := rand.Read(buff[:])
	if err != nil {
		panic(err)
	}
	return core.NewRecordID(pulse, buff[:])
}

func genRefWithID(id *core.RecordID) *core.RecordRef {
	return core.NewRecordRef(domainID, *id)
}

func genRandomRef(pulse core.PulseNumber) *core.RecordRef {
	return genRefWithID(genRandomID(pulse))
}

func getTestData(t *testing.T) (
	context.Context,
	*storage.DB,
	*LedgerArtifactManager,
	*blockexplorer.BlockExplorerManager,
	func(), // cleaner
) {
	ctx := inslogger.TestContext(t)
	db, cleaner := storagetest.TmpDB(ctx, t, "")
	mb := testmessagebus.NewTestMessageBus()
	handler := MessageHandler{db: db, jetDropHandlers: map[core.MessageType]internalHandler{}}
	handler.Link(core.Components{MessageBus: mb})
	am := LedgerArtifactManager{
		db:                   db,
		messageBus:           mb,
		getChildrenChunkSize: 100,
	}
	be, _ := blockexplorer.NewBlockExplorer(db)
	be.Link(core.Components{MessageBus: mb})
	return ctx, db, &am, be, cleaner
}

func TestLedgerArtifactManager_RegisterRequest(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	msg := message.GenesisRequest{Name: "my little message"}
	id, err := am.RegisterRequest(ctx, &msg)
	assert.NoError(t, err)
	rec, err := db.GetRecord(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, message.MustSerializeBytes(&msg), rec.(*record.CallRequest).Payload)
}

func TestLedgerArtifactManager_DeclareType(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	typeDec := []byte{1, 2, 3}
	id, err := am.DeclareType(ctx, domainRef, requestRef, typeDec)
	assert.NoError(t, err)
	typeRec, err := db.GetRecord(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, &record.TypeRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain:  domainRef,
			Request: requestRef,
		},
		TypeDeclaration: typeDec,
	}, typeRec)
}

func TestLedgerArtifactManager_DeployCode_CreatesCorrectRecord(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	id, err := am.DeployCode(
		ctx,
		domainRef,
		requestRef,
		[]byte{1, 2, 3},
		core.MachineTypeBuiltin,
	)
	assert.NoError(t, err)
	codeRec, err := db.GetRecord(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, codeRec, &record.CodeRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain:  domainRef,
			Request: requestRef,
		},
		Code:        record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, []byte{1, 2, 3}),
		MachineType: core.MachineTypeBuiltin,
	})
}

func TestLedgerArtifactManager_ActivateObject_CreatesCorrectRecord(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	memory := []byte{1, 2, 3}
	codeRef := genRandomRef(0)
	parentID, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain: *genRandomRef(0),
			},
		},
	)
	db.SetObjectIndex(ctx, parentID, &index.ObjectLifeline{
		LatestState: parentID,
	})

	objRef := *genRandomRef(0)
	objDesc, err := am.ActivateObject(
		ctx,
		domainRef,
		objRef,
		*genRefWithID(parentID),
		*codeRef,
		false,
		memory,
	)
	assert.Nil(t, err)
	activateRec, err := db.GetRecord(ctx, objDesc.StateID())
	assert.Nil(t, err)
	assert.Equal(t, activateRec, &record.ObjectActivateRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain:  domainRef,
			Request: objRef,
		},
		ObjectStateRecord: record.ObjectStateRecord{
			Memory:      record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, memory),
			Image:       *codeRef,
			IsPrototype: false,
		},
		Parent:     *genRefWithID(parentID),
		IsDelegate: false,
	})

	idx, err := db.GetObjectIndex(ctx, parentID, false)
	assert.NoError(t, err)
	childRec, err := db.GetRecord(ctx, idx.ChildPointer)
	assert.NoError(t, err)
	assert.Equal(t, objRef, childRec.(*record.ChildRecord).Ref)

	idx, err = db.GetObjectIndex(ctx, objRef.Record(), false)
	assert.NoError(t, err)
	assert.Equal(t, *objDesc.StateID(), *idx.LatestState)
	assert.Equal(t, *objDesc.Parent(), idx.Parent)
}

func TestLedgerArtifactManager_DeactivateObject_CreatesCorrectRecord(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	objID, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain: *genRandomRef(0),
			},
		},
	)
	db.SetObjectIndex(ctx, objID, &index.ObjectLifeline{
		State:       record.StateActivation,
		LatestState: objID,
	})
	deactivateID, err := am.DeactivateObject(
		ctx,
		domainRef,
		requestRef,
		&ObjectDescriptor{
			ctx:   ctx,
			head:  *genRefWithID(objID),
			state: *objID,
		},
	)
	assert.Nil(t, err)
	deactivateRec, err := db.GetRecord(ctx, deactivateID)
	assert.Nil(t, err)
	assert.Equal(t, deactivateRec, &record.DeactivationRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain:  domainRef,
			Request: requestRef,
		},
		PrevState: *objID,
	})
}

func TestLedgerArtifactManager_UpdateObject_CreatesCorrectRecord(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	objID, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain: *genRandomRef(0),
			},
		},
	)
	db.SetObjectIndex(ctx, objID, &index.ObjectLifeline{
		State:       record.StateActivation,
		LatestState: objID,
	})
	memory := []byte{1, 2, 3}
	prototype := genRandomRef(0)
	obj, err := am.UpdateObject(
		ctx,
		domainRef,
		requestRef,
		&ObjectDescriptor{
			ctx:       ctx,
			head:      *genRefWithID(objID),
			state:     *objID,
			prototype: prototype,
		},
		memory,
	)
	assert.Nil(t, err)
	updateRec, err := db.GetRecord(ctx, obj.StateID())
	assert.Nil(t, err)
	assert.Equal(t, updateRec, &record.ObjectAmendRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain:  domainRef,
			Request: requestRef,
		},
		ObjectStateRecord: record.ObjectStateRecord{
			Memory:      record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, memory),
			Image:       *prototype,
			IsPrototype: false,
		},
		PrevState: *objID,
	})
}

func TestLedgerArtifactManager_GetObject_VerifiesRecords(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	objID := genRandomID(0)
	_, err := am.GetObject(ctx, *genRefWithID(objID), nil, false)
	assert.NotNil(t, err)

	deactivateID, _ := db.SetRecord(ctx, core.GenesisPulse.PulseNumber, &record.DeactivationRecord{})
	objectIndex := index.ObjectLifeline{
		LatestState: deactivateID,
	}
	db.SetObjectIndex(ctx, objID, &objectIndex)

	_, err = am.GetObject(ctx, *genRefWithID(objID), nil, false)
	assert.Equal(t, core.ErrDeactivated, err)
}

func TestLedgerArtifactManager_GetObject_ReturnsCorrectDescriptors(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	prototypeRef := genRandomRef(0)
	parentRef := genRandomRef(0)
	objRef := genRandomRef(0)
	db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain: domainRef,
			},
			ObjectStateRecord: record.ObjectStateRecord{
				Memory: record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, []byte{3}),
			},
			Parent: *parentRef,
		},
	)
	db.SetBlob(ctx, core.GenesisPulse.PulseNumber, []byte{3})
	objectAmendID, _ := db.SetRecord(ctx, core.GenesisPulse.PulseNumber, &record.ObjectAmendRecord{
		SideEffectRecord: record.SideEffectRecord{
			Domain: domainRef,
		},
		ObjectStateRecord: record.ObjectStateRecord{
			Memory: record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, []byte{4}),
			Image:  *prototypeRef,
		},
	})
	db.SetBlob(ctx, core.GenesisPulse.PulseNumber, []byte{4})
	objectIndex := index.ObjectLifeline{
		LatestState:  objectAmendID,
		ChildPointer: genRandomID(0),
		Parent:       *parentRef,
	}
	db.SetObjectIndex(ctx, objRef.Record(), &objectIndex)

	objDesc, err := am.GetObject(ctx, *objRef, nil, false)
	assert.NoError(t, err)
	expectedObjDesc := &ObjectDescriptor{
		ctx: ctx,
		am:  am,

		head:         *objRef,
		state:        *objectAmendID,
		prototype:    prototypeRef,
		isPrototype:  false,
		childPointer: objectIndex.ChildPointer,
		memory:       []byte{4},
		parent:       *parentRef,
	}

	assert.Equal(t, *expectedObjDesc, *objDesc.(*ObjectDescriptor))
}

func TestLedgerArtifactManager_GetChildren(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	parentID, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ObjectActivateRecord{
			SideEffectRecord: record.SideEffectRecord{
				Domain: domainRef,
			},
			ObjectStateRecord: record.ObjectStateRecord{
				Memory: record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, []byte{0}),
			},
		})
	child1Ref := genRandomRef(1)
	child2Ref := genRandomRef(1)
	child3Ref := genRandomRef(2)

	childMeta1, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ChildRecord{
			Ref: *child1Ref,
		})
	childMeta2, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ChildRecord{
			PrevChild: childMeta1,
			Ref:       *child2Ref,
		})
	childMeta3, _ := db.SetRecord(
		ctx,
		core.GenesisPulse.PulseNumber,
		&record.ChildRecord{
			PrevChild: childMeta2,
			Ref:       *child3Ref,
		})

	parentIndex := index.ObjectLifeline{
		LatestState:  parentID,
		ChildPointer: childMeta3,
	}
	db.SetObjectIndex(ctx, parentID, &parentIndex)

	t.Run("returns correct children without pulse", func(t *testing.T) {
		i, err := am.GetChildren(ctx, *genRefWithID(parentID), nil)
		assert.NoError(t, err)
		child, err := i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child3Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child2Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child1Ref, *child)
		hasNext := i.HasNext()
		assert.False(t, hasNext)
		_, err = i.Next()
		assert.Error(t, err)
	})

	t.Run("returns correct children with pulse", func(t *testing.T) {
		pn := core.PulseNumber(1)
		i, err := am.GetChildren(ctx, *genRefWithID(parentID), &pn)
		assert.NoError(t, err)
		child, err := i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child2Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child1Ref, *child)
		hasNext := i.HasNext()
		assert.NoError(t, err)
		assert.False(t, hasNext)
		_, err = i.Next()
		assert.Error(t, err)
	})

	t.Run("returns correct children in many chunks", func(t *testing.T) {
		am.getChildrenChunkSize = 1
		i, err := am.GetChildren(ctx, *genRefWithID(parentID), nil)
		assert.NoError(t, err)
		child, err := i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child3Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child2Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child1Ref, *child)
		hasNext := i.HasNext()
		assert.NoError(t, err)
		assert.False(t, hasNext)
		_, err = i.Next()
		assert.Error(t, err)
	})

	t.Run("doesn't fail when has no children to return", func(t *testing.T) {
		am.getChildrenChunkSize = 1
		pn := core.PulseNumber(3)
		i, err := am.GetChildren(ctx, *genRefWithID(parentID), &pn)
		assert.NoError(t, err)
		child, err := i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child3Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child2Ref, *child)
		child, err = i.Next()
		assert.NoError(t, err)
		assert.Equal(t, *child1Ref, *child)
		hasNext := i.HasNext()
		assert.NoError(t, err)
		assert.False(t, hasNext)
		_, err = i.Next()
		assert.Error(t, err)
	})
}

func TestLedgerArtifactManager_HandleJetDrop(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	codeRecord := record.CodeRecord{
		Code: record.CalculateIDForBlob(core.GenesisPulse.PulseNumber, []byte{1, 2, 3, 3, 2, 1}),
	}
	recHash := hash.NewIDHash()
	_, err := codeRecord.WriteHashData(recHash)
	assert.NoError(t, err)
	latestPulse, err := db.GetLatestPulseNumber(ctx)
	assert.NoError(t, err)
	id := core.NewRecordID(latestPulse, recHash.Sum(nil))

	setRecordMessage := message.SetRecord{
		Record: record.SerializeRecord(&codeRecord),
	}

	rep, err := am.messageBus.Send(
		ctx,
		&message.JetDrop{
			Messages: [][]byte{
				message.ToBytes(&setRecordMessage),
			},
			PulseNumber: core.GenesisPulse.PulseNumber,
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, reply.OK{}, *rep.(*reply.OK))

	rec, err := db.GetRecord(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, codeRecord, *rec.(*record.CodeRecord))
}

func TestLedgerArtifactManager_RegisterValidation(t *testing.T) {
	t.Parallel()
	ctx, _, am, _, cleaner := getTestData(t)
	defer cleaner()

	objID, err := am.RegisterRequest(ctx, &message.GenesisRequest{Name: "object"})
	objRef := genRefWithID(objID)
	assert.NoError(t, err)

	desc, err := am.ActivateObject(
		ctx,
		domainRef,
		*objRef,
		*am.GenesisRef(),
		*genRandomRef(0),
		false,
		[]byte{1},
	)
	assert.NoError(t, err)
	stateID1 := desc.StateID()

	desc, err = am.GetObject(ctx, *objRef, nil, false)
	assert.NoError(t, err)
	assert.Equal(t, *stateID1, *desc.StateID())

	_, err = am.GetObject(ctx, *objRef, nil, true)
	assert.Equal(t, err, core.ErrStateNotAvailable)

	desc, err = am.UpdateObject(
		ctx,
		domainRef,
		*genRandomRef(0),
		desc,
		[]byte{2},
	)
	assert.NoError(t, err)
	stateID2 := desc.StateID()

	desc, err = am.GetObject(ctx, *objRef, nil, false)
	assert.NoError(t, err)
	desc, err = am.UpdateObject(
		ctx,
		domainRef,
		*genRandomRef(0),
		desc,
		[]byte{3},
	)
	assert.NoError(t, err)
	stateID3 := desc.StateID()
	err = am.RegisterValidation(ctx, *objRef, *stateID2, true, nil)
	assert.NoError(t, err)

	desc, err = am.GetObject(ctx, *objRef, nil, false)
	assert.NoError(t, err)
	assert.Equal(t, *stateID3, *desc.StateID())
	desc, err = am.GetObject(ctx, *objRef, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, *stateID2, *desc.StateID())
}

func TestLedgerArtifactManager_RegisterResult(t *testing.T) {
	t.Parallel()
	ctx, db, am, _, cleaner := getTestData(t)
	defer cleaner()

	request := genRandomRef(0)
	requestID, err := am.RegisterResult(ctx, *request, []byte{1, 2, 3})
	assert.NoError(t, err)

	rec, err := db.GetRecord(ctx, requestID)
	assert.NoError(t, err)
	assert.Equal(t, record.ResultRecord{Request: *request, Payload: []byte{1, 2, 3}}, *rec.(*record.ResultRecord))
}
