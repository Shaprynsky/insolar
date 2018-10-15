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

package proxyctx

import (
	"github.com/insolar/insolar/core"
)

// ProxyHelper interface with methods that are needed by contract proxies
type ProxyHelper interface {
	RouteCall(ref core.RecordRef, wait bool, method string, args []byte) ([]byte, error)
	SaveAsChild(parentRef, classRef core.RecordRef, constructorName string, argsSerialized []byte) (core.RecordRef, error)
	GetObjChildren(head core.RecordRef, class core.RecordRef) ([]core.RecordRef, error)
	SaveAsDelegate(parentRef, classRef core.RecordRef, constructorName string, argsSerialized []byte) (core.RecordRef, error)
	GetDelegate(object, ofType core.RecordRef) (core.RecordRef, error)
	DeactivateObject(object core.RecordRef) error
	Serialize(what interface{}, to *[]byte) error
	Deserialize(from []byte, into interface{}) error
	MakeErrorSerializable(error) error
}

// Current - hackish way to give proxies access to the current environment
var Current ProxyHelper
