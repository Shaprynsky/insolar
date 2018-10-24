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

package dhtnetwork

import (
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/cascade"
	"github.com/insolar/insolar/network/dhtnetwork/consensus"
	"github.com/insolar/insolar/network/dhtnetwork/hosthandler"
	"github.com/insolar/insolar/network/dhtnetwork/resolver"
	"github.com/insolar/insolar/network/dhtnetwork/rpc"
	"github.com/insolar/insolar/network/dhtnetwork/signhandler"
	"github.com/insolar/insolar/network/dhtnetwork/store"
	"github.com/insolar/insolar/network/transport"
	"github.com/insolar/insolar/network/transport/host"
	"github.com/insolar/insolar/network/transport/id"

	"github.com/insolar/insolar/network/transport/relay"
	"github.com/pkg/errors"
)

// NewHostNetwork creates and returns DHT network.
func NewHostNetwork(
	cfg configuration.Configuration,
	cascade *cascade.Cascade,
	certificate core.Certificate,
	pulseCallback network.OnPulse,
) (*DHT, error) {

	proxy := relay.NewProxy()

	tp, err := transport.NewTransport(cfg.Host.Transport, proxy)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create transport")
	}

	originAddress, err := host.NewAddress(tp.PublicAddress())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to ")
	}

	nodeID := core.NewRefFromBase58(cfg.Node.Node.ID)
	encodedOriginID := resolver.ResolveHostID(nodeID)
	originID := id.FromBase58(encodedOriginID)
	origin, err := host.NewOrigin([]id.ID{originID}, originAddress)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Origin")
	}

	options := &Options{BootstrapHosts: getBootstrapHosts(cfg.Host.BootstrapHosts)}
	sign := signhandler.NewSignHandler(certificate)
	ncf := hosthandler.NewNetworkCommonFacade(rpc.NewRPCFactory(nil).Create(), cascade, sign, pulseCallback)

	network, err := NewDHT(
		store.NewMemoryStoreFactory().Create(),
		origin,
		tp,
		ncf,
		options,
		proxy,
		cfg.Host.Timeout,
		cfg.Host.InfinityBootstrap,
		nodeID,
		cfg.Host.MajorityRule,
		certificate,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create DHT")
	}
	networkConsensus := consensus.NewInsolarConsensus(network)
	network.GetNetworkCommonFacade().SetConsensus(networkConsensus)
	return network, nil
}

func getBootstrapHosts(addresses []string) []*host.Host {
	var hosts []*host.Host
	for _, a := range addresses {
		address, err := host.NewAddress(a)
		if err != nil {
			log.Errorln("Failed to create bootstrap address:", err.Error())
		}
		hosts = append(hosts, host.NewHost(address))
	}
	return hosts
}
