package converters

import (
	"fmt"
	"io"
	"sort"

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

type docxEndpointRef struct {
	path      string
	method    string
	operation domain.Operation
}

// groupPathsByTag groups paths by their operation tags.
func (c *DocxConverter) groupPathsByTag(doc *domain.OpenAPIDocument) map[string][]docxEndpointRef {
	result := make(map[string][]docxEndpointRef)

	for _, path := range doc.Paths {
		for _, op := range path.Operations {
			tags := op.Tags
			if len(tags) == 0 {
				tags = []string{"Default"}
			}

			for _, tag := range tags {
				result[tag] = append(result[tag], docxEndpointRef{
					path:      path.Path,
					method:    op.Method,
					operation: op,
				})
			}
		}
	}

	// Sort endpoints within each tag by path then method
	for tag := range result {
		sort.Slice(result[tag], func(i, j int) bool {
			if result[tag][i].path == result[tag][j].path {
				return result[tag][i].method < result[tag][j].method
			}

			return result[tag][i].path < result[tag][j].path
		})
	}

	return result
}

// collectTagComponents gathers all unique component names used by endpoints in a tag.
func (c *DocxConverter) collectTagComponents(endpoints []docxEndpointRef) []string {
	componentSet := make(map[string]struct{})

	for _, ep := range endpoints {
		// Check request body
		if ep.operation.RequestBody != nil {
			for _, media := range ep.operation.RequestBody.Content {
				c.collectSchemaRefs(media.Schema, componentSet)
			}
		}

		// Check responses
		for _, resp := range ep.operation.Responses {
			for _, media := range resp.Content {
				c.collectSchemaRefs(media.Schema, componentSet)
			}
		}

		// Check parameters
		for _, param := range ep.operation.Parameters {
			c.collectSchemaRefs(param.Schema, componentSet)
		}
	}

	// Convert set to sorted slice
	components := make([]string, 0, len(componentSet))
	for name := range componentSet {
		components = append(components, name)
	}
	sort.Strings(components)

	return components
}

// collectSchemaRefs recursively collects component references from a schema.
func (c *DocxConverter) collectSchemaRefs(schema domain.Schema, refs map[string]struct{}) {
	if schema.Ref != "" {
		refs[extractRefName(schema.Ref)] = struct{}{}
	}

	for _, prop := range schema.Properties {
		c.collectSchemaRefs(prop, refs)
	}

	if schema.Items != nil {
		c.collectSchemaRefs(*schema.Items, refs)
	}
}

func (c *DocxConverter) addPaths(document *docx.RootDoc, doc *domain.OpenAPIDocument) {
	if len(doc.Paths) == 0 {
		return
	}

	_, _ = document.AddHeading("API Endpoints", 1)

	// Group by tags
	tagPaths := c.groupPathsByTag(doc)
	tags := make([]string, 0, len(tagPaths))
	for tag := range tagPaths {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		// Tag header
		_, _ = document.AddHeading(tag, 2)

		// Add components used by this tag's endpoints
		tagComponents := c.collectTagComponents(tagPaths[tag])
		if len(tagComponents) > 0 {
			c.addTagComponents(document, tagComponents, doc.Components)
		}

		// Add endpoints
		for _, ep := range tagPaths[tag] {
			c.addOperation(document, ep.path, ep.operation)
		}
	}
}

// addTagComponents renders the component schemas used by endpoints in a tag.
func (c *DocxConverter) addTagComponents(document *docx.RootDoc, componentNames []string, components map[string]domain.Schema) {
	_, _ = document.AddHeading("Schemas Used", 3)

	for _, name := range componentNames {
		schema, exists := components[name]
		if !exists {
			continue
		}

		c.addComponentSchema(document, name, schema)
	}

	document.AddEmptyParagraph()
}

// addComponentSchema renders a single component schema.
func (c *DocxConverter) addComponentSchema(document *docx.RootDoc, name string, schema domain.Schema) {
	// Schema name as bold heading
	_, _ = document.AddHeading(name, 4)

	// Type info
	if schema.Type != "" {
		typeStr := schema.Type
		if schema.Format != "" {
			typeStr = fmt.Sprintf("%s (%s)", schema.Type, schema.Format)
		}
		document.AddParagraph(fmt.Sprintf("Type: %s", typeStr))
	}

	// Description
	if schema.Description != "" {
		document.AddParagraph(schema.Description)
	}

	// Properties
	if len(schema.Properties) > 0 {
		document.AddParagraph("Properties:")

		propNames := make([]string, 0, len(schema.Properties))
		for propName := range schema.Properties {
			propNames = append(propNames, propName)
		}
		sort.Strings(propNames)

		for _, propName := range propNames {
			prop := schema.Properties[propName]
			propType := prop.Type
			if prop.Ref != "" {
				propType = extractRefName(prop.Ref)
			} else if prop.Format != "" {
				propType = fmt.Sprintf("%s (%s)", prop.Type, prop.Format)
			}

			propDesc := ""
			if prop.Description != "" {
				propDesc = fmt.Sprintf(" - %s", prop.Description)
			}

			document.AddParagraph(fmt.Sprintf("  • %s (%s)%s", propName, propType, propDesc))
		}
	}

	document.AddEmptyParagraph()
}

func (c *DocxConverter) addOperation(document *docx.RootDoc, pathStr string, op domain.Operation) {
	// Method and path header
	_, _ = document.AddHeading(fmt.Sprintf("%s %s", formatMethod(op.Method), pathStr), 3)

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
		_, _ = document.AddHeading("Parameters", 4)

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
		_, _ = document.AddHeading("Responses", 4)

		for _, resp := range op.Responses {
			document.AddParagraph(fmt.Sprintf("• %s: %s", resp.StatusCode, resp.Description))
		}
	}

	document.AddEmptyParagraph()
}
