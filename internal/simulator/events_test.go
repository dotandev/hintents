package simulator

import (
	"encoding/base64"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticEvent_ParseData(t *testing.T) {
	// Create a valid ScVal (e.g. ScValTypeScvU32)
	u32Val := xdr.Uint32(42)
	val := xdr.ScVal{
		Type: xdr.ScValTypeScvU32,
		U32:  &u32Val,
	}

	b, err := val.MarshalBinary()
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(b)

	event := DiagnosticEvent{
		Data: encoded,
	}

	parsed, err := event.ParseData()
	require.NoError(t, err)
	assert.Equal(t, xdr.ScValTypeScvU32, parsed.Type)
	assert.NotNil(t, parsed.U32)
	assert.Equal(t, xdr.Uint32(42), *parsed.U32)

	// Test empty Data
	emptyEvent := DiagnosticEvent{Data: ""}
	emptyParsed, err := emptyEvent.ParseData()
	require.NoError(t, err)
	assert.Equal(t, xdr.ScValType(0), emptyParsed.Type)

	// Test invalid base64
	invalidB64Event := DiagnosticEvent{Data: "not_base64@#$"}
	_, err = invalidB64Event.ParseData()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode data base64")

	// Test invalid XDR
	invalidXDREvent := DiagnosticEvent{Data: base64.StdEncoding.EncodeToString([]byte("bad xdr byte seq"))}
	_, err = invalidXDREvent.ParseData()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal data xdr")
}

func TestDiagnosticEvent_ParseTopics(t *testing.T) {
	u32Val1 := xdr.Uint32(11)
	val1 := xdr.ScVal{
		Type: xdr.ScValTypeScvU32,
		U32:  &u32Val1,
	}
	b1, err := val1.MarshalBinary()
	require.NoError(t, err)

	u32Val2 := xdr.Uint32(22)
	val2 := xdr.ScVal{
		Type: xdr.ScValTypeScvU32,
		U32:  &u32Val2,
	}
	b2, err := val2.MarshalBinary()
	require.NoError(t, err)

	event := DiagnosticEvent{
		Topics: []string{
			base64.StdEncoding.EncodeToString(b1),
			base64.StdEncoding.EncodeToString(b2),
		},
	}

	topics, err := event.ParseTopics()
	require.NoError(t, err)
	require.Len(t, topics, 2)
	assert.Equal(t, xdr.Uint32(11), *topics[0].U32)
	assert.Equal(t, xdr.Uint32(22), *topics[1].U32)

	// Test invalid topic base64
	badB64Event := DiagnosticEvent{Topics: []string{"bad@#$"}}
	_, err = badB64Event.ParseTopics()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode topic[0] base64")

	// Test invalid topic XDR
	badXDREvent := DiagnosticEvent{Topics: []string{base64.StdEncoding.EncodeToString([]byte("invalid xdr bytes"))}}
	_, err = badXDREvent.ParseTopics()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal topic[0] xdr")
}
