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

package keystore

import (
	"testing"

	"github.com/insolar/insolar/configuration"
	"github.com/stretchr/testify/assert"
)

const (
	testKeys    = "testdata/keys.json"
	testBadKeys = "testdata/bad_keys.json"
)

func getConfiguration(keyPath string) configuration.Configuration {
	cfg := configuration.NewConfiguration()
	cfg.KeysPath = keyPath
	return cfg
}

func TestNewKeyStore(t *testing.T) {
	ks, err := NewKeyStore(getConfiguration(testKeys))
	assert.NoError(t, err)
	assert.NotNil(t, ks)
}

func TestNewKeyStore_Fails(t *testing.T) {
	ks, err := NewKeyStore(getConfiguration(testBadKeys))
	assert.Error(t, err)
	assert.Nil(t, ks)
}

func TestKeyStore_GetPrivateKey(t *testing.T) {
	ks, err := NewKeyStore(getConfiguration(testKeys))
	assert.NoError(t, err)

	pk, err := ks.GetPrivateKey("")
	assert.NotNil(t, pk)
	assert.NoError(t, err)
}
