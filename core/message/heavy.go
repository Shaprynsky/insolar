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

package message

import (
	"github.com/insolar/insolar/core"
)

// HeavyPayload carries Key/Value records for replication to Heavy Material node.
type HeavyPayload struct {
	Records []core.KV
}

// GetCaller implementation of Message interface.
func (HeavyPayload) GetCaller() *core.RecordRef {
	return nil
}

// Type implementation of Message interface.
func (e *HeavyPayload) Type() core.MessageType {
	return core.TypeHeavyPayload
}

// HeavyStartStop carries heavy replication start/stop signal with pulse range.
type HeavyStartStop struct {
	Begin    core.PulseNumber
	End      core.PulseNumber
	Finished bool
}

// GetCaller implementation of Message interface.
func (HeavyStartStop) GetCaller() *core.RecordRef {
	return nil
}

// Type implementation of Message interface.
func (e *HeavyStartStop) Type() core.MessageType {
	return core.TypeHeavyStartStop
}
