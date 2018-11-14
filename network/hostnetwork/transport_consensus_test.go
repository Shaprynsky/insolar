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

package hostnetwork

import (
	"sync"
	"testing"
	"time"

	"github.com/insolar/insolar/consensus/packets"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/transport/host"
	"github.com/insolar/insolar/network/transport/packet/types"
	"github.com/stretchr/testify/assert"
)

func createTwoConsensusNetworks(id1, id2 core.ShortNodeID) (t1, t2 network.ConsensusNetwork, err error) {
	m := newMockResolver()

	cn1, err := NewConsensusNetwork("127.0.0.1:0", ID1, id1, m)
	if err != nil {
		return nil, nil, err
	}
	cn2, err := NewConsensusNetwork("127.0.0.1:0", ID2, id2, m)
	if err != nil {
		return nil, nil, err
	}

	routing1, err := host.NewHostNS(cn1.PublicAddress(), core.NewRefFromBase58(ID1), id1)
	if err != nil {
		return nil, nil, err
	}
	routing2, err := host.NewHostNS(cn2.PublicAddress(), core.NewRefFromBase58(ID2), id2)
	if err != nil {
		return nil, nil, err
	}
	m.addMappingHost(routing1)
	m.addMappingHost(routing2)

	return cn1, cn2, nil
}

func TestTransportConsensus_SendRequest(t *testing.T) {
	cn1, cn2, err := createTwoConsensusNetworks(0, 1)
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)

	handler := func(r network.Request) {
		log.Info("handler triggered")
		wg.Done()
	}
	cn2.RegisterRequestHandler(types.Phase1, handler)

	cn2.Start()
	cn1.Start()
	defer func() {
		cn1.Stop()
		cn2.Stop()
	}()

	request := cn1.NewRequestBuilder().Type(types.Phase1).Data(&packets.Phase1Packet{}).Build()
	err = cn1.SendRequest(request, cn2.GetNodeID())
	assert.NoError(t, err)
	success := network.WaitTimeout(&wg, time.Second)
	assert.True(t, success)
}

func TestTransportConsensus_RegisterPacketHandler(t *testing.T) {
	m := newMockResolver()

	cn, err := NewConsensusNetwork("127.0.0.1:0", ID1, 0, m)
	assert.NoError(t, err)
	defer cn.Stop()
	handler := func(request network.Request) {
		// do nothing
	}
	f := func() {
		cn.RegisterRequestHandler(types.Phase1, handler)
	}
	assert.NotPanics(t, f, "first request handler register should not panic")
	assert.Panics(t, f, "second request handler register should panic because it is already registered")
}
