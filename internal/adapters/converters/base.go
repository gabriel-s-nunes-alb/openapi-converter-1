// Package converters provides implementations for converting OpenAPI documents to various formats.
package converters

import (
	"fmt"
	"strings"

	"github.com/GabrielNunesIT/openapi-converter/internal/domain"
)

// formatMethod returns a styled method string.
func formatMethod(method string) string {
	return strings.ToUpper(method)
}

// formatParameters returns a formatted parameter list.
func formatParameters(params []domain.Parameter) string {
	if len(params) == 0 {
		return "None"
	}

	var result strings.Builder

	for _, p := range params {
		required := ""
		if p.Required {
			required = " (required)"
		}

		result.WriteString(fmt.Sprintf("- %s (%s): %s%s\n", p.Name, p.In, p.Description, required))
	}

	return result.String()
}

// formatResponses returns a formatted response list.
func formatResponses(responses []domain.Response) string {
	if len(responses) == 0 {
		return "None"
	}

	var result strings.Builder

	for _, r := range responses {
		result.WriteString(fmt.Sprintf("- %s: %s\n", r.StatusCode, r.Description))
	}

	return result.String()
}
