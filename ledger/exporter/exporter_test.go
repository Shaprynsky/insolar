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

package exporter

import (
	"strconv"
	"testing"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/ledger/record"
	"github.com/insolar/insolar/ledger/storage/storagetest"
	"github.com/jbenet/go-base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestExporter_Export(t *testing.T) {
	ctx := inslogger.TestContext(t)
	db, clean := storagetest.TmpDB(ctx, t)
	defer clean()

	exporter := NewExporter(db)

	err := db.AddPulse(ctx, core.Pulse{PulseNumber: core.FirstPulseNumber, PulseTimestamp: 1})
	require.NoError(t, err)
	err = db.AddPulse(ctx, core.Pulse{PulseNumber: core.FirstPulseNumber + 1, PulseTimestamp: 2})
	require.NoError(t, err)
	err = db.AddPulse(ctx, core.Pulse{PulseNumber: core.FirstPulseNumber + 2, PulseTimestamp: 3})
	require.NoError(t, err)

	type testData struct {
		Field string
	}
	mem := make([]byte, 0)
	codec.NewEncoderBytes(&mem, &codec.CborHandle{}).MustEncode(testData{Field: "objectValue"})
	blobID, err := db.SetBlob(ctx, core.FirstPulseNumber+1, mem)
	require.NoError(t, err)
	_, err = db.SetRecord(ctx, core.FirstPulseNumber+1, &record.GenesisRecord{})
	require.NoError(t, err)
	objectID, err := db.SetRecord(ctx, core.FirstPulseNumber+1, &record.ObjectActivateRecord{
		ObjectStateRecord: record.ObjectStateRecord{
			Memory: blobID,
		},
		IsDelegate: true,
	})
	payload := message.ParcelToBytes(&message.Parcel{LogTraceID: "callRequest"})
	requestID, err := db.SetRecord(ctx, core.FirstPulseNumber+1, &record.CallRequest{
		Payload: payload,
	})
	require.NoError(t, err)

	type kv = map[string]interface{}
	result, err := exporter.Export(ctx, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Data))
	assert.Equal(t, 3, result.Size)
	assert.Nil(t, result.NextFrom)

	result, err = exporter.Export(ctx, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, 2, result.Size)
	assert.Equal(t, core.FirstPulseNumber+2, int(*result.NextFrom))

	pulse := result.Data[strconv.FormatUint(uint64(core.FirstPulseNumber), 10)].(pulseData).Pulse
	assert.Equal(t, core.FirstPulseNumber, int(pulse.PulseNumber))
	assert.Equal(t, int64(1), pulse.PulseTimestamp)
	pulse = result.Data[strconv.FormatUint(uint64(core.FirstPulseNumber+1), 10)].(pulseData).Pulse
	assert.Equal(t, core.FirstPulseNumber+1, int(pulse.PulseNumber))
	assert.Equal(t, int64(2), pulse.PulseTimestamp)

	records := result.Data[strconv.FormatUint(uint64(core.FirstPulseNumber+1), 10)].(pulseData).Records
	object := records[base58.Encode(objectID[:])]
	assert.Equal(t, "TypeActivate", object.Type)
	assert.Equal(t, true, object.Data.(*record.ObjectActivateRecord).IsDelegate)
	assert.Equal(t, "objectValue", object.Payload["Memory"].(kv)["Field"])

	request := records[base58.Encode(requestID[:])]
	assert.Equal(t, "TypeCallRequest", request.Type)
	assert.Equal(t, payload, request.Data.(*record.CallRequest).Payload)
	assert.Equal(t, "callRequest", request.Payload["Payload"].(*message.Parcel).LogTraceID)
}
