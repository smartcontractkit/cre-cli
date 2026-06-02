package registrytype

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// Type is the normalised registry type stored in context.yaml.
type Type string

const (
	OnChain  Type = "on-chain"
	OffChain Type = "off-chain"
	Unknown  Type = "unknown"
)

const (
	GQLOnChain  = "ON_CHAIN"
	GQLOffChain = "OFF_CHAIN"
)

// FromGQL maps a GraphQL registry type to the normalised context.yaml value.
func FromGQL(gqlType string, log *zerolog.Logger) Type {
	switch gqlType {
	case GQLOnChain:
		return OnChain
	case GQLOffChain:
		return OffChain
	default:
		log.Warn().Str("type", gqlType).Msg("unrecognised registry type from server")
		return Unknown
	}
}

// Parse converts a raw type string from context.yaml to a Type.
// Unrecognised values return an error; there is no default to on-chain.
func Parse(raw string) (Type, error) {
	switch {
	case strings.EqualFold(raw, string(OffChain)), strings.EqualFold(raw, "off_chain"):
		return OffChain, nil
	case strings.EqualFold(raw, string(OnChain)), strings.EqualFold(raw, "on_chain"):
		return OnChain, nil
	case strings.EqualFold(raw, string(Unknown)):
		return Unknown, nil
	default:
		return "", fmt.Errorf("unrecognised registry type %q", raw)
	}
}
