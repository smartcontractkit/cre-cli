package redact

import (
	"path/filepath"
	"regexp"
	"strings"
)

const RedactedValue = "[REDACTED]"

// maxFlagValueLength bounds the length of a flag value that is otherwise considered
// safe to record. A well-formed CLI flag value (identifier, path, enum, number) is
// always well under this; anything longer is more likely to be output, an error
// message, or a payload that ended up in a flag's Value by accident.
const maxFlagValueLength = 256

var (
	fullRedactFlags = map[string]struct{}{
		"env":                    {},
		"public-env":             {},
		"http-payload":           {},
		"public_key":             {},
		"ledger-derivation-path": {},
		"config":                 {},
		"changeset-file":         {},
	}

	// pathRedactFlags hold filesystem paths. Only the base name is kept, since the
	// full path can leak local directory structure or the OS username.
	pathRedactFlags = map[string]struct{}{
		"project-root": {},
	}

	urlValueFlags = map[string]struct{}{
		"rpc-url": {},
		"wasm":    {},
	}

	jwtSegmentPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	bearerPattern     = regexp.MustCompile(`(?i)(Bearer\s+)[^\s]+`)
	apikeyPattern     = regexp.MustCompile(`(?i)(Apikey\s+)[^\s]+`)
	envSecretPattern  = regexp.MustCompile(`(?i)(CRE_API_KEY\s*=\s*)\S+`)
	privateKeyPattern = regexp.MustCompile(`(?i)\b(?:0x)?[0-9a-f]{64}\b`)
	urlPattern        = regexp.MustCompile(`https?://[^\s"'<>]+`)

	templateAddSensitivePattern = regexp.MustCompile(`(?i)(://|\?|ghp_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+)`)

	// suspiciousContentPattern matches text that looks like it came from command
	// output, an error/stack trace, or an HTTP exchange rather than a value a user
	// would type as a CLI flag.
	suspiciousContentPattern = regexp.MustCompile(`(?i)(traceback|panic:|goroutine \d|exception|stack trace|"error"\s*:|http/1\.[01]\s+\d{3}|<html)`)
)

// Flag redacts a single CLI flag value based on flag name.
func Flag(name, value string) string {
	if value == "" {
		return value
	}

	if _, ok := fullRedactFlags[name]; ok {
		return RedactedValue
	}

	if name == "limits" && !isNonSensitiveLimitsValue(value) {
		return RedactedValue
	}

	if _, ok := urlValueFlags[name]; ok {
		return redactURLFlagValue(name, value)
	}

	if _, ok := pathRedactFlags[name]; ok {
		return filepath.Base(value)
	}

	if looksLikeUnintendedContent(value) {
		return RedactedValue
	}

	return value
}

// looksLikeUnintendedContent flags values that don't look like something a user
// would type as a CLI flag - e.g. command output, an error message, or an escaped
// HTTP payload that ended up bound to a flag's Value instead of its intended input.
func looksLikeUnintendedContent(value string) bool {
	if len(value) > maxFlagValueLength {
		return true
	}
	if strings.ContainsAny(value, "\n\r") {
		return true
	}
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return true
	}
	return suspiciousContentPattern.MatchString(value)
}

func isNonSensitiveLimitsValue(value string) bool {
	switch strings.ToLower(value) {
	case "default", "none":
		return true
	default:
		return false
	}
}

func redactURLFlagValue(name, value string) string {
	if name == "rpc-url" {
		chainName, rpcURL, ok := strings.Cut(value, "=")
		if !ok {
			return RedactedValue
		}
		if !looksLikeURL(rpcURL) {
			return value
		}
		return chainName + "=" + URL(rpcURL)
	}

	if looksLikeURL(value) {
		return URL(value)
	}
	return value
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// Args applies command-specific redaction to positional arguments.
func Args(action, subcommand string, args []string) []string {
	if len(args) == 0 {
		return args
	}

	redacted := make([]string, len(args))
	copy(redacted, args)

	switch action {
	case "secrets":
		for i, arg := range redacted {
			redacted[i] = filepath.Base(arg)
		}
	case "templates":
		if subcommand == "add" {
			for i, arg := range redacted {
				if templateAddSensitivePattern.MatchString(arg) {
					redacted[i] = RedactedValue
				}
			}
		}
	}

	// Backstop: positional args are otherwise passed through verbatim, so guard
	// against output/error/payload text that ended up on the command line.
	for i, arg := range redacted {
		if arg != RedactedValue && looksLikeUnintendedContent(arg) {
			redacted[i] = RedactedValue
		}
	}

	return redacted
}

// ErrorMessage scrubs known secret patterns from error strings before telemetry export.
func ErrorMessage(msg string) string {
	if msg == "" {
		return msg
	}

	msg = jwtSegmentPattern.ReplaceAllString(msg, RedactedValue)
	msg = bearerPattern.ReplaceAllString(msg, "${1}"+RedactedValue)
	msg = apikeyPattern.ReplaceAllString(msg, "${1}"+RedactedValue)
	msg = envSecretPattern.ReplaceAllString(msg, "${1}"+RedactedValue)
	msg = privateKeyPattern.ReplaceAllString(msg, RedactedValue)
	msg = urlPattern.ReplaceAllStringFunc(msg, func(raw string) string {
		return URL(raw)
	})

	return msg
}

// SafeJWTClaimsForLog returns an allowlisted subset of JWT claims safe for debug logging.
func SafeJWTClaimsForLog(claims map[string]interface{}) map[string]interface{} {
	if len(claims) == 0 {
		return nil
	}

	safe := make(map[string]interface{})
	for key, value := range claims {
		switch key {
		case "org_id", "sub", "exp", "iat", "iss", "aud":
			safe[key] = value
		default:
			if strings.HasSuffix(key, "organization_status") || strings.HasSuffix(key, "organization_roles") {
				safe[claimLogKey(key)] = value
			}
		}
	}

	if len(safe) == 0 {
		return nil
	}
	return safe
}

func claimLogKey(key string) string {
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		return key[idx+1:]
	}
	return key
}
