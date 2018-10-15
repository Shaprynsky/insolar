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

package storage_test

import (
	"sync"
	"testing"
	"time"

	"github.com/insolar/insolar/ledger/index"
	"github.com/insolar/insolar/ledger/record"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/insolar/insolar/ledger/storage/storagetest"
	"github.com/stretchr/testify/assert"
)

/*
check lock on select for update in 2 parallel transactions tx1 and tx2
which try reads and writes the same key simultaneously

  tx1                    tx2
   |                      |
<start>                 <start>
 get(k), for_update=T      |
 set(k)
   |----- proceed -------->|
 ..sleep..               get(k), for_update=T/F
 commit()                set(k)
  <end>                  commit()
                        <end>
*/

func TestStore_Transaction_LockOnUpdate(t *testing.T) {
	t.Parallel()
	db, cleaner := storagetest.TmpDB(t, "")
	defer cleaner()

	classid := &record.ID{Pulse: 100500}
	idxid := &record.ID{}
	classvalue0 := &index.ClassLifeline{
		LatestState: *classid,
	}
	db.SetClassIndex(idxid, classvalue0)

	rec1 := record.ID{Pulse: 1}
	rec2 := record.ID{Pulse: 2}
	lockfn := func(t *testing.T, withlock bool) *index.ClassLifeline {
		started2 := make(chan bool)
		proceed2 := make(chan bool)
		var wg sync.WaitGroup
		var tx1err error
		var tx2err error
		wg.Add(1)
		go func() {
			tx1err = db.Update(func(tx *storage.TransactionManager) error {
				// log.Debugf("tx1: start")
				<-started2
				// log.Debug("tx1: GetClassIndex before")
				idxlife, geterr := tx.GetClassIndex(idxid, true)
				// log.Debug("tx1: GetClassIndex after")
				if geterr != nil {
					return geterr
				}
				// log.Debugf("tx1: got %+v\n", idxlife)
				idxlife.AmendRefs = append(idxlife.AmendRefs, rec1)

				seterr := tx.SetClassIndex(idxid, idxlife)
				if seterr != nil {
					return seterr
				}
				// log.Debugf("tx1: set %+v\n", idxlife)
				close(proceed2)
				time.Sleep(100 * time.Millisecond)
				return seterr
			})
			// log.Debugf("tx1: finished")
			wg.Done()
		}()
		wg.Add(1)
		go func() {
			tx2err = db.Update(func(tx *storage.TransactionManager) error {
				close(started2)
				// log.Debug("tx2: start")
				<-proceed2
				// log.Debug("tx2: GetClassIndex before")
				idxlife, geterr := tx.GetClassIndex(idxid, withlock)
				// log.Debug("tx2: GetClassIndex after")
				if geterr != nil {
					return geterr
				}
				// log.Debugf("tx2: got %+v\n", idxlife)
				idxlife.AmendRefs = append(idxlife.AmendRefs, rec2)

				seterr := tx.SetClassIndex(idxid, idxlife)
				if seterr != nil {
					return seterr
				}
				// log.Debugf("tx2: set %+v\n", idxlife)
				return seterr
			})
			// log.Debugf("tx2: finished")
			wg.Done()
		}()
		wg.Wait()

		assert.NoError(t, tx1err)
		assert.NoError(t, tx2err)
		idxlife, geterr := db.GetClassIndex(idxid, false)
		assert.NoError(t, geterr)
		// log.Debugf("withlock=%v) result: got %+v", withlock, idxlife)

		// cleanup AmendRefs
		assert.NoError(t, db.SetClassIndex(idxid, classvalue0))
		return idxlife
	}
	t.Run("with lock", func(t *testing.T) {
		idxlife := lockfn(t, true)
		assert.Equal(t, &index.ClassLifeline{
			LatestState: *classid,
			AmendRefs:   []record.ID{rec1, rec2},
		}, idxlife)
	})
	t.Run("no lock", func(t *testing.T) {
		idxlife := lockfn(t, false)
		assert.Equal(t, &index.ClassLifeline{
			LatestState: *classid,
			AmendRefs:   []record.ID{rec1},
		}, idxlife)
	})
}
