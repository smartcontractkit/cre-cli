package simulate

import (
	"regexp"
	"strconv"
	"time"
)

const TIMEOUT = 30 * time.Second
const (
	SEPOLIA_MOCK_KEYSTONE_FORWARDER_ADDRESS = "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"
	MAINNET_MOCK_KEYSTONE_FORWARDER_ADDRESS = "0xa3d1ad4ac559a6575a114998affb2fb2ec97a7d9"
	SEPOLIA_CHAIN_SELECTOR                  = 16015286601757825753
	MAINNET_CHAIN_SELECTOR                  = 5009297550715157269
)

// ---- SUPPORTED CHAINS ----
var supportedEVM = []struct {
	Selector  uint64
	Forwarder string
}{
	{Selector: SEPOLIA_CHAIN_SELECTOR, Forwarder: SEPOLIA_MOCK_KEYSTONE_FORWARDER_ADDRESS},
	{Selector: MAINNET_CHAIN_SELECTOR, Forwarder: MAINNET_MOCK_KEYSTONE_FORWARDER_ADDRESS},
}

// parse "ChainSelector:<digits>" from trigger id, e.g. "evm:ChainSelector:5009297550715157269@1.0.0 LogTrigger"
var chainSelectorRe = regexp.MustCompile(`(?i)chainselector:(\d+)`)

func parseChainSelectorFromTriggerID(id string) (uint64, bool) {
	m := chainSelectorRe.FindStringSubmatch(id)
	if len(m) < 2 {
		return 0, false
	}

	v, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}
