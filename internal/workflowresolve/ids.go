package workflowresolve

// LooksLikeWorkflowID returns true for 64-char hex strings (on-chain WorkflowId).
func LooksLikeWorkflowID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// LooksLikeUUID returns true for the standard UUID shape (8-4-4-4-12).
func LooksLikeUUID(s string) bool {
	parts := splitDash(s)
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lengths[i] {
			return false
		}
	}
	return true
}

// LooksLikeOnChainExecutionID returns true for 64-char hex execution ids shown in Explorer.
func LooksLikeOnChainExecutionID(s string) bool {
	return LooksLikeWorkflowID(s)
}

func splitDash(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
