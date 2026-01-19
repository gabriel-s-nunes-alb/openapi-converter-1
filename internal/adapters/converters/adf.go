package converters

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/GabrielNunesIT/openapi-converter/internal/domain"
)

const adfFormat = "confluence"

// ADFConverter converts OpenAPI documents to Atlassian Document Format (ADF) for Confluence.
type ADFConverter struct{}

// NewADFConverter creates a new ADF converter.
func NewADFConverter() *ADFConverter {
	return &ADFConverter{}
}

// Format returns the output format name.
func (c *ADFConverter) Format() string {
	return adfFormat
}

// ADF node types.
type adfDocument struct {
	Version int       `json:"version"`
	Type    string    `json:"type"`
	Content []adfNode `json:"content"`
}

type adfNode struct {
	Type    string     `json:"type"`
	Attrs   *adfAttrs  `json:"attrs,omitempty"`
	Content []adfNode  `json:"content,omitempty"`
	Text    string     `json:"text,omitempty"`
	Marks   []adfMark  `json:"marks,omitempty"`
}

type adfAttrs struct {
	Level int    `json:"level,omitempty"`
	Order int    `json:"order,omitempty"`
	URL   string `json:"url,omitempty"`
}

type adfMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Convert transforms an OpenAPI document to ADF JSON format.
func (c *ADFConverter) Convert(doc *domain.OpenAPIDocument, output io.Writer) error {
	adf := &adfDocument{
		Version: 1,
		Type:    "doc",
		Content: []adfNode{},
	}

	// Title
	adf.Content = append(adf.Content, c.heading(doc.Title, 1))
	adf.Content = append(adf.Content, c.paragraph(fmt.Sprintf("Version: %s", doc.Version)))

	// Description
	if doc.Description != "" {
		adf.Content = append(adf.Content, c.heading("Description", 2))
		adf.Content = append(adf.Content, c.paragraph(doc.Description))
	}

	// Servers
	if len(doc.Servers) > 0 {
		adf.Content = append(adf.Content, c.heading("Servers", 2))
		adf.Content = append(adf.Content, c.serverList(doc.Servers))
	}

	// Endpoints
	if len(doc.Paths) > 0 {
		adf.Content = append(adf.Content, c.heading("API Endpoints", 2))

		for _, path := range doc.Paths {
			adf.Content = append(adf.Content, c.pathNodes(path)...)
		}
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(adf); err != nil {
		return fmt.Errorf("failed to encode ADF: %w", err)
	}

	return nil
}

func (c *ADFConverter) heading(text string, level int) adfNode {
	return adfNode{
		Type:  "heading",
		Attrs: &adfAttrs{Level: level},
		Content: []adfNode{
			{Type: "text", Text: text},
		},
	}
}

func (c *ADFConverter) paragraph(text string) adfNode {
	return adfNode{
		Type: "paragraph",
		Content: []adfNode{
			{Type: "text", Text: text},
		},
	}
}

func (c *ADFConverter) boldText(text string) adfNode {
	return adfNode{
		Type: "text",
		Text: text,
		Marks: []adfMark{
			{Type: "strong"},
		},
	}
}

func (c *ADFConverter) codeText(text string) adfNode {
	return adfNode{
		Type: "text",
		Text: text,
		Marks: []adfMark{
			{Type: "code"},
		},
	}
}

func (c *ADFConverter) serverList(servers []domain.Server) adfNode {
	items := make([]adfNode, 0, len(servers))

	for _, server := range servers {
		text := server.URL
		if server.Description != "" {
			text = fmt.Sprintf("%s - %s", server.URL, server.Description)
		}

		items = append(items, adfNode{
			Type: "listItem",
			Content: []adfNode{
				c.paragraph(text),
			},
		})
	}

	return adfNode{
		Type:    "bulletList",
		Content: items,
	}
}

func (c *ADFConverter) pathNodes(path domain.Path) []adfNode {
	var nodes []adfNode

	for _, operation := range path.Operations {
		nodes = append(nodes, c.operationNodes(path.Path, operation)...)
	}

	return nodes
}

func (c *ADFConverter) operationNodes(pathStr string, operation domain.Operation) []adfNode {
	var nodes []adfNode

	// Endpoint heading with method and path
	endpointTitle := fmt.Sprintf("%s %s", formatMethod(operation.Method), pathStr)
	nodes = append(nodes, c.heading(endpointTitle, 3))

	// Summary (bold)
	if operation.Summary != "" {
		nodes = append(nodes, adfNode{
			Type: "paragraph",
			Content: []adfNode{
				c.boldText(operation.Summary),
			},
		})
	}

	// Description
	if operation.Description != "" {
		nodes = append(nodes, c.paragraph(operation.Description))
	}

	// Parameters
	if len(operation.Parameters) > 0 {
		nodes = append(nodes, c.heading("Parameters", 4))
		nodes = append(nodes, c.parameterList(operation.Parameters))
	}

	// Responses
	if len(operation.Responses) > 0 {
		nodes = append(nodes, c.heading("Responses", 4))
		nodes = append(nodes, c.responseList(operation.Responses))
	}

	// Divider between endpoints
	nodes = append(nodes, adfNode{Type: "rule"})

	return nodes
}

func (c *ADFConverter) parameterList(params []domain.Parameter) adfNode {
	items := make([]adfNode, 0, len(params))

	for _, param := range params {
		required := ""
		if param.Required {
			required = " (required)"
		}

		text := fmt.Sprintf("%s (%s): %s%s", param.Name, param.In, param.Description, required)

		items = append(items, adfNode{
			Type: "listItem",
			Content: []adfNode{
				{
					Type: "paragraph",
					Content: []adfNode{
						c.codeText(param.Name),
						{Type: "text", Text: fmt.Sprintf(" (%s): %s%s", param.In, param.Description, required)},
					},
				},
			},
		})

		// Suppress unused variable
		_ = text
	}

	return adfNode{
		Type:    "bulletList",
		Content: items,
	}
}

func (c *ADFConverter) responseList(responses []domain.Response) adfNode {
	items := make([]adfNode, 0, len(responses))

	for _, resp := range responses {
		items = append(items, adfNode{
			Type: "listItem",
			Content: []adfNode{
				{
					Type: "paragraph",
					Content: []adfNode{
						c.codeText(resp.StatusCode),
						{Type: "text", Text: fmt.Sprintf(": %s", resp.Description)},
					},
				},
			},
		})
	}

	return adfNode{
		Type:    "bulletList",
		Content: items,
	}
}
