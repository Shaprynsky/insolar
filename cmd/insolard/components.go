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

package main

import (
	"context"

	"github.com/insolar/insolar/api"
	"github.com/insolar/insolar/certificate"
	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/genesis"
	"github.com/insolar/insolar/ledger"
	"github.com/insolar/insolar/logicrunner"
	"github.com/insolar/insolar/messagebus"
	"github.com/insolar/insolar/metrics"
	"github.com/insolar/insolar/network/nodenetwork"
	"github.com/insolar/insolar/network/servicenetwork"
	"github.com/insolar/insolar/networkcoordinator"
	"github.com/insolar/insolar/pulsar"
	"github.com/insolar/insolar/pulsar/entropygenerator"
	"github.com/insolar/insolar/version/manager"
)

// InitComponents creates and links all insolard components
func InitComponents(ctx context.Context, cfg configuration.Configuration, isBootstrap bool, nodeKeysPath string) (*component.Manager, *ComponentManager, *Repl, error) {
	var cert *certificate.Certificate
	var err error
	if isBootstrap {
		cert, err = certificate.NewCertificatesWithKeys(cfg.KeysPath)
		checkError(ctx, err, "failed to start Certificate (bootstrap mode)")
	} else {
		cert, err = certificate.NewCertificate(cfg.KeysPath, cfg.CertificatePath)
		checkError(ctx, err, "failed to start Certificate")
	}

	nodeNetwork, err := nodenetwork.NewNodeNetwork(cfg)
	checkError(ctx, err, "failed to start NodeNetwork")

	logicRunner, err := logicrunner.NewLogicRunner(&cfg.LogicRunner)
	checkError(ctx, err, "failed to start LogicRunner")

	ledger, err := ledger.NewLedger(ctx, cfg.Ledger)
	checkError(ctx, err, "failed to start Ledger")

	nw, err := servicenetwork.NewServiceNetwork(cfg)
	checkError(ctx, err, "failed to start Network")

	messageBus, err := messagebus.NewMessageBus(cfg)
	checkError(ctx, err, "failed to start MessageBus")

	var gen core.Genesis
	if isBootstrap {
		gen, err = genesis.NewGenesis(cfg.Genesis, isBootstrap, bootstrapNodesInfo(ctx, nodeKeysPath))
		checkError(ctx, err, "failed to start Bootstrapper (bootstrap mode)")
	} else {
		gen, err = genesis.NewGenesis(cfg.Genesis, isBootstrap, []map[string]string{})
		checkError(ctx, err, "failed to start Bootstrapper")
	}

	apiRunner, err := api.NewRunner(&cfg.APIRunner)
	checkError(ctx, err, "failed to start ApiRunner")

	metricsHandler, err := metrics.NewMetrics(ctx, cfg.Metrics)
	checkError(ctx, err, "failed to start Metrics")

	networkCoordinator, err := networkcoordinator.New()
	checkError(ctx, err, "failed to start NetworkCoordinator")

	versionManager, err := manager.NewVersionManager(cfg.VersionManager)
	checkError(ctx, err, "failed to load VersionManager: ")

	// move to logic runner ??
	err = logicRunner.OnPulse(ctx, *pulsar.NewPulse(cfg.Pulsar.NumberDelta, 0, &entropygenerator.StandardEntropyGenerator{}))
	checkError(ctx, err, "failed init pulse for LogicRunner")

	cm := component.Manager{}
	cm.Register(
		cert,
		nodeNetwork,
		logicRunner,
		ledger,
		nw,
		messageBus,
		gen,
		apiRunner,
		metricsHandler,
		networkCoordinator,
		versionManager,
	)

	cmOld := ComponentManager{components: core.Components{
		Certificate:        cert,
		NodeNetwork:        nodeNetwork,
		LogicRunner:        logicRunner,
		Ledger:             ledger,
		Network:            nw,
		MessageBus:         messageBus,
		Genesis:            gen,
		APIRunner:          apiRunner,
		NetworkCoordinator: networkCoordinator,
		VersionManager:     versionManager,
	}}

	return &cm, &cmOld, &Repl{Manager: ledger.GetPulseManager(), NodeNetwork: nodeNetwork}, nil
}
