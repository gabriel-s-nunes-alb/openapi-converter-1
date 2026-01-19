package domain

import "io"

// Converter defines the interface for document converters.
type Converter interface {
	// Convert transforms an OpenAPI document to the target format.
	Convert(doc *OpenAPIDocument, output io.Writer) error

	// Format returns the output format name (e.g., "pdf", "docx").
	Format() string
}
