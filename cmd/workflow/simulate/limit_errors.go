package simulate

import "fmt"

// LimitKind identifies a specific simulation limit type, allowing callers to
// distinguish limit-exceeded failures from other errors via errors.As.
type LimitKind string

const (
	LimitWASMBinary           LimitKind = "wasm_binary_size"
	LimitWASMCompressedBinary LimitKind = "wasm_compressed_binary_size"
	LimitHTTPRequest          LimitKind = "http_request_size"
	LimitHTTPResponse         LimitKind = "http_response_size"
	LimitConfHTTPRequest      LimitKind = "confidential_http_request_size"
	LimitConfHTTPResponse     LimitKind = "confidential_http_response_size"
	LimitConsensusObservation LimitKind = "consensus_observation_size"
	LimitChainWriteReport     LimitKind = "chain_write_report_size"
	LimitEVMGas               LimitKind = "evm_gas"
)

// LimitExceededError is a typed error for simulation limit violations.
// Callers can use errors.As to retrieve the Kind and distinguish limit
// failures from other errors without string matching.
type LimitExceededError struct {
	Kind LimitKind
	Msg  string
}

func (e *LimitExceededError) Error() string { return e.Msg }

// limitExceeded builds a user-facing error for a byte-size simulation limit
// violation. mirrorsProd should be true when the limit directly maps to a
// production runtime constraint.
func limitExceeded(kind LimitKind, resource string, actual, limit uint64, mirrorsProd bool, remediation string) *LimitExceededError {
	prod := " This limit mirrors a production constraint."
	if !mirrorsProd {
		prod = ""
	}
	return &LimitExceededError{
		Kind: kind,
		Msg: fmt.Sprintf(
			"%s of %d bytes exceeds the simulation limit of %d bytes.%s\n%s. Use 'cre workflow limits export' to customize limits, or --limits=none to disable.",
			resource, actual, limit, prod, remediation,
		),
	}
}

// limitExceededUnit builds a user-facing error for a non-byte simulation limit
// (e.g., EVM gas units).
func limitExceededUnit(kind LimitKind, resource string, actual, limit uint64, unit string, mirrorsProd bool, remediation string) *LimitExceededError {
	prod := " This limit mirrors a production constraint."
	if !mirrorsProd {
		prod = ""
	}
	return &LimitExceededError{
		Kind: kind,
		Msg: fmt.Sprintf(
			"%s of %d %s exceeds the simulation limit of %d %s.%s\n%s. Use 'cre workflow limits export' to customize limits, or --limits=none to disable.",
			resource, actual, unit, limit, unit, prod, remediation,
		),
	}
}
