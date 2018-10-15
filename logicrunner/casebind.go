/*
 *
 *  *    Copyright 2018 Insolar
 *  *
 *  *    Licensed under the Apache License, Version 2.0 (the "License");
 *  *    you may not use this file except in compliance with the License.
 *  *    You may obtain a copy of the License at
 *  *
 *  *        http://www.apache.org/licenses/LICENSE-2.0
 *  *
 *  *    Unless required by applicable law or agreed to in writing, software
 *  *    distributed under the License is distributed on an "AS IS" BASIS,
 *  *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  *    See the License for the specific language governing permissions and
 *  *    limitations under the License.
 *
 */

package logicrunner

import (
	"bytes"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/reply"
	"github.com/pkg/errors"
	"github.com/ugorji/go/codec"
	"golang.org/x/crypto/sha3"
)

func HashInterface(in interface{}) []byte {
	s := []byte{}
	ch := new(codec.CborHandle)
	err := codec.NewEncoderBytes(&s, ch).Encode(in)
	if err != nil {
		panic("Can't marshal: " + err.Error())
	}
	sh := sha3.New224()
	return sh.Sum(s)
}

func (lr *LogicRunner) addObjectCaseRecord(ref core.RecordRef, cr core.CaseRecord) {
	lr.caseBindMutex.Lock()
	lr.caseBind.Records[ref] = append(lr.caseBind.Records[ref], cr)
	lr.caseBindMutex.Unlock()
}

func (lr *LogicRunner) lastObjectCaseRecord(ref core.RecordRef) core.CaseRecord {
	lr.caseBindMutex.Lock()
	defer lr.caseBindMutex.Unlock()
	list := lr.caseBind.Records[ref]
	return list[len(list)-1]
}

func (lr *LogicRunner) getNextValidationStep(ref core.RecordRef) (*core.CaseRecord, int) {
	lr.caseBindReplaysMutex.Lock()
	defer lr.caseBindReplaysMutex.Unlock()
	r, ok := lr.caseBindReplays[ref]
	if !ok {
		return nil, -1
	} else if r.RecordsLen <= r.Step {
		return nil, r.Step
	}
	ret := r.Records[r.Step]
	r.Step++
	lr.caseBindReplays[ref] = r
	return &ret, r.Step
}

func (lr *LogicRunner) Validate(ref core.RecordRef, p core.Pulse, cr []core.CaseRecord) (int, error) {
	if len(cr) < 1 {
		return 0, errors.New("casebind is empty")
	}

	err := func() error {
		lr.caseBindReplaysMutex.Lock()
		defer lr.caseBindReplaysMutex.Unlock()
		if _, ok := lr.caseBindReplays[ref]; ok {
			return errors.New("already validating this ref")
		}
		lr.caseBindReplays[ref] = core.CaseBindReplay{
			Pulse:      p,
			Records:    cr,
			RecordsLen: len(cr),
			Step:       0,
		}
		return nil
	}()
	if err != nil {
		return 0, err
	}

	defer func() {
		lr.caseBindReplaysMutex.Lock()
		defer lr.caseBindReplaysMutex.Unlock()
		delete(lr.caseBindReplays, ref)
	}()

	for {
		start, step := lr.getNextValidationStep(ref)
		if step < 0 {
			return step, errors.New("no validation data")
		} else if start == nil { // finish
			return step, nil
		}
		if start.Type != core.CaseRecordTypeStart {
			return step, errors.New("step between two shores")
		}

		ret, err := lr.Execute(start.Resp.(core.Message))
		if err != nil {
			return 0, errors.Wrap(err, "validation step failed")
		}
		stop, step := lr.getNextValidationStep(ref)
		if step < 0 {
			return 0, errors.New("validation container broken")
		} else if stop.Type != core.CaseRecordTypeResult {
			return step, errors.New("Validation stoped not on result")
		}

		switch need := stop.Resp.(type) {
		case *reply.CallMethod:
			if got, ok := ret.(*reply.CallMethod); !ok {
				return step, errors.New("not result type callmethod")
			} else if !bytes.Equal(got.Data, need.Data) {
				return step, errors.New("body mismatch")
			} else if !bytes.Equal(got.Result, need.Result) {
				return step, errors.New("result mismatch")
			}
		case *reply.CallConstructor:
			if got, ok := ret.(*reply.CallConstructor); !ok {
				return step, errors.New("not result type callmethod")
			} else if !got.Object.Equal(*need.Object) {
				return step, errors.New("constructed refs mismatch mismatch")
			}
		default:
			return step, errors.New("unknown result type")
		}
	}
}

// ValidationBehaviour is a special object that responsible for validation behavior of other methods.
type ValidationBehaviour interface {
	Begin(refs core.RecordRef, record core.CaseRecord)
	End(refs core.RecordRef, record core.CaseRecord)
	ModifyContext(ctx *core.LogicCallContext)
	NeedSave() bool
}

type ValidationSaver struct {
	lr *LogicRunner
}

func (vb ValidationSaver) NeedSave() bool {
	return true
}

func (vb ValidationSaver) ModifyContext(ctx *core.LogicCallContext) {
	// nothing need
}

func (vb ValidationSaver) Begin(refs core.RecordRef, record core.CaseRecord) {
	vb.lr.addObjectCaseRecord(refs, record)
}

func (vb ValidationSaver) End(refs core.RecordRef, record core.CaseRecord) {
	vb.lr.addObjectCaseRecord(refs, record)
}

type ValidationChecker struct {
	lr *LogicRunner
	cb core.CaseBindReplay
}

func (vb ValidationChecker) NeedSave() bool {
	return false
}

func (vb ValidationChecker) ModifyContext(ctx *core.LogicCallContext) {
	ctx.Pulse = vb.cb.Pulse
}

func (vb ValidationChecker) Begin(refs core.RecordRef, record core.CaseRecord) {
	// do nothing, everything done in lr.Validate
}

func (vb ValidationChecker) End(refs core.RecordRef, record core.CaseRecord) {
	// do nothing, everything done in lr.Validate
}
