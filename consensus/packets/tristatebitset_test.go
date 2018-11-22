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

package packets

import (
	"testing"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/testutils"
	"github.com/stretchr/testify/assert"
)

const refsCount = 5

func initRefs() []core.RecordRef {
	result := make([]core.RecordRef, refsCount)
	for i := 0; i < refsCount; i++ {
		result[i] = testutils.RandomRef()
	}
	return result
}

func initBitCells(refs []core.RecordRef) []BitSetCell {
	result := make([]BitSetCell, refsCount)
	for i, ref := range refs {
		result[i] = BitSetCell{NodeID: ref, State: TimedOut}
	}
	return result
}

type BitSetMapperMock struct {
	refs []core.RecordRef
}

func (bsmm *BitSetMapperMock) IndexToRef(index int) (core.RecordRef, error) {
	if index > refsCount-1 {
		return testutils.RandomRef(), ErrBitSetOutOfRange
	}
	return bsmm.refs[index], nil
}

func (bsmm *BitSetMapperMock) RefToIndex(nodeID core.RecordRef) (int, error) {
	for i, ref := range bsmm.refs {
		if ref == nodeID {
			return i, nil
		}
	}
	return 0, ErrBitSetNodeIsMissing
}

func (bsmm *BitSetMapperMock) Length() int {
	return refsCount
}

func TestNewTriStateBitSet(t *testing.T) {
	refs := initRefs()
	cells := initBitCells(refs)

	_, err := NewBitSet(cells, &BitSetMapperMock{refs: refs})
	assert.NoError(t, err)
}

func TestTriStateBitSet_GetBuckets(t *testing.T) {
	refs := initRefs()
	cells := initBitCells(refs)

	bitset, _ := NewBitSet(cells, &BitSetMapperMock{refs: refs})
	assert.Equal(t, cells, bitset.GetCells())
}

func TestTriStateBitSet_ApplyChanges(t *testing.T) {
	refs := initRefs()
	cells := initBitCells(refs)

	bitset, _ := NewBitSet(cells, &BitSetMapperMock{refs: refs})

	cells[refsCount-3].State = Fraud
	bitset.ApplyChanges(cells)
	assert.Equal(t, cells, bitset.GetCells())
	cells[refsCount-4].State = Legit
	assert.NotEqual(t, cells, bitset.GetCells())
}

func TestBitArray(t *testing.T) {
	refs := initRefs()
	cells := initBitCells(refs)

	bitset, _ := NewTriStateBitSet(cells, &BitSetMapperMock{refs: refs})

	array1, err := bitset.cellsToBitArray()
	assert.NoError(t, err)

	cells[refsCount-3].State = Fraud
	bitset.ApplyChanges(cells)
	array2, err := bitset.cellsToBitArray()
	assert.NoError(t, err)
	err = changeBitState(array1, refsCount-3, Fraud)
	assert.NoError(t, err)

	assert.Equal(t, array1, array2)
}
