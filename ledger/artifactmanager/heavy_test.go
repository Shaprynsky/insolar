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
	"testing"

	"github.com/dgraph-io/badger"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerArtifactManager_handleHeavy(t *testing.T) {
	t.Parallel()
	ctx, db, _, cleaner := getTestData(t)
	defer cleaner()

	mh := NewMessageHandler(db, storage.NewRecentStorage(0))

	payload := []core.KV{
		{K: []byte("ABC"), V: []byte("CDE")},
		{K: []byte("ABC"), V: []byte("CDE")},
		{K: []byte("CDE"), V: []byte("ABC")},
	}

	parcel := &message.Parcel{
		Msg: &message.HeavyPayload{Records: payload},
	}

	var err error
	_, err = mh.handleHeavyPayload(ctx, parcel)
	require.NoError(t, err)

	badgerdb := db.GetBadgerDB()
	err = badgerdb.View(func(tx *badger.Txn) error {
		for _, kv := range payload {
			item, err := tx.Get(kv.K)
			if !assert.NoError(t, err) {
				continue
			}
			value, err := item.Value()
			if !assert.NoError(t, err) {
				continue
			}
			// fmt.Println("Got key:", string(item.Key()))
			assert.Equal(t, kv.V, value)
		}
		return nil
	})
	require.NoError(t, err)
}
