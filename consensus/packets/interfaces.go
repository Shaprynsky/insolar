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
	"errors"
	"io"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/network/transport/packet/types"
)

type RoutingHeader struct {
	OriginID   core.ShortNodeID
	TargetID   core.ShortNodeID
	PacketType types.PacketType
}

type PacketRoutable interface {
	// SetPacketHeader set routing information for transport level.
	SetPacketHeader(header *RoutingHeader) error
	// GetPacketHeader get routing information from transport level.
	GetPacketHeader() (*RoutingHeader, error)
}

type Serializer interface {
	Serialize() ([]byte, error)
	Deserialize(data io.Reader) error
}

type ConsensusPacket interface {
	Serializer
	PacketRoutable
}

func ExtractPacket(reader io.Reader) (ConsensusPacket, error) {
	return nil, errors.New("not implemented")
}
