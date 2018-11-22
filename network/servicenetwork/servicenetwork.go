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

package servicenetwork

import (
	"context"
	"strconv"

	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/consensus/phases"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/controller"
	"github.com/insolar/insolar/network/fakepulsar"
	"github.com/insolar/insolar/network/hostnetwork"
	"github.com/insolar/insolar/network/routing"
	"github.com/pkg/errors"
)

// ServiceNetwork is facade for network.
type ServiceNetwork struct {
	cfg    configuration.Configuration
	scheme core.PlatformCryptographyScheme

	hostNetwork  network.HostNetwork  // TODO: should be injected
	controller   network.Controller   // TODO: should be injected
	routingTable network.RoutingTable // TODO: should be injected

	Certificate         core.Certificate         `inject:""`
	NodeNetwork         core.NodeNetwork         `inject:""`
	PulseManager        core.PulseManager        `inject:""`
	PhaseManager        phases.PhaseManager      `inject:""`
	CryptographyService core.CryptographyService `inject:""`
	NetworkCoordinator  core.NetworkCoordinator  `inject:""`
	NodeKeeper          network.NodeKeeper       `inject:""`

	fakePulsar *fakepulsar.FakePulsar
}

// NewServiceNetwork returns a new ServiceNetwork.
func NewServiceNetwork(conf configuration.Configuration, scheme core.PlatformCryptographyScheme) (*ServiceNetwork, error) {
	serviceNetwork := &ServiceNetwork{cfg: conf, scheme: scheme}
	return serviceNetwork, nil
}

// GetAddress returns host public address.
func (n *ServiceNetwork) GetAddress() string {
	return n.hostNetwork.PublicAddress()
}

// GetNodeID returns current node id.
func (n *ServiceNetwork) GetNodeID() core.RecordRef {
	return n.NodeNetwork.GetOrigin().ID()
}

// GetGlobuleID returns current globule id.
func (n *ServiceNetwork) GetGlobuleID() core.GlobuleID {
	return 0
}

// SendMessage sends a message from MessageBus.
func (n *ServiceNetwork) SendMessage(nodeID core.RecordRef, method string, msg core.Parcel) ([]byte, error) {
	return n.controller.SendMessage(nodeID, method, msg)
}

// SendCascadeMessage sends a message from MessageBus to a cascade of nodes
func (n *ServiceNetwork) SendCascadeMessage(data core.Cascade, method string, msg core.Parcel) error {
	return n.controller.SendCascadeMessage(data, method, msg)
}

// RemoteProcedureRegister registers procedure for remote call on this host.
func (n *ServiceNetwork) RemoteProcedureRegister(name string, method core.RemoteProcedure) {
	n.controller.RemoteProcedureRegister(name, method)
}

// Start implements component.Initer
func (n *ServiceNetwork) Init(ctx context.Context) error {
	routingTable, hostnetwork, controller, err := newNetworkComponents(n.cfg, n, n.scheme)
	if err != nil {
		return errors.Wrap(err, "Failed to create network components.")
	}
	n.fakePulsar = fakepulsar.NewFakePulsar(n.HandlePulse, n.cfg.Pulsar.PulseTime)
	n.routingTable = routingTable
	n.hostNetwork = hostnetwork
	n.controller = controller
	return nil
}

// Start implements component.Starter
func (n *ServiceNetwork) Start(ctx context.Context) error {
	log.Infoln("Network starts listening...")
	n.hostNetwork.Start(ctx)

	n.controller.Inject(n.CryptographyService, n.NetworkCoordinator, n.NodeKeeper)
	n.routingTable.Inject(n.NodeKeeper)

	log.Infoln("Bootstrapping network...")
	err := n.controller.Bootstrap(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to bootstrap network")
	}

	log.Infoln("Authorizing network...")
	err = n.controller.Authorize(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to authorize network")
	}

	n.fakePulsar.Start(ctx)

	return nil
}

// Stop implements core.Component
func (n *ServiceNetwork) Stop(ctx context.Context) error {
	n.hostNetwork.Stop()
	return nil
}

func (n *ServiceNetwork) HandlePulse(ctx context.Context, pulse core.Pulse) {
	if !n.isFakePulse(&pulse) {
		n.fakePulsar.Stop(ctx)
	}
	traceID := "pulse_" + strconv.FormatUint(uint64(pulse.PulseNumber), 10)
	ctx, logger := inslogger.WithTraceField(ctx, traceID)
	logger.Infof("Got new pulse number: %d", pulse.PulseNumber)
	if n.PulseManager == nil {
		logger.Error("PulseManager is not initialized")
		return
	}
	currentPulse, err := n.PulseManager.Current(ctx)
	if err != nil {
		logger.Error(errors.Wrap(err, "Could not get current pulse"))
		return
	}
	if (pulse.PulseNumber > currentPulse.PulseNumber) &&
		(pulse.PulseNumber >= currentPulse.NextPulseNumber) {
		err = n.PulseManager.Set(ctx, pulse)
		if err != nil {
			logger.Error(errors.Wrap(err, "Failed to set pulse"))
			return
		}
		logger.Infof("Set new current pulse number: %d", pulse.PulseNumber)
		go func(logger core.Logger, network *ServiceNetwork) {
			// FIXME: we need to resend pulse only to nodes outside the globe, we send pulse to nodes inside the globe on phase1 of the consensus
			// network.controller.ResendPulseToKnownHosts(pulse)
			if network.NetworkCoordinator == nil {
				return
			}
			err := network.NetworkCoordinator.WriteActiveNodes(ctx, pulse.PulseNumber, network.NodeNetwork.GetActiveNodes())
			if err != nil {
				logger.Warn("Error writing active nodes to ledger: " + err.Error())
			}
			err = n.PhaseManager.OnPulse(ctx, &pulse)
			if err != nil {
				logger.Warn("phase manager fail: " + err.Error())
			}
		}(logger, n)

		// TODO: PLACE NEW CONSENSUS HERE
	} else {
		logger.Infof("Incorrect pulse number. Current: %d. New: %d", currentPulse.PulseNumber, pulse.PulseNumber)
	}
}

func (n *ServiceNetwork) isFakePulse(pulse *core.Pulse) bool {
	return (pulse.NextPulseNumber == 0) && (pulse.PulseNumber == 0)
}

// newNetworkComponents create network.HostNetwork and network.Controller for new network
func newNetworkComponents(conf configuration.Configuration,
	pulseHandler network.PulseHandler, scheme core.PlatformCryptographyScheme) (network.RoutingTable, network.HostNetwork, network.Controller, error) {
	routingTable := &routing.Table{}
	internalTransport, err := hostnetwork.NewInternalTransport(conf)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "error creating internal transport")
	}
	hostNetwork := hostnetwork.NewHostTransport(internalTransport, routingTable)
	options := controller.ConfigureOptions(conf.Host)
	networkController := controller.NewNetworkController(pulseHandler, options, internalTransport, routingTable, hostNetwork, scheme)
	return routingTable, hostNetwork, networkController, nil
}
