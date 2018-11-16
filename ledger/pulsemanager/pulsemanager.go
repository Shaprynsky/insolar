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

package pulsemanager

import (
	"context"
	"sync"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/ledger/jetdrop"
	"github.com/insolar/insolar/ledger/storage"
)

// PulseManager implements core.PulseManager.
type PulseManager struct {
	db      *storage.DB
	LR      core.LogicRunner `inject:""`
	Bus     core.MessageBus  `inject:""`
	NodeNet core.NodeNetwork `inject:""`

	setLock sync.Mutex
}

// Current returns current pulse structure.
func (m *PulseManager) Current(ctx context.Context) (*core.Pulse, error) {
	latestPulse, err := m.db.GetLatestPulseNumber(ctx)
	if err != nil {
		return nil, err
	}
	pulse, err := m.db.GetPulse(ctx, latestPulse)
	if err != nil {
		return nil, err
	}
	return &pulse.Pulse, nil
}

func (m *PulseManager) processDrop(ctx context.Context) error {
	latestPulseNumber, err := m.db.GetLatestPulseNumber(ctx)
	if err != nil {
		return err
	}
	latestPulse, err := m.db.GetPulse(ctx, latestPulseNumber)
	if err != nil {
		return err
	}
	prevDrop, err := m.db.GetDrop(ctx, latestPulse.Prev)
	if err != nil {
		return err
	}
	drop, messages, err := m.db.CreateDrop(ctx, latestPulseNumber, prevDrop.Hash)
	if err != nil {
		return err
	}
	err = m.db.SetDrop(ctx, drop)
	if err != nil {
		return err
	}

	dropSerialized, err := jetdrop.Encode(drop)
	if err != nil {
		return err
	}

	msg := &message.JetDrop{
		Drop:        dropSerialized,
		Messages:    messages,
		PulseNumber: latestPulseNumber,
	}
	_, err = m.Bus.Send(ctx, msg)
	if err != nil {
		return err
	}
	return nil
}

// Set set's new pulse and closes current jet drop.
func (m *PulseManager) Set(ctx context.Context, pulse core.Pulse) error {
	// Ensure this does not execute in parallel.
	m.setLock.Lock()
	defer m.setLock.Unlock()

	// Run only on material executor.
	if m.NodeNet.GetOrigin().Role() == core.RoleLightMaterial {
		err := m.processDrop(ctx)
		if err != nil {
			return err
		}
	}

	err := m.db.AddPulse(ctx, pulse)
	if err != nil {
		return err
	}

	err = m.db.SetActiveNodes(pulse.PulseNumber, m.NodeNet.GetActiveNodes())
	if err != nil {
		return err
	}

	return m.LR.OnPulse(ctx, pulse)
}

// NewPulseManager creates PulseManager instance.
func NewPulseManager(db *storage.DB) *PulseManager {
	return &PulseManager{db: db}
}
