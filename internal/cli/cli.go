// Package cli provides the command-line interface for the OpenAPI converter.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GabrielNunesIT/go-libs/logger"
	"github.com/GabrielNunesIT/openapi-converter/internal/adapters/converters"
	"github.com/GabrielNunesIT/openapi-converter/internal/domain"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cobra"
)

// CLI holds the command-line interface configuration.
type CLI struct {
	log        logger.ILogger
	rootCmd    *cobra.Command
	inputFile  string
	outputFile string
	format     string
}

// New creates a new CLI instance.
func New(log logger.ILogger) *CLI {
	cli := &CLI{
		log: log,
	}

	cli.rootCmd = &cobra.Command{
		Use:   "openapi-converter",
		Short: "Convert OpenAPI specifications to PDF or Word documents",
		Long:  "A CLI tool that converts OpenAPI 3.x specifications to various document formats including PDF and Word (DOCX).",
		RunE:  cli.run,
	}

	cli.setupFlags()

	return cli
}

func (c *CLI) setupFlags() {
	c.rootCmd.Flags().StringVarP(&c.inputFile, "input", "i", "", "Path to the OpenAPI specification file (required)")
	c.rootCmd.Flags().StringVarP(&c.outputFile, "output", "o", "", "Path for the output file (required)")
	c.rootCmd.Flags().StringVarP(&c.format, "format", "f", "pdf", "Output format: pdf, docx")

	_ = c.rootCmd.MarkFlagRequired("input")
	_ = c.rootCmd.MarkFlagRequired("output")
}

// Execute runs the CLI.
func (c *CLI) Execute() error {
	return c.rootCmd.Execute()
}

func (c *CLI) run(_ *cobra.Command, _ []string) error {
	c.log.Infof("Loading OpenAPI specification from: %s", c.inputFile)

	doc, err := c.loadOpenAPI(c.inputFile)
	if err != nil {
		return fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}

	c.log.Infof("Loaded API: %s (v%s)", doc.Title, doc.Version)

	converter, err := c.getConverter()
	if err != nil {
		return err
	}

	c.log.Infof("Converting to %s format...", converter.Format())

	outputFile, err := os.Create(c.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if err := converter.Convert(doc, outputFile); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	c.log.Infof("Successfully created: %s", c.outputFile)

	return nil
}

func (c *CLI) getConverter() (domain.Converter, error) {
	format := strings.ToLower(c.format)

	switch format {
	case "pdf":
		return converters.NewPDFConverter(), nil
	case "docx", "word":
		return converters.NewDocxConverter(), nil
	case "confluence", "adf":
		return converters.NewADFConverter(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: pdf, docx, confluence)", c.format)
	}
}

func (c *CLI) loadOpenAPI(path string) (*domain.OpenAPIDocument, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	spec, err := loader.LoadFromFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI file: %w", err)
	}

	return c.convertSpec(spec), nil
}

func (c *CLI) convertSpec(spec *openapi3.T) *domain.OpenAPIDocument {
	doc := &domain.OpenAPIDocument{
		Title:       spec.Info.Title,
		Version:     spec.Info.Version,
		Description: spec.Info.Description,
		Components:  make(map[string]domain.Schema),
	}

	// Convert servers
	for _, server := range spec.Servers {
		doc.Servers = append(doc.Servers, domain.Server{
			URL:         server.URL,
			Description: server.Description,
		})
	}

	// Convert tags
	for _, tag := range spec.Tags {
		if tag != nil {
			doc.Tags = append(doc.Tags, domain.Tag{
				Name:        tag.Name,
				Description: tag.Description,
			})
		}
	}

	// Convert paths
	for pathStr, pathItem := range spec.Paths.Map() {
		path := domain.Path{Path: pathStr}

		path.Operations = c.convertOperations(pathItem)
		doc.Paths = append(doc.Paths, path)
	}

	// Convert components/schemas
	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, schemaRef := range spec.Components.Schemas {
			doc.Components[name] = c.convertSchema(schemaRef)
		}
	}

	return doc
}

func (c *CLI) convertOperations(pathItem *openapi3.PathItem) []domain.Operation {
	var operations []domain.Operation

	methods := map[string]*openapi3.Operation{
		"GET":     pathItem.Get,
		"POST":    pathItem.Post,
		"PUT":     pathItem.Put,
		"DELETE":  pathItem.Delete,
		"PATCH":   pathItem.Patch,
		"HEAD":    pathItem.Head,
		"OPTIONS": pathItem.Options,
	}

	for method, op := range methods {
		if op == nil {
			continue
		}

		operation := domain.Operation{
			Method:      method,
			Summary:     op.Summary,
			Description: op.Description,
			OperationID: op.OperationID,
			Tags:        op.Tags,
		}

		// Convert parameters
		for _, param := range op.Parameters {
			if param.Value == nil {
				continue
			}

			operation.Parameters = append(operation.Parameters, domain.Parameter{
				Name:        param.Value.Name,
				In:          param.Value.In,
				Description: param.Value.Description,
				Required:    param.Value.Required,
				Schema:      c.convertSchema(param.Value.Schema),
			})
		}

		// Convert responses
		if op.Responses != nil {
			for statusCode, response := range op.Responses.Map() {
				if response.Value == nil {
					continue
				}

				resp := domain.Response{
					StatusCode: statusCode,
				}

				if response.Value.Description != nil {
					resp.Description = *response.Value.Description
				}

				resp.Content = c.convertContent(response.Value.Content)
				operation.Responses = append(operation.Responses, resp)
			}
		}

		// Convert request body
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			operation.RequestBody = &domain.RequestBody{
				Description: op.RequestBody.Value.Description,
				Required:    op.RequestBody.Value.Required,
				Content:     c.convertContent(op.RequestBody.Value.Content),
			}
		}

		operations = append(operations, operation)
	}

	return operations
}

func (c *CLI) convertContent(content openapi3.Content) map[string]domain.MediaType {
	result := make(map[string]domain.MediaType)

	for mediaType, item := range content {
		result[mediaType] = domain.MediaType{
			Schema: c.convertSchema(item.Schema),
		}
	}

	return result
}

func (c *CLI) convertSchema(ref *openapi3.SchemaRef) domain.Schema {
	if ref == nil {
		return domain.Schema{}
	}

	schema := domain.Schema{
		Ref: ref.Ref,
	}

	if ref.Value != nil {
		types := ref.Value.Type.Slice()
		if len(types) > 0 {
			schema.Type = types[0]
		}
		schema.Format = ref.Value.Format
		schema.Description = ref.Value.Description

		// Convert properties
		if len(ref.Value.Properties) > 0 {
			schema.Properties = make(map[string]domain.Schema)

			for name, prop := range ref.Value.Properties {
				schema.Properties[name] = c.convertSchema(prop)
			}
		}

		// Convert items for arrays
		if ref.Value.Items != nil {
			itemSchema := c.convertSchema(ref.Value.Items)
			schema.Items = &itemSchema
		}
	}

	return schema
}
