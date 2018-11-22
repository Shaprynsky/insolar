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

package messagebus

import (
	"context"
	"testing"

	"github.com/gojuno/minimock"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/platformpolicy"
	"github.com/insolar/insolar/testutils"
	"github.com/stretchr/testify/require"
)

func TestRecorder_Send(t *testing.T) {
	mc := minimock.NewController(t)
	defer mc.Finish()

	pcs := platformpolicy.NewPlatformCryptographyScheme()

	ctx := inslogger.TestContext(t)
	msg := message.GenesisRequest{Name: "test"}
	parcel := message.Parcel{Msg: &msg}
	msgHash := GetMessageHash(pcs, &parcel)
	expectedRep := reply.Object{Memory: []byte{1, 2, 3}}
	pm := testutils.NewPulseManagerMock(mc)
	s := NewsenderMock(mc)
	s.CreateParcelFunc = func(p context.Context, p2 core.Message, p3 *core.SendOptions) (r core.Parcel, r1 error) {
		return &message.Parcel{Msg: p2}, nil
	}

	tape := NewtapeMock(mc)
	recorder := newRecorder(s, tape, pm, pcs)

	t.Run("with no reply on the tape sends the message and returns reply", func(t *testing.T) {
		tape.GetReplyMock.Expect(ctx, msgHash).Return(&expectedRep, nil)
		s.SendParcelMock.Expect(ctx, &parcel, nil)

		_, err := recorder.Send(ctx, &msg)
		require.NoError(t, err)
	})

	t.Run("with reply on the tape doesn't send the message and returns reply from the tape", func(t *testing.T) {
		tape.GetReplyMock.Expect(ctx, msgHash).Return(&expectedRep, nil)
		s.SendParcelMock.Set(nil)

		_, err := recorder.Send(ctx, &msg)
		require.NoError(t, err)
	})
}
