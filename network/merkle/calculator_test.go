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

package merkle

import (
	"context"
	"testing"

	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/ledger/ledgertestutils"
	"github.com/insolar/insolar/testutils/certificate"
	"github.com/insolar/insolar/testutils/nodekeeper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type calculatorSuite struct {
	suite.Suite

	pulseManager core.PulseManager
	nodeNetwork  core.NodeNetwork

	calculator Calculator
}

func (t *calculatorSuite) TestGetNodeProof() {
	pulse, err := t.pulseManager.Current(context.Background())
	t.Assert().NoError(err)

	ph, np, err := t.calculator.GetPulseProof(context.Background(), &PulseEntry{Pulse: pulse})

	t.Assert().NoError(err)
	t.Assert().NotNil(np)

	valid := np.IsValid(context.Background(), t.nodeNetwork.GetOrigin(), ph)
	t.Assert().True(valid)
}

func (t *calculatorSuite) TestGetGlobuleProof() {
	// gp, err := t.calculator.GetGlobuleProof(context.Background())
	//
	// t.Assert().NoError(err)
	// t.Assert().NotNil(gp)
	//
	// globuleHash, err := t.calculator.GetGlobuleHash(context.Background(), t.nodeNetwork.GetActiveNodes())
	// t.Assert().NoError(err)
	//
	// valid := gp.IsValid(context.Background(), t.nodeNetwork.GetOrigin(), globuleHash)
	// t.Assert().True(valid)
}

func (t *calculatorSuite) TestGetCloudProof() {
	// cp, err := t.calculator.GetCloudProof(context.Background())
	//
	// t.Assert().NoError(err)
	// t.Assert().NotNil(cp)
	//
	// cloudHash, err := t.calculator.GetCloudHash(context.Background())
	// t.Assert().NoError(err)
	//
	// valid := cp.IsValid(context.Background(), t.nodeNetwork.GetOrigin(), cloudHash)
	// t.Assert().True(valid)
}

func TestCalculator(t *testing.T) {
	c := certificate.GetTestCertificate()
	nk := nodekeeper.GetTestNodekeeper(c)
	// FIXME: TmpLedger is deprecated. Use mocks instead.
	l, clean := ledgertestutils.TmpLedger(t, "", core.Components{})

	calculator := &calculator{}

	cm := component.Manager{}
	cm.Register(nk, l, c, calculator)

	assert.NotNil(t, calculator.Ledger)
	assert.NotNil(t, calculator.NodeNetwork)
	assert.NotNil(t, calculator.Certificate)

	s := &calculatorSuite{
		Suite:        suite.Suite{},
		calculator:   calculator,
		pulseManager: l.GetPulseManager(),
		nodeNetwork:  nk,
	}
	suite.Run(t, s)

	clean()
}
