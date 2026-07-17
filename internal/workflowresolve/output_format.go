package workflowresolve

import "fmt"

const OutputFormatJSON = "json"

// ResolveOutputFormat normalises --json / --output flags into a validated output format.
func ResolveOutputFormat(outputFormat string, jsonFlag bool) (string, error) {
	if jsonFlag {
		outputFormat = OutputFormatJSON
	}
	if outputFormat != "" && outputFormat != OutputFormatJSON {
		return "", fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, OutputFormatJSON)
	}
	return outputFormat, nil
}
