// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package simulator

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestTimeTravelSchema(t *testing.T) {
	req := SimulationRequest{
		EnvelopeXdr:    "AAAA...",
		Timestamp:      1738077842,
		LedgerSequence: 1234,
	}
	assert.Equal(t, int64(1738077842), req.Timestamp)
	assert.Equal(t, uint32(1234), req.LedgerSequence)
}
