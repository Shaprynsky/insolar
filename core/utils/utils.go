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

package utils

import (
	"encoding/binary"
	"syscall"

	"github.com/satori/go.uuid"
)

// RandTraceID returns random traceID in uuid format
func RandTraceID() string {
	qid, err := uuid.NewV4()
	if err != nil {
		return "createRandomTraceIDFailed:" + err.Error()
	}
	return qid.String()
}

func UInt32ToBytes(n uint32) []byte {
	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, n)
	return buff
}

func SendGracefulStopSignal() error {
	return syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}
