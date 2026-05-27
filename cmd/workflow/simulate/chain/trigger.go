package chain

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseTriggerChainSelector extracts the chain selector from a trigger ID of
// the form "<prefix>:ChainSelector:<digits>..." (case-insensitive). The prefix
// anchor ensures each chain family only claims its own IDs. Returns 0, false
// if the ID does not match.
func ParseTriggerChainSelector(prefix, id string) (uint64, bool) {
	if prefix == "" {
		return 0, false
	}
	re := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `:chainselector:(\d+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(id))
	if len(m) < 2 {
		return 0, false
	}
	v, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
