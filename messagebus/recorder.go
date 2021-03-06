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
	"io"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/ledger/localstorage"
)

// Recorder is a MessageBus wrapper that stores received replies to the tape. The tape then can be transferred and
// used by Player to replay those replies.
type recorder struct {
	sender
	tape   tape
	pm     core.PulseManager
	scheme core.PlatformCryptographyScheme
}

// newRecorder create new recorder instance.
func newRecorder(s sender, tape tape, pm core.PulseManager, scheme core.PlatformCryptographyScheme) *recorder {
	return &recorder{sender: s, tape: tape, pm: pm, scheme: scheme}
}

// WriteTape writes recorder's tape to the provided writer.
func (r *recorder) WriteTape(ctx context.Context, w io.Writer) error {
	return r.tape.Write(ctx, w)
}

// Send wraps MessageBus Send to save received replies to the tape. This reply is also used to return directly from the
// tape is the message is sent again, thus providing a cash for message replies.
func (r *recorder) Send(ctx context.Context, msg core.Message, optionSetter ...core.SendOption) (core.Reply, error) {
	var (
		rep core.Reply
		err error
	)
	var options *core.SendOptions
	if len(optionSetter) > 0 {
		options = &core.SendOptions{}
		for _, setter := range optionSetter {
			setter(options)
		}
	}

	parcel, err := r.CreateParcel(ctx, msg, options)
	if err != nil{
		return nil, err
	}
	id := GetMessageHash(r.scheme, parcel)

	// Check if Value for this message is already stored.
	rep, err = r.tape.GetReply(ctx, id)
	if err == nil {
		return rep, nil
	}
	if err != localstorage.ErrNotFound {
		return nil, err
	}

	// Actually send message.
	rep, err = r.SendParcel(ctx, parcel, options)
	if err != nil {
		return nil, err
	}

	// Save the received Value on the storageTape.
	err = r.tape.SetReply(ctx, id, rep)
	if err != nil {
		return nil, err
	}

	return rep, nil
}
