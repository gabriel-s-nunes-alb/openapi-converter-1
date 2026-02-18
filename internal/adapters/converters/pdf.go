package converters

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/GabrielNunesIT/openapi-converter/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

const (
	pdfFormat      = "pdf"
	pdfPageWidth   = 190.0
	pdfMarginLeft  = 10.0
	pdfMarginTop   = 10.0
	pdfMarginRight = 10.0
	pdfLineHeight  = 5.0
)

// PDFConverter converts OpenAPI documents to PDF format.
type PDFConverter struct {
	pdf            *gofpdf.Fpdf
	tocItems       []tocItem
	linkID         int
	componentLinks map[string]int // Map "tag:component" to link ID
	currentTag     string         // Current tag context for link resolution
}

type tocItem struct {
	title  string
	level  int
	linkID int
	page   int
}

// NewPDFConverter creates a new PDF converter.
func NewPDFConverter() *PDFConverter {
	return &PDFConverter{}
}

// Format returns the output format name.
func (c *PDFConverter) Format() string {
	return pdfFormat
}

// Convert transforms an OpenAPI document to PDF format.
func (c *PDFConverter) Convert(doc *domain.OpenAPIDocument, output io.Writer) error {
	c.pdf = gofpdf.New("P", "mm", "A4", "")
	c.pdf.SetMargins(pdfMarginLeft, pdfMarginTop, pdfMarginRight)
	c.pdf.SetDrawColor(180, 180, 180) // Light gray for all borders
	c.tocItems = nil
	c.linkID = 0
	c.componentLinks = make(map[string]int)
	c.currentTag = ""

	// First pass: collect TOC items with placeholder pages
	c.collectTOC(doc)

	// Title page
	c.addTitlePage(doc)

	// Table of contents
	c.addTableOfContents()

	// Content pages
	c.addContent(doc)

	return c.pdf.Output(output)
}

func (c *PDFConverter) collectTOC(doc *domain.OpenAPIDocument) {
	// Add main sections to TOC
	c.tocItems = append(c.tocItems, tocItem{title: "Overview", level: 1, linkID: c.pdf.AddLink()})

	if len(doc.Servers) > 0 {
		c.tocItems = append(c.tocItems, tocItem{title: "Servers", level: 1, linkID: c.pdf.AddLink()})
	}

	// Group paths by tags
	tagPaths := c.groupPathsByTag(doc)
	tags := make([]string, 0, len(tagPaths))
	for tag := range tagPaths {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	// Pre-create links for all tag+component combinations
	for _, tag := range tags {
		tagComponents := c.collectTagComponents(tagPaths[tag])
		for _, compName := range tagComponents {
			key := tag + ":" + compName
			c.componentLinks[key] = c.pdf.AddLink()
		}
	}

	// Add Endpoints section
	c.tocItems = append(c.tocItems, tocItem{title: "API Endpoints", level: 1, linkID: c.pdf.AddLink()})

	for _, tag := range tags {
		c.tocItems = append(c.tocItems, tocItem{title: tag, level: 2, linkID: c.pdf.AddLink()})

		for _, ep := range tagPaths[tag] {
			title := fmt.Sprintf("%s %s", ep.method, ep.path)
			c.tocItems = append(c.tocItems, tocItem{title: title, level: 3, linkID: c.pdf.AddLink()})
		}
	}
}

// collectTagComponents gathers all unique component names used by endpoints in a tag.
func (c *PDFConverter) collectTagComponents(endpoints []endpointRef) []string {
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
func (c *PDFConverter) collectSchemaRefs(schema domain.Schema, refs map[string]struct{}) {
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

type endpointRef struct {
	path      string
	method    string
	operation domain.Operation
}

func (c *PDFConverter) groupPathsByTag(doc *domain.OpenAPIDocument) map[string][]endpointRef {
	result := make(map[string][]endpointRef)

	for _, path := range doc.Paths {
		for _, op := range path.Operations {
			tags := op.Tags
			if len(tags) == 0 {
				tags = []string{"Default"}
			}

			for _, tag := range tags {
				result[tag] = append(result[tag], endpointRef{
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

func (c *PDFConverter) addTitlePage(doc *domain.OpenAPIDocument) {
	c.pdf.AddPage()

	// Title
	c.pdf.SetFont("Arial", "B", 28)
	c.pdf.Ln(40)
	c.pdf.CellFormat(pdfPageWidth, 15, doc.Title, "", 1, "C", false, 0, "")
	c.pdf.Ln(5)

	// Version
	c.pdf.SetFont("Arial", "", 14)
	c.pdf.SetTextColor(100, 100, 100)
	c.pdf.CellFormat(pdfPageWidth, 8, fmt.Sprintf("Version %s", doc.Version), "", 1, "C", false, 0, "")
	c.pdf.SetTextColor(0, 0, 0)
	c.pdf.Ln(20)

	// Description
	if doc.Description != "" {
		c.pdf.SetFont("Arial", "", 11)
		// Clean HTML from description
		desc := stripHTML(doc.Description)
		c.pdf.MultiCell(pdfPageWidth, 6, desc, "", "C", false)
	}

	c.pdf.Ln(30)

	// API Info
	c.pdf.SetFont("Arial", "", 10)
	c.pdf.SetTextColor(128, 128, 128)
	c.pdf.CellFormat(pdfPageWidth, 6, "OpenAPI Specification Document", "", 1, "C", false, 0, "")
	c.pdf.SetTextColor(0, 0, 0)
}

func (c *PDFConverter) addTableOfContents() {
	c.pdf.AddPage()

	c.pdf.SetFont("Arial", "B", 20)
	c.pdf.CellFormat(pdfPageWidth, 10, "Table of Contents", "", 1, "", false, 0, "")
	c.pdf.Ln(8)

	for _, item := range c.tocItems {
		indent := float64(item.level-1) * 8

		switch item.level {
		case 1:
			c.pdf.SetFont("Arial", "B", 12)
		case 2:
			c.pdf.SetFont("Arial", "B", 10)
		default:
			c.pdf.SetFont("Arial", "", 9)
		}

		// Title with link
		c.pdf.SetX(pdfMarginLeft + indent)
		title := item.title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		c.pdf.CellFormat(pdfPageWidth-indent, pdfLineHeight, title, "", 1, "", false, item.linkID, "")
	}
}

func (c *PDFConverter) addContent(doc *domain.OpenAPIDocument) {
	tocIndex := 0

	// Overview section
	c.pdf.AddPage()
	c.setLinkDest(tocIndex)
	tocIndex++

	c.addSectionHeader("Overview")

	if doc.Description != "" {
		c.pdf.SetFont("Arial", "", 10)
		c.pdf.MultiCell(pdfPageWidth, 5, stripHTML(doc.Description), "", "", false)
		c.pdf.Ln(4)
	}

	// Servers
	if len(doc.Servers) > 0 {
		c.checkPageBreak(40)
		c.setLinkDest(tocIndex)
		tocIndex++

		c.addSectionHeader("Servers")

		for _, server := range doc.Servers {
			c.pdf.SetFont("Arial", "B", 10)
			c.pdf.SetTextColor(0, 102, 204)
			c.pdf.CellFormat(pdfPageWidth, 6, server.URL, "", 1, "", false, 0, "")
			c.pdf.SetTextColor(0, 0, 0)

			if server.Description != "" {
				c.pdf.SetFont("Arial", "", 9)
				c.pdf.SetTextColor(100, 100, 100)
				c.pdf.MultiCell(pdfPageWidth, 4, server.Description, "", "", false)
				c.pdf.SetTextColor(0, 0, 0)
			}
			c.pdf.Ln(2)
		}
		c.pdf.Ln(4)
	}

	// API Endpoints header
	c.pdf.AddPage()
	c.setLinkDest(tocIndex)
	tocIndex++

	c.addSectionHeader("API Endpoints")
	c.pdf.Ln(4)

	// Create lookup for tag descriptions
	tagDescs := make(map[string]string)
	for _, t := range doc.Tags {
		tagDescs[t.Name] = t.Description
	}

	// Group by tags
	tagPaths := c.groupPathsByTag(doc)
	tags := make([]string, 0, len(tagPaths))
	for tag := range tagPaths {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		c.pdf.AddPage()
		c.setLinkDest(tocIndex)
		tocIndex++

		// Tag header
		c.pdf.SetFont("Arial", "B", 14)
		c.pdf.SetFillColor(240, 240, 240)
		c.pdf.CellFormat(pdfPageWidth, 8, tag, "", 1, "", true, 0, "")
		c.pdf.Ln(4)

		// Set current tag context for link resolution
		c.currentTag = tag

		// Tag description
		if desc, ok := tagDescs[tag]; ok && desc != "" {
			c.pdf.SetFont("Arial", "", 10)
			c.pdf.MultiCell(pdfPageWidth, 5, stripHTML(desc), "", "", false)
			c.pdf.Ln(4)
		}

		// Endpoints Summary
		c.addEndpointsSummary(tagPaths[tag], tocIndex)
		c.pdf.Ln(6)

		for _, ep := range tagPaths[tag] {
			c.checkPageBreak(50)
			c.setLinkDest(tocIndex)
			tocIndex++

			c.addEndpoint(ep.path, ep.operation)
		}

		// Add components used by this tag's endpoints at the bottom
		tagComponents := c.collectTagComponents(tagPaths[tag])
		if len(tagComponents) > 0 {
			c.pdf.Ln(6)
			c.pdf.SetDrawColor(180, 180, 180)
			c.pdf.Line(pdfMarginLeft, c.pdf.GetY(), pdfMarginLeft+pdfPageWidth, c.pdf.GetY())
			c.pdf.Ln(6)
			c.addTagComponents(tag, tagComponents, doc.Components)
		}

		c.pdf.Ln(4)
	}
}

func (c *PDFConverter) setLinkDest(tocIndex int) {
	if tocIndex < len(c.tocItems) {
		c.pdf.SetLink(c.tocItems[tocIndex].linkID, -1, -1)
	}
}

func (c *PDFConverter) addSectionHeader(title string) {
	c.pdf.SetFont("Arial", "B", 18)
	c.pdf.CellFormat(pdfPageWidth, 10, title, "", 1, "", false, 0, "")
	c.pdf.Ln(4)
}

func (c *PDFConverter) addEndpoint(pathStr string, op domain.Operation) {
	// Method badge with color
	c.pdf.SetFont("Arial", "B", 11)

	methodColors := map[string][3]int{
		"GET":     {97, 175, 254},  // Blue
		"POST":    {73, 204, 144},  // Green
		"PUT":     {252, 161, 48},  // Orange
		"DELETE":  {249, 62, 62},   // Red
		"PATCH":   {80, 227, 194},  // Teal
		"HEAD":    {144, 97, 249},  // Purple
		"OPTIONS": {128, 128, 128}, // Gray
	}

	color := methodColors[op.Method]
	if color == [3]int{} {
		color = [3]int{128, 128, 128}
	}

	c.pdf.SetFillColor(color[0], color[1], color[2])
	c.pdf.SetTextColor(255, 255, 255)
	methodWidth := float64(len(op.Method)*3) + 8
	c.pdf.CellFormat(methodWidth, 7, op.Method, "", 0, "C", true, 0, "")

	// Path
	c.pdf.SetTextColor(0, 0, 0)
	c.pdf.SetFont("Arial", "B", 11)
	c.pdf.CellFormat(pdfPageWidth-methodWidth, 7, " "+pathStr, "", 1, "", false, 0, "")
	c.pdf.Ln(2)

	// Operation ID
	if op.OperationID != "" {
		c.pdf.SetFont("Arial", "", 8)
		c.pdf.SetTextColor(128, 128, 128)
		c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("Operation ID: %s", op.OperationID), "", 1, "", false, 0, "")
		c.pdf.SetTextColor(0, 0, 0)
	}

	// Summary
	if op.Summary != "" {
		c.pdf.SetFont("Arial", "B", 10)
		c.pdf.MultiCell(pdfPageWidth, 5, stripHTML(op.Summary), "", "", false)
	}

	// Description
	if op.Description != "" {
		c.pdf.SetFont("Arial", "", 9)
		desc := stripHTML(op.Description)
		c.pdf.MultiCell(pdfPageWidth, 4, desc, "", "", false)
	}
	c.pdf.Ln(2)

	// Parameters
	if len(op.Parameters) > 0 {
		c.addSubHeader("Parameters")
		c.addParameterTable(op.Parameters)
	}

	// Request Body
	if op.RequestBody != nil {
		c.addSubHeader("Request Body")
		c.addRequestBody(op.RequestBody)
	}

	// Responses
	if len(op.Responses) > 0 {
		c.addSubHeader("Responses")
		c.addResponseTable(op.Responses)
	}

	// Separator
	c.pdf.Ln(2)
	c.pdf.SetDrawColor(220, 220, 220)
	c.pdf.Line(pdfMarginLeft, c.pdf.GetY(), pdfMarginLeft+pdfPageWidth, c.pdf.GetY())
	c.pdf.SetDrawColor(180, 180, 180) // Reset to standard light gray
	c.pdf.Ln(6)
}

func (c *PDFConverter) addSubHeader(title string) {
	c.pdf.SetFont("Arial", "B", 10)
	c.pdf.SetTextColor(60, 60, 60)
	c.pdf.CellFormat(pdfPageWidth, 6, title, "", 1, "", false, 0, "")
	c.pdf.SetTextColor(0, 0, 0)
}

func (c *PDFConverter) addParameterTable(params []domain.Parameter) {
	// Table header
	c.pdf.SetFont("Arial", "B", 8)
	c.pdf.SetFillColor(245, 245, 245)

	colWidths := []float64{35, 20, 15, 60, 60}
	headers := []string{"Name", "In", "Required", "Type", "Description"}

	for i, header := range headers {
		c.pdf.CellFormat(colWidths[i], 6, header, "1", 0, "", true, 0, "")
	}
	c.pdf.Ln(-1)

	// Table rows
	c.pdf.SetFont("Arial", "", 8)
	for _, param := range params {
		required := "No"
		if param.Required {
			required = "Yes"
		}

		schemaType := param.Schema.Type
		if param.Schema.Format != "" {
			schemaType = fmt.Sprintf("%s (%s)", schemaType, param.Schema.Format)
		}
		if param.Schema.Ref != "" {
			schemaType = extractRefName(param.Schema.Ref)
		}

		desc := stripHTML(param.Description)

		contents := []string{param.Name, param.In, required, schemaType, desc}
		aligns := []string{"L", "L", "C", "L", "L"}
		
		c.addTableRow(colWidths, contents, aligns, nil)
	}
	c.pdf.Ln(3)
}

func (c *PDFConverter) addRequestBody(rb *domain.RequestBody) {
	if rb.Required {
		c.pdf.SetFont("Arial", "I", 9)
		c.pdf.SetTextColor(60, 60, 60)
		c.pdf.CellFormat(pdfPageWidth, 5, "Required", "", 1, "", false, 0, "")
		c.pdf.SetTextColor(0, 0, 0)
	}

	if rb.Description != "" {
		c.pdf.SetFont("Arial", "", 9)
		c.pdf.MultiCell(pdfPageWidth, 4, stripHTML(rb.Description), "", "", false)
	}

	// Content types
	if len(rb.Content) > 0 {
		c.pdf.Ln(2)
		c.pdf.SetFont("Arial", "B", 8)
		c.pdf.SetFillColor(245, 245, 245)

		colWidths := []float64{60, 130}
		headers := []string{"Content-Type", "Object"}

		for i, header := range headers {
			c.pdf.CellFormat(colWidths[i], 6, header, "1", 0, "", true, 0, "")
		}
		c.pdf.Ln(-1)

		c.pdf.SetFont("Arial", "", 8)

		contentTypes := make([]string, 0, len(rb.Content))
		for ct := range rb.Content {
			contentTypes = append(contentTypes, ct)
		}
		sort.Strings(contentTypes)

		type bodyExample struct {
			title   string
			content interface{}
		}
		var examples []bodyExample

		for _, contentType := range contentTypes {
			media := rb.Content[contentType]
			
			objectStr := ""
			var linkID int

			if media.Schema.Ref != "" {
				refName := extractRefName(media.Schema.Ref)
				objectStr = refName
				key := c.currentTag + ":" + refName
				linkID = c.componentLinks[key]
			} else {
				objectStr = media.Schema.Type
				if media.Schema.Format != "" {
					objectStr = fmt.Sprintf("%s (%s)", objectStr, media.Schema.Format)
				}
				if objectStr == "array" && media.Schema.Items != nil {
					itemType := media.Schema.Items.Type
					if media.Schema.Items.Ref != "" {
						refName := extractRefName(media.Schema.Items.Ref)
						itemType = refName
						key := c.currentTag + ":" + refName
						linkID = c.componentLinks[key]
					}
					objectStr = fmt.Sprintf("[]%s", itemType)
				}
				if objectStr == "" {
					objectStr = "Object"
				}
			}

			contents := []string{contentType, objectStr}
			aligns := []string{"L", "L"}
			linkIDs := []int{0, linkID}
			
			c.addTableRow(colWidths, contents, aligns, linkIDs)

			if media.Example != nil {
				examples = append(examples, bodyExample{title: contentType, content: media.Example})
			}

			if len(media.Examples) > 0 {
				names := make([]string, 0, len(media.Examples))
                for name := range media.Examples {
                    names = append(names, name)
                }
                sort.Strings(names)
                for _, name := range names {
					examples = append(examples, bodyExample{title: fmt.Sprintf("%s (%s)", contentType, name), content: media.Examples[name]})
				}
			}
		}

		if len(examples) > 0 {
			c.pdf.Ln(4)
			c.addSubHeader("Request Examples")
			for _, ex := range examples {
				c.addExample(ex.title, ex.content)
			}
		}
	}
	c.pdf.Ln(2)
}

func (c *PDFConverter) addSchemaInfo(schema domain.Schema, indent int) {
	c.pdf.SetFont("Arial", "", 8)
	indentStr := strings.Repeat("  ", indent)

	if schema.Ref != "" {
		refName := extractRefName(schema.Ref)
		key := c.currentTag + ":" + refName
		linkID := c.componentLinks[key]
		c.pdf.SetTextColor(0, 102, 204)
		c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("%sObject: %s", indentStr, refName), "", 1, "", false, linkID, "")
		c.pdf.SetTextColor(0, 0, 0)
		return
	}

	schemaType := schema.Type
	if schema.Format != "" {
		schemaType = fmt.Sprintf("%s (%s)", schemaType, schema.Format)
	}

	if schemaType != "" && schemaType != "object" {
		c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("%sType: %s", indentStr, schemaType), "", 1, "", false, 0, "")
	}

	if schema.Description != "" {
		desc := stripHTML(schema.Description)
		
		// Handle indentation for description
		indentWidth := c.pdf.GetStringWidth(strings.Repeat("  ", indent))
		currentX := c.pdf.GetX()
		c.pdf.SetX(currentX + indentWidth)
		c.pdf.MultiCell(pdfPageWidth-indentWidth, 4, desc, "", "", false)
	}

	// Properties
	if len(schema.Properties) > 0 {
		c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("%sProperties:", indentStr), "", 1, "", false, 0, "")
		for name, prop := range schema.Properties {
			propType := prop.Type
			if prop.Ref != "" {
				propType = extractRefName(prop.Ref)
			}
			c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("%s  - %s: %s", indentStr, name, propType), "", 1, "", false, 0, "")
		}
	}

	// Array items
	if schema.Items != nil {
		c.pdf.CellFormat(pdfPageWidth, 4, fmt.Sprintf("%sItems:", indentStr), "", 1, "", false, 0, "")
		c.addSchemaInfo(*schema.Items, indent+1)
	}
}

func (c *PDFConverter) addResponseTable(responses []domain.Response) {
	// Sort responses by status code
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].StatusCode < responses[j].StatusCode
	})

	// Table header
	c.pdf.SetFont("Arial", "B", 8)
	c.pdf.SetFillColor(245, 245, 245)

	colWidths := []float64{25, 95, 70}
	headers := []string{"Status", "Description", "Object"}

	for i, header := range headers {
		c.pdf.CellFormat(colWidths[i], 6, header, "1", 0, "", true, 0, "")
	}
	c.pdf.Ln(-1)

	// Table rows
	c.pdf.SetFont("Arial", "", 8)
	for _, resp := range responses {
		desc := stripHTML(resp.Description)

		// Get schema reference
		schemaRef := ""
		var schemaLinkID int
		for _, media := range resp.Content {
			if media.Schema.Ref != "" {
				refName := extractRefName(media.Schema.Ref)
				schemaRef = refName
				key := c.currentTag + ":" + refName
				schemaLinkID = c.componentLinks[key]

				break
			} else if media.Schema.Type != "" {
				schemaRef = media.Schema.Type
			}
		}

		// Color code status
		// Note: color change only affects the status code text if we set it before drawing
		// But addTableRow doesn't support per-cell text color yet unless we enhance it.
		// For simplicity, we drop the color feature for status code or we have to enhance addTableRow.
		// Or we can just set color inside addTableRow if we pass it? 
		// Actually typical tables don't need colored status codes desperately, but let's keep it simple.
		
		contents := []string{resp.StatusCode, desc, schemaRef}
		aligns := []string{"C", "L", "L"}
		linkIDs := []int{0, 0, schemaLinkID}
		
		c.addTableRow(colWidths, contents, aligns, linkIDs)
	}

	// Gather examples from responses to display after table
	type respExample struct {
		title   string
		content interface{}
	}
	var examples []respExample

	for _, resp := range responses {
		for mediaType, media := range resp.Content {
            // First check if there is a single example
			if media.Example != nil {
				examples = append(examples, respExample{
					title:   fmt.Sprintf("%s - %s", resp.StatusCode, mediaType),
					content: media.Example,
				})
			}
            // Also check for named examples
            if len(media.Examples) > 0 {
                // To keep order consistent
                names := make([]string, 0, len(media.Examples))
                for name := range media.Examples {
                    names = append(names, name)
                }
                sort.Strings(names)
                for _, name := range names {
                    examples = append(examples, respExample{
                        title:   fmt.Sprintf("%s - %s (%s)", resp.StatusCode, mediaType, name),
                        content: media.Examples[name],
                    })
                }
            }
		}
	}

	if len(examples) > 0 {
		c.pdf.Ln(4)
		c.addSubHeader("Response Examples")
		for _, ex := range examples {
			c.addExample(ex.title, ex.content)
		}
	}

	c.pdf.Ln(3)
}

func (c *PDFConverter) checkPageBreak(height float64) {
	_, pageHeight := c.pdf.GetPageSize()
	_, _, _, bottomMargin := c.pdf.GetMargins()

	if c.pdf.GetY()+height > pageHeight-bottomMargin-10 {
		c.pdf.AddPage()
	}
}

func stripHTML(s string) string {
	// Simple HTML tag removal
	result := s
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	// Clean up common HTML entities
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	result = strings.ReplaceAll(result, "\n\n", "\n")
	return strings.TrimSpace(result)
}

func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}

func (c *PDFConverter) addComponentSchema(name string, schema domain.Schema) {
	// Component name as Title
	c.pdf.SetFont("Arial", "B", 12)
	c.pdf.CellFormat(pdfPageWidth, 7, name, "", 1, "", false, 0, "")

	// Type
	if schema.Type != "" && schema.Type != "object" {
		c.pdf.SetFont("Arial", "", 9)
		typeStr := schema.Type
		if schema.Format != "" {
			typeStr = fmt.Sprintf("%s (%s)", schema.Type, schema.Format)
		}
		c.pdf.CellFormat(pdfPageWidth, 5, fmt.Sprintf("Type: %s", typeStr), "", 1, "", false, 0, "")
	}

	// Description
	if schema.Description != "" {
		c.pdf.SetFont("Arial", "", 9)
		c.pdf.SetTextColor(100, 100, 100)
		desc := stripHTML(schema.Description)
		c.pdf.MultiCell(pdfPageWidth, 4, desc, "", "", false)
		c.pdf.SetTextColor(0, 0, 0)
	}

	// Properties table
	if len(schema.Properties) > 0 {
		c.pdf.Ln(2)

		// Component Name Header
		c.pdf.SetFont("Arial", "B", 9)
		c.pdf.SetFillColor(245, 245, 245)
		c.pdf.CellFormat(pdfPageWidth, 6, name, "1", 1, "C", true, 0, "")

		// Table header
		c.pdf.SetFont("Arial", "B", 8)
		c.pdf.SetFillColor(245, 245, 245)
		propColWidths := []float64{50, 50, 90}
		propHeaders := []string{"Name", "Type", "Description"}

		for i, header := range propHeaders {
			c.pdf.CellFormat(propColWidths[i], 5, header, "1", 0, "", true, 0, "")
		}
		c.pdf.Ln(-1)

		// Property rows
		c.pdf.SetFont("Arial", "", 8)
		propNames := make([]string, 0, len(schema.Properties))
		for propName := range schema.Properties {
			propNames = append(propNames, propName)
		}
		sort.Strings(propNames)

		for _, propName := range propNames {
			prop := schema.Properties[propName]

			propType := prop.Type
			var propLinkID int
			if prop.Ref != "" {
				refName := extractRefName(prop.Ref)
				propType = refName
				key := c.currentTag + ":" + refName
				propLinkID = c.componentLinks[key]
			} else if prop.Format != "" {
				propType = fmt.Sprintf("%s (%s)", prop.Type, prop.Format)
			}

			propDesc := stripHTML(prop.Description)

			contents := []string{propName, propType, propDesc}
			aligns := []string{"L", "L", "L"}
			linkIDs := []int{0, propLinkID, 0}
			
			c.addTableRow(propColWidths, contents, aligns, linkIDs)
		}
	}

	c.pdf.Ln(6)
}

// addTagComponents renders the component schemas used by endpoints in a tag.
func (c *PDFConverter) addTagComponents(tag string, componentNames []string, components map[string]domain.Schema) {
	c.pdf.SetFont("Arial", "B", 11)
	c.pdf.SetTextColor(60, 60, 60)
	c.pdf.CellFormat(pdfPageWidth, 6, "Objects Used", "", 1, "", false, 0, "")
	c.pdf.SetTextColor(0, 0, 0)
	c.pdf.Ln(2)

	for _, name := range componentNames {
		schema, exists := components[name]
		if !exists {
			continue
		}

		c.checkPageBreak(30)

		// Set the link destination for this tag+component
		key := tag + ":" + name
		if linkID, ok := c.componentLinks[key]; ok {
			c.pdf.SetLink(linkID, -1, -1)
		}

		c.addComponentSchema(name, schema)
	}

	// Separator after components
	c.pdf.Ln(2)
	c.pdf.SetDrawColor(180, 180, 180)
	c.pdf.Line(pdfMarginLeft, c.pdf.GetY(), pdfMarginLeft+pdfPageWidth, c.pdf.GetY())
	c.pdf.Ln(6)
}

func (c *PDFConverter) addTableRow(colWidths []float64, contents []string, aligns []string, linkIDs []int) {
	// Calculate max height based on content wrapping
	maxLines := 1
	for i, content := range contents {
		width := colWidths[i]
		lines := c.pdf.SplitLines([]byte(content), width)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	rowHeight := float64(maxLines) * 5.0 // 5.0 is base line height for cells

	c.checkPageBreak(rowHeight)

	// Draw cells
	startX := c.pdf.GetX()
	startY := c.pdf.GetY()

	for i, content := range contents {
		width := colWidths[i]
		
		align := ""
		if len(aligns) > i {
			align = aligns[i]
		}
		
		linkID := 0
		if len(linkIDs) > i {
			linkID = linkIDs[i]
		}
		
		// If linkID is present, set text color blue
		if linkID > 0 {
			c.pdf.SetTextColor(0, 102, 204)
		}

		// Draw content
		c.pdf.SetXY(startX, startY)
		c.pdf.MultiCell(width, 5.0, content, "0", align, false)
		if linkID > 0 {
			// Add link over the area
			c.pdf.Link(startX, startY, width, rowHeight, linkID)
			c.pdf.SetTextColor(0, 0, 0) // Reset color
		}

		// Draw border
		c.pdf.Rect(startX, startY, width, rowHeight, "D")
		
		// Move X for next cell
		startX += width
	}
	
	// Move cursor to next row
	c.pdf.SetXY(pdfMarginLeft, startY+rowHeight)
}

func (c *PDFConverter) addEndpointsSummary(endpoints []endpointRef, startTocIndex int) {
	if len(endpoints) == 0 {
		return
	}

	c.pdf.SetFont("Arial", "B", 11)
	c.pdf.CellFormat(pdfPageWidth, 6, "Endpoints in this section", "", 1, "", false, 0, "")
	c.pdf.Ln(2)

	// Table header
	c.pdf.SetFont("Arial", "B", 9)
	c.pdf.SetFillColor(245, 245, 245)

	colWidths := []float64{100, 75, 15}
	headers := []string{"Summary", "Path", "Method"}

	for i, header := range headers {
		c.pdf.CellFormat(colWidths[i], 6, header, "1", 0, "", true, 0, "")
	}
	c.pdf.Ln(-1)

	// Table rows
	c.pdf.SetFont("Arial", "", 9)
	currentTocIndex := startTocIndex

	for _, ep := range endpoints {
		summary := stripHTML(ep.operation.Summary)
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}

		contents := []string{summary, ep.path, ep.method}
		aligns := []string{"L", "L", "C"}
		
		var linkIDs []int
		if currentTocIndex < len(c.tocItems) {
			linkID := c.tocItems[currentTocIndex].linkID
			linkIDs = []int{linkID, linkID, linkID}
		}
		
		c.addTableRow(colWidths, contents, aligns, linkIDs)
		currentTocIndex++
	}
}

func (c *PDFConverter) addExample(title string, example interface{}) {
	c.checkPageBreak(30) // Ensure enough space or break

	c.pdf.SetFont("Arial", "I", 9)
	c.pdf.SetTextColor(60, 60, 60)
	c.pdf.CellFormat(pdfPageWidth, 6, "Example ("+title+"):", "", 1, "", false, 0, "")

	c.pdf.SetFont("Courier", "", 8)
	c.pdf.SetTextColor(0, 0, 0)
	c.pdf.SetFillColor(250, 250, 250)

	var content string
	if b, err := json.MarshalIndent(example, "", "  "); err == nil {
		content = string(b)
	} else {
		content = fmt.Sprintf("%v", example)
	}

	// Calculate height
	lines := strings.Split(content, "\n")
	height := float64(len(lines)) * 4.0 // 4.0 is likely not enough for MultiCell, usually line height.
	// MultiCell line height is passed as argument, 4 here.
	c.checkPageBreak(height + 2)

	c.pdf.MultiCell(pdfPageWidth, 4, content, "1", "", true)
	c.pdf.Ln(4)
}
