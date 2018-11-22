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

package configuration

import (
	"github.com/insolar/insolar/core"
)

// Storage configures Ledger's storage.
type Storage struct {
	// DataDirectory is a directory where database's files live.
	DataDirectory string
	// TxRetriesOnConflict defines how many retries on transaction conflicts
	// storage update methods should do.
	TxRetriesOnConflict int
}

// JetCoordinator holds configuration for JetCoordinator.
type JetCoordinator struct {
	RoleCounts map[int]int
}

// ArtifactManager holds configuration for ArtifactManager.
type ArtifactManager struct {
	// Maximum pulse difference (NOT number of pulses) between current and the latest replicated on heavy.
	LightChainLimit core.PulseNumber
}

// Ledger holds configuration for ledger.
type Ledger struct {
	// Storage defines storage configuration.
	Storage Storage
	// JetCoordinator defines jet coordinator configuration.
	JetCoordinator JetCoordinator
	// HeavyReplication defines replication to heavy storage node.
	HeavyReplication HeavyReplication
	// ArtifactManager holds configuration for ArtifactManager.
	ArtifactManager ArtifactManager
}

// HeavyReplication configures replication to heavy node
type HeavyReplication struct {
	SyncMessageLimit int
}

// NewLedger creates new default Ledger configuration.
func NewLedger() Ledger {
	return Ledger{
		Storage: Storage{
			DataDirectory:       "./data",
			TxRetriesOnConflict: 3,
		},

		JetCoordinator: JetCoordinator{
			RoleCounts: map[int]int{
				int(core.RoleVirtualExecutor):  1,
				int(core.RoleHeavyExecutor):    1,
				int(core.RoleLightExecutor):    1,
				int(core.RoleVirtualValidator): 1,
				int(core.RoleLightValidator):   1,
			},
		},

		ArtifactManager: ArtifactManager{
			LightChainLimit: 10 * 30, // 30 pulses
		},
	}
}
