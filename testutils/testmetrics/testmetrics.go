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

package testmetrics

import (
	"context"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/metrics"
)

// TestMetrics provides testing helpers for metrics.
type TestMetrics struct {
	ctx     context.Context
	Metrics *metrics.Metrics
}

var (
	oncecfg sync.Once
	cfg     configuration.Metrics
)

// Start configures, creates and starts metrics server,
// returns initialized TestMetrics object.
func Start(ctx context.Context) TestMetrics {
	inslog := inslogger.FromContext(ctx)
	oncecfg.Do(func() {
		cfg = configuration.NewMetrics()
	})
	host, _ := parseAddr(cfg.ListenAddress)

	// just use any available port
	cfg.ListenAddress = host + ":0"

	m, err := metrics.NewMetrics(ctx, cfg)
	if err != nil {
		panic(err)
	}

	err = m.Start(ctx)
	if err != nil {
		inslog.Fatal("metrics server failed to start:", err)
	}

	return TestMetrics{
		ctx:     ctx,
		Metrics: m,
	}
}

func parseAddr(address string) (string, int32) {
	pair := strings.SplitN(address, ":", 2)
	currentPort, err := strconv.Atoi(pair[1])
	if err != nil {
		panic(err)
	}
	return pair[0], int32(currentPort)
}

// FetchContent fetches content from metrics server, returns stringifyed content.
func (tm TestMetrics) FetchContent() (string, error) {
	fetchurl := "http://" + tm.Metrics.AddrString() + "/metrics"
	// fmt.Println("Fetch:", fetchurl)
	response, err := http.Get(fetchurl)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	content, err := ioutil.ReadAll(response.Body)
	return string(content), err
}

// Stop wraps metrics Stop method.
func (tm TestMetrics) Stop() error {
	return tm.Metrics.Stop(tm.ctx)
}
