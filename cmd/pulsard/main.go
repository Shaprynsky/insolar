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
	"fmt"
	"net"
	"time"

	"github.com/chzyer/readline"
	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/cryptography"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/keystore"
	"github.com/insolar/insolar/platformpolicy"
	"github.com/insolar/insolar/pulsar"
	"github.com/insolar/insolar/pulsar/entropygenerator"
	"github.com/insolar/insolar/pulsar/storage"
	"github.com/insolar/insolar/version"
	"github.com/satori/go.uuid"
	jww "github.com/spf13/jwalterweatherman"
)

// Need to fix problem with start pulsar
func main() {
	uniqueID := RandTraceID()
	ctx, inslog := inslogger.WithTraceField(context.Background(), uniqueID)

	jww.SetStdoutThreshold(jww.LevelDebug)
	cfgHolder := configuration.NewHolder()
	err := cfgHolder.Load()
	if err != nil {
		inslog.Warnln("failed to load configuration from file: ", err.Error())
	}

	err = cfgHolder.LoadEnv()
	if err != nil {
		inslog.Warnln("failed to load configuration from env:", err.Error())
	}

	server, storage := initPulsar(ctx, cfgHolder.Configuration)
	server.ID = uniqueID

	go server.StartServer(ctx)
	pulseTicker, refreshTicker := runPulsar(ctx, server, cfgHolder.Configuration.Pulsar)

	fmt.Println("Press any button to exit")
	rl, err := readline.New("")
	if err != nil {
		inslog.Error(err)
		panic(err)
	}
	_, err = rl.Readline()
	if err != nil {
		inslog.Warn(err)
	}

	defer func() {
		pulseTicker.Stop()
		refreshTicker.Stop()
		err := storage.Close()
		if err != nil {
			inslog.Error(err)
		}
		server.StopServer(ctx)
	}()

}

func initPulsar(ctx context.Context, cfg configuration.Configuration) (*pulsar.Pulsar, pulsarstorage.PulsarStorage) {
	fmt.Print("Starts with configuration:\n", configuration.ToString(cfg))
	fmt.Println("Version: ", version.GetFullVersion())

	keyStore, err := keystore.NewKeyStore(cfg.KeysPath)
	if err != nil {
		inslogger.FromContext(ctx).Fatal(err)
		panic(err)
	}
	cryptographyScheme := platformpolicy.NewPlatformCryptographyScheme()
	cryptographyService := cryptography.NewCryptographyService()

	cm := &component.Manager{}
	cm.Register(cryptographyScheme, keyStore)
	cm.Inject(cryptographyService)

	storage, err := pulsarstorage.NewStorageBadger(cfg.Pulsar, nil)
	if err != nil {
		inslogger.FromContext(ctx).Fatal(err)
		panic(err)
	}
	switcher := &pulsar.StateSwitcherImpl{}
	server, err := pulsar.NewPulsar(
		cfg.Pulsar,
		cryptographyService,
		cryptographyScheme,
		platformpolicy.NewKeyProcessor(),
		storage,
		&pulsar.RPCClientWrapperFactoryImpl{},
		&entropygenerator.StandardEntropyGenerator{},
		switcher,
		net.Listen,
	)

	if err != nil {
		inslogger.FromContext(ctx).Fatal(err)
		panic(err)
	}
	switcher.SetPulsar(server)

	return server, storage
}

func runPulsar(ctx context.Context, server *pulsar.Pulsar, cfg configuration.Pulsar) (pulseTicker *time.Ticker, refreshTicker *time.Ticker) {
	server.CheckConnectionsToPulsars(ctx)

	var nextPulseNumber core.PulseNumber
	if server.GetLastPulse().PulseNumber == core.GenesisPulse.PulseNumber {
		nextPulseNumber = core.CalculatePulseNumber(time.Now())
	} else {
		nextPulseNumber = server.GetLastPulse().PulseNumber + core.PulseNumber(cfg.NumberDelta)
	}

	err := server.StartConsensusProcess(ctx, nextPulseNumber)
	if err != nil {
		inslogger.FromContext(ctx).Fatal(err)
		panic(err)
	}
	pulseTicker = time.NewTicker(time.Duration(cfg.PulseTime) * time.Millisecond)
	go func() {
		for range pulseTicker.C {
			err = server.StartConsensusProcess(ctx, core.PulseNumber(server.GetLastPulse().PulseNumber+core.PulseNumber(cfg.NumberDelta)))
			if err != nil {
				inslogger.FromContext(ctx).Fatal(err)
				panic(err)
			}
		}
	}()

	refreshTicker = time.NewTicker(1 * time.Second)
	go func() {
		for range refreshTicker.C {
			server.CheckConnectionsToPulsars(ctx)
		}
	}()

	return
}

// RandTraceID returns random traceID in uuid format
func RandTraceID() string {
	qid, err := uuid.NewV4()
	if err != nil {
		panic("createRandomTraceIDFailed:" + err.Error())
	}
	return qid.String()
}
