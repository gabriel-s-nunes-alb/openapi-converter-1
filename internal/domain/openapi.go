// Package domain provides core business models and interfaces for the OpenAPI converter.
package domain

// OpenAPIDocument represents a parsed OpenAPI specification.
type OpenAPIDocument struct {
	Title       string
	Version     string
	Description string
	Servers     []Server
	Tags        []Tag
	Paths       []Path
	Components  map[string]Schema // Schema components (key is schema name)
	SecuritySchemes map[string]SecurityScheme
	Security        []map[string][]string
}

// SecurityScheme represents a security scheme.
type SecurityScheme struct {
	Type        string
	Name        string
	Description string
	In          string
	Scheme      string
}

// Server represents an API server.
type Server struct {
	URL         string
	Description string
}

// Tag represents an OpenAPI tag.
type Tag struct {
	Name        string
	Description string
}

// Path represents an API endpoint path.
type Path struct {
	Path       string
	Operations []Operation
}

// Operation represents an HTTP operation on a path.
type Operation struct {
	Method      string
	Summary     string
	Description string
	OperationID string
	Tags        []string
	Parameters  []Parameter
	RequestBody *RequestBody
	Responses   []Response
}

// Parameter represents a request parameter.
type Parameter struct {
	Name        string
	In          string // query, path, header, cookie
	Description string
	Required    bool
	Schema      Schema
}

// RequestBody represents a request body.
type RequestBody struct {
	Description string
	Required    bool
	Content     map[string]MediaType
}

// MediaType represents the content type and schema.
type MediaType struct {
	Schema   Schema
	Example  interface{}
	Examples map[string]interface{}
}

// Response represents an API response.
type Response struct {
	StatusCode  string
	Description string
	Content     map[string]MediaType
}

// Schema represents a JSON schema for request/response bodies.
type Schema struct {
	Type        string
	Format      string
	Description string
	Properties  map[string]Schema
	Items       *Schema
	Ref         string
}
