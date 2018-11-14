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

package pulsar

import (
	"bytes"
	"context"
	"sort"

	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/pkg/errors"
	"github.com/ugorji/go/codec"

	"github.com/insolar/insolar/core"
)

// FetchNeighbour searches neighbour of the pulsar by pubKey of a neighbout
func (currentPulsar *Pulsar) FetchNeighbour(pubKey string) (*Neighbour, error) {
	neighbour, ok := currentPulsar.Neighbours[pubKey]
	if !ok {
		return nil, errors.New("forbidden connection")
	}
	return neighbour, nil
}

// IsStateFailed checks if state of the pulsar is failed or not
func (currentPulsar *Pulsar) IsStateFailed() bool {
	return currentPulsar.StateSwitcher.GetState() == Failed
}

func (currentPulsar *Pulsar) isStandalone() bool {
	return len(currentPulsar.Neighbours) == 0
}

func (currentPulsar *Pulsar) getMaxTraitorsCount() int {
	nodes := len(currentPulsar.Neighbours) + 1
	return (nodes - 1) / 3
}

func (currentPulsar *Pulsar) getMinimumNonTraitorsCount() int {
	nodes := len(currentPulsar.Neighbours) + 1
	return nodes - currentPulsar.getMaxTraitorsCount()
}

func (currentPulsar *Pulsar) handleErrorState(ctx context.Context, err error) {
	inslogger.FromContext(ctx).Error(err)

	currentPulsar.clearState()
}

func (currentPulsar *Pulsar) clearState() {
	currentPulsar.GeneratedEntropy = [core.EntropySize]byte{}
	currentPulsar.GeneratedEntropySign = []byte{}

	currentPulsar.CurrentSlotEntropy = core.Entropy{}
	currentPulsar.CurrentSlotPulseSender = ""

	currentPulsar.currentSlotSenderConfirmationsLock.Lock()
	currentPulsar.CurrentSlotSenderConfirmations = map[string]core.PulseSenderConfirmation{}
	currentPulsar.currentSlotSenderConfirmationsLock.Unlock()

	currentPulsar.OwnedBftRow = map[string]*BftCell{}
	currentPulsar.BftGridLock.Lock()
	currentPulsar.bftGrid = map[string]map[string]*BftCell{}
	currentPulsar.BftGridLock.Unlock()
}

func (currentPulsar *Pulsar) generateNewEntropyAndSign() error {
	currentPulsar.GeneratedEntropy = currentPulsar.EntropyGenerator.GenerateEntropy()
	signature, err := signData(currentPulsar.CryptographyService, currentPulsar.GeneratedEntropy)
	if err != nil {
		return err
	}
	currentPulsar.GeneratedEntropySign = signature

	return nil
}

func (currentPulsar *Pulsar) preparePayload(body interface{}) (*Payload, error) {
	sign, err := signData(currentPulsar.CryptographyService, body)
	if err != nil {
		return nil, err
	}

	return &Payload{Body: body, PublicKey: currentPulsar.PublicKeyRaw, Signature: sign}, nil
}

func checkPayloadSignature(service core.CryptographyService, processor core.KeyProcessor, request *Payload) (bool, error) {
	return checkSignature(service, processor, request.Body, request.PublicKey, request.Signature)
}

func checkSignature(
	service core.CryptographyService,
	processor core.KeyProcessor,
	data interface{},
	pub string,
	signature []byte,
) (bool, error) {
	cborH := &codec.CborHandle{}
	var b bytes.Buffer
	enc := codec.NewEncoder(&b, cborH)
	err := enc.Encode(data)
	if err != nil {
		return false, err
	}

	publicKey, err := processor.ImportPublicKey([]byte(pub))
	if err != nil {
		return false, err
	}

	return service.Verify(publicKey, core.SignatureFromBytes(signature), b.Bytes()), nil
}

func signData(service core.CryptographyService, data interface{}) ([]byte, error) {
	cborH := &codec.CborHandle{}
	var b bytes.Buffer
	enc := codec.NewEncoder(&b, cborH)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	signature, err := service.Sign(b.Bytes())
	if err != nil {
		return nil, err
	}

	return signature.Bytes(), nil
}

func selectByEntropy(scheme core.PlatformCryptographyScheme, entropy core.Entropy, values []string, count int) ([]string, error) { // nolint: megacheck
	type idxHash struct {
		idx  int
		hash []byte
	}

	if len(values) < count {
		return nil, errors.New("count value should be less than values size")
	}

	hashes := make([]*idxHash, 0, len(values))
	for i, value := range values {
		h := scheme.ReferenceHasher()
		_, err := h.Write(entropy[:])
		if err != nil {
			return nil, err
		}
		_, err = h.Write([]byte(value))
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, &idxHash{
			idx:  i,
			hash: h.Sum(nil),
		})
	}

	sort.SliceStable(hashes, func(i, j int) bool { return bytes.Compare(hashes[i].hash, hashes[j].hash) < 0 })

	selected := make([]string, 0, count)
	for i := 0; i < count; i++ {
		selected = append(selected, values[hashes[i].idx])
	}
	return selected, nil
}

// GetLastPulse returns last pulse in the thread-safe mode
func (currentPulsar *Pulsar) GetLastPulse() *core.Pulse {
	currentPulsar.lastPulseLock.RLock()
	defer currentPulsar.lastPulseLock.RUnlock()
	return currentPulsar.lastPulse
}

// SetLastPulse sets last pulse in the thread-safe mode
func (currentPulsar *Pulsar) SetLastPulse(newPulse *core.Pulse) {
	currentPulsar.lastPulseLock.Lock()
	defer currentPulsar.lastPulseLock.Unlock()
	currentPulsar.lastPulse = newPulse

}
