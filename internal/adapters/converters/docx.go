package converters

import (
	"fmt"
	"io"

	"github.com/GabrielNunesIT/openapi-converter/internal/domain"
	"github.com/gomutex/godocx"
	"github.com/gomutex/godocx/docx"
)

const docxFormat = "docx"

// DocxConverter converts OpenAPI documents to Word (DOCX) format.
type DocxConverter struct{}

// NewDocxConverter creates a new DOCX converter.
func NewDocxConverter() *DocxConverter {
	return &DocxConverter{}
}

// Format returns the output format name.
func (c *DocxConverter) Format() string {
	return docxFormat
}

// Convert transforms an OpenAPI document to DOCX format.
func (c *DocxConverter) Convert(doc *domain.OpenAPIDocument, output io.Writer) error {
	document, err := godocx.NewDocument()
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}

	c.addTitle(document, doc)
	c.addDescription(document, doc)
	c.addServers(document, doc)
	c.addPaths(document, doc)

	if err := document.Write(output); err != nil {
		return fmt.Errorf("failed to write document: %w", err)
	}

	return nil
}

func (c *DocxConverter) addTitle(document *docx.RootDoc, doc *domain.OpenAPIDocument) {
	_, _ = document.AddHeading(doc.Title, 0) // Level 0 = Title style
	document.AddParagraph(fmt.Sprintf("Version: %s", doc.Version))
	document.AddEmptyParagraph()
}

func (c *DocxConverter) addDescription(document *docx.RootDoc, doc *domain.OpenAPIDocument) {
	if doc.Description == "" {
		return
	}

	_, _ = document.AddHeading("Description", 1)
	document.AddParagraph(doc.Description)
	document.AddEmptyParagraph()
}

func (c *DocxConverter) addServers(document *docx.RootDoc, doc *domain.OpenAPIDocument) {
	if len(doc.Servers) == 0 {
		return
	}

	_, _ = document.AddHeading("Servers", 1)

	for _, server := range doc.Servers {
		text := server.URL
		if server.Description != "" {
			text = fmt.Sprintf("%s - %s", server.URL, server.Description)
		}

		document.AddParagraph(fmt.Sprintf("• %s", text))
	}

	document.AddEmptyParagraph()
}

func (c *DocxConverter) addPaths(document *docx.RootDoc, doc *domain.OpenAPIDocument) {
	if len(doc.Paths) == 0 {
		return
	}

	_, _ = document.AddHeading("API Endpoints", 1)

	for _, path := range doc.Paths {
		c.addPath(document, path)
	}
}

func (c *DocxConverter) addPath(document *docx.RootDoc, path domain.Path) {
	for _, op := range path.Operations {
		c.addOperation(document, path.Path, op)
	}
}

func (c *DocxConverter) addOperation(document *docx.RootDoc, pathStr string, op domain.Operation) {
	// Method and path header
	_, _ = document.AddHeading(fmt.Sprintf("%s %s", formatMethod(op.Method), pathStr), 2)

	// Summary
	if op.Summary != "" {
		document.AddParagraph(op.Summary)
	}

	// Description
	if op.Description != "" {
		document.AddParagraph(op.Description)
	}

	// Parameters
	if len(op.Parameters) > 0 {
		_, _ = document.AddHeading("Parameters", 3)

		for _, param := range op.Parameters {
			required := ""
			if param.Required {
				required = " (required)"
			}

			document.AddParagraph(fmt.Sprintf("• %s (%s): %s%s", param.Name, param.In, param.Description, required))
		}
	}

	// Responses
	if len(op.Responses) > 0 {
		_, _ = document.AddHeading("Responses", 3)

		for _, resp := range op.Responses {
			document.AddParagraph(fmt.Sprintf("• %s: %s", resp.StatusCode, resp.Description))
		}
	}

	document.AddEmptyParagraph()
}
