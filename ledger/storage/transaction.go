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

package storage

import (
	"github.com/dgraph-io/badger"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/cryptohelpers/hash"
	"github.com/insolar/insolar/ledger/index"
	"github.com/insolar/insolar/ledger/record"
	"github.com/insolar/insolar/log"
)

type keyval struct {
	k []byte
	v []byte
}

// TransactionManager is used to ensure persistent writes to disk.
type TransactionManager struct {
	db        *DB
	update    bool
	locks     []*record.ID
	txupdates map[string]keyval
}

func prefixkey(prefix byte, key []byte) []byte {
	k := make([]byte, core.RecordRefSize+1)
	k[0] = prefix
	_ = copy(k[1:], key)
	return k
}

func (m *TransactionManager) lockOnID(id *record.ID) {
	m.db.idlocker.Lock(id)
	m.locks = append(m.locks, id)
}

func (m *TransactionManager) releaseLocks() {
	for _, id := range m.locks {
		m.db.idlocker.Unlock(id)
	}
}

// Commit tries to write transaction on disk. Returns error on fail.
func (m *TransactionManager) Commit() error {
	if len(m.txupdates) == 0 {
		return nil
	}
	var err error
	tx := m.db.db.NewTransaction(m.update)
	defer tx.Discard()
	for _, rec := range m.txupdates {
		err = tx.Set(rec.k, rec.v)
		if err != nil {
			break
		}
	}
	m.txupdates = nil
	if err != nil {
		return err
	}
	return tx.Commit(nil)
}

// Discard terminates transaction without disk writes.
func (m *TransactionManager) Discard() {
	m.releaseLocks()
	if m.update {
		m.db.dropWG.Done()
	}
}

// GetRequest returns request record from BadgerDB by *record.Reference.
//
// It returns ErrNotFound if the DB does not contain the key.
func (m *TransactionManager) GetRequest(id *record.ID) (record.Request, error) {
	rec, err := m.GetRecord(id)
	if err != nil {
		return nil, err
	}
	// TODO: return error if record is not a request.
	req := rec.(record.Request)
	return req, nil
}

// SetRequest stores request record in BadgerDB and returns *record.ID of new record.
//
// If record exists SetRequest just returns *record.ID without error.
func (m *TransactionManager) SetRequest(req record.Request) (*record.ID, error) {
	log.Debugf("SetRequest call")
	id, err := m.SetRecord(req)
	if err != nil && err != ErrOverride {
		return nil, err
	}
	return id, nil
}

func (m *TransactionManager) set(key, val []byte) {
	m.txupdates[string(key)] = keyval{k: key, v: val}
}

// GetRecord returns record from BadgerDB by *record.Reference.
//
// It returns ErrNotFound if the DB does not contain the key.
func (m *TransactionManager) GetRecord(id *record.ID) (record.Record, error) {
	k := prefixkey(scopeIDRecord, record.ID2Bytes(*id))
	log.Debugf("GetRecord by id %+v (key=%x)", id, k)
	buf, err := m.Get(k)
	if err != nil {
		return nil, err
	}
	raw, err := record.DecodeToRaw(buf)
	if err != nil {
		return nil, err
	}
	return raw.ToRecord(), nil
}

// SetRecord stores record in BadgerDB and returns *record.ID of new record.
//
// If record exists returns both *record.ID and ErrOverride error.
// If record not found returns nil and ErrNotFound error
func (m *TransactionManager) SetRecord(rec record.Record) (*record.ID, error) {
	raw, err := record.EncodeToRaw(rec)
	if err != nil {
		return nil, err
	}

	var h []byte
	if req, ok := rec.(record.Request); ok {
		// we should calculate request hashes consistently with logicrunner.
		h = hash.IDHashBytes(req.GetPayload())
	} else {
		h = raw.Hash()
	}
	id := record.ID{
		Pulse: m.db.GetCurrentPulse(),
		Hash:  h,
	}
	k := prefixkey(scopeIDRecord, record.ID2Bytes(id))
	geterr := m.db.db.View(func(tx *badger.Txn) error {
		_, err := tx.Get(k)
		return err
	})
	if geterr == nil {
		return &id, ErrOverride
	}
	if geterr != badger.ErrKeyNotFound {
		return nil, ErrNotFound
	}

	m.set(k, record.MustEncodeRaw(raw))
	return &id, nil
}

// GetClassIndex fetches class lifeline's index.
func (m *TransactionManager) GetClassIndex(id *record.ID, forupdate bool) (*index.ClassLifeline, error) {
	if forupdate {
		m.lockOnID(id)
	}
	k := prefixkey(scopeIDLifeline, record.ID2Bytes(*id))
	buf, err := m.Get(k)
	if err != nil {
		return nil, err
	}
	return index.DecodeClassLifeline(buf)
}

// SetClassIndex stores class lifeline index.
func (m *TransactionManager) SetClassIndex(id *record.ID, idx *index.ClassLifeline) error {
	k := prefixkey(scopeIDLifeline, record.ID2Bytes(*id))
	encoded, err := index.EncodeClassLifeline(idx)
	if err != nil {
		return err
	}
	m.set(k, encoded)
	return nil
}

// GetObjectIndex fetches object lifeline index.
func (m *TransactionManager) GetObjectIndex(id *record.ID, forupdate bool) (*index.ObjectLifeline, error) {
	if forupdate {
		m.lockOnID(id)
	}
	k := prefixkey(scopeIDLifeline, record.ID2Bytes(*id))
	buf, err := m.Get(k)
	if err != nil {
		return nil, err
	}
	return index.DecodeObjectLifeline(buf)
}

// SetObjectIndex stores object lifeline index.
func (m *TransactionManager) SetObjectIndex(id *record.ID, idx *index.ObjectLifeline) error {
	k := prefixkey(scopeIDLifeline, record.ID2Bytes(*id))
	if idx.Delegates == nil {
		idx.Delegates = map[core.RecordRef]record.Reference{}
	}
	encoded, err := index.EncodeObjectLifeline(idx)
	if err != nil {
		return err
	}
	m.set(k, encoded)
	return nil
}

// GetEntropy returns entropy from storage for given pulse.
//
// GeneratedEntropy is used for calculating node roles.
func (m *TransactionManager) GetEntropy(pulse core.PulseNumber) (*core.Entropy, error) {
	k := prefixkey(scopeIDEntropy, pulse.Bytes())
	buf, err := m.Get(k)
	if err != nil {
		return nil, err
	}
	var entropy core.Entropy
	copy(entropy[:], buf)
	return &entropy, nil
}

// SetEntropy stores given entropy for given pulse in storage.
//
// GeneratedEntropy is used for calculating node roles.
func (m *TransactionManager) SetEntropy(pulse core.PulseNumber, entropy core.Entropy) error {
	k := prefixkey(scopeIDEntropy, pulse.Bytes())
	m.set(k, entropy[:])
	return nil
}

// Get returns value by key.
func (m *TransactionManager) Get(key []byte) ([]byte, error) {
	if kv, ok := m.txupdates[string(key)]; ok {
		return kv.v, nil
	}

	txn := m.db.db.NewTransaction(false)
	defer txn.Discard()
	item, err := txn.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return item.ValueCopy(nil)
}

// Set stores value by key.
func (m *TransactionManager) Set(key, value []byte) error {
	m.set(key, value)
	return nil
}
