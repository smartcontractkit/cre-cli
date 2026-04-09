package simulate

import (
	"regexp"
	"strconv"
	"time"
)

const WorkflowExecutionTimeout = 5 * time.Minute

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
