package common

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// vaultResponseUnmarshal ignores unknown proto fields so newer gateway payloads
// remain decodable when the CLI's chainlink-common pin lags the vault DON.
var vaultResponseUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}

func unmarshalVaultResponsePayload(data []byte, msg proto.Message) error {
	return vaultResponseUnmarshal.Unmarshal(data, msg)
}
