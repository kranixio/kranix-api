package graphql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kranix-io/kranix-packages/types"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Server represents the GraphQL server.
type Server struct {
	schema *ast.Schema
}

// NewServer creates a new GraphQL server.
func NewServer() (*Server, error) {
	schemaStr, err := loadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	schema, err := gqlparser.LoadSchema(schemaStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &Server{
		schema: schema,
	}, nil
}

// loadSchema loads the GraphQL schema from the embedded file.
func loadSchema() (*ast.Source, error) {
	// In production, this would be embedded or loaded from a file
	// For now, return a basic schema
	return &ast.Source{
		Input: schema,
		Name:  "schema.graphql",
	}, nil
}

// ServeHTTP handles GraphQL HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := s.executeQuery(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GraphQLRequest represents a GraphQL request.
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message string   `json:"message"`
	Path    []string `json:"path,omitempty"`
}

// executeQuery executes a GraphQL query.
func (s *Server) executeQuery(ctx interface{}, req *GraphQLRequest) (*GraphQLResponse, error) {
	// Parse the query
	_, err := gqlparser.LoadQuery(s.schema, req.Query)
	if err != nil {
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: err.Error()}},
		}, nil
	}

	// Execute the query
	// TODO: Implement proper query execution with resolvers
	// For now, return a placeholder response
	result := map[string]interface{}{
		"workloads": []types.Workload{},
	}

	return &GraphQLResponse{
		Data: result,
	}, nil
}

// ConvertTime converts time.Time to GraphQL Time scalar.
func ConvertTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ParseTime parses a GraphQL Time scalar to time.Time.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// Basic schema embedded for now
const schema = `
scalar Time

type Workload {
  id: ID!
  name: String!
  namespace: String!
  spec: WorkloadSpec!
  status: WorkloadStatus!
  createdAt: Time!
  updatedAt: Time!
  labels: [Label!]!
  tenant: TenantInfo
}

type WorkloadSpec {
  name: String!
  namespace: String
  image: String!
  replicas: Int!
  env: [EnvVar!]!
  command: String
  resources: ResourceSpec
  ports: [PortSpec!]!
  backend: String!
  composeFile: String
}

type WorkloadStatus {
  id: ID!
  name: String!
  namespace: String
  state: String!
  image: String
  replicas: Int
  ready: Int
  host: String
  pods: [String!]!
  phase: WorkloadPhase!
  readyReplicas: Int!
  message: String
  lastUpdated: Time!
}

enum WorkloadPhase {
  Pending
  Deploying
  Running
  Degraded
  Failed
}

type ResourceSpec {
  cpuRequest: String
  cpuLimit: String
  memoryRequest: String
  memoryLimit: String
}

type PortSpec {
  name: String
  containerPort: Int!
  protocol: String
}

type TenantInfo {
  id: ID!
  name: String!
  namespace: String!
  labels: [Label!]!
  quota: TenantQuota
  isolation: TenantIsolation
}

type TenantQuota {
  maxCPU: String
  maxMemory: String
  maxWorkloads: Int
  maxReplicas: Int
  maxStorage: String
  maxCustomMetrics: Int
}

type TenantIsolation {
  networkPolicy: Boolean!
  resourceQuota: Boolean!
  limitRange: Boolean!
  podSecurityPolicy: Boolean!
  storageClass: String
}

type Label {
  key: String!
  value: String!
}

type EnvVar {
  key: String!
  value: String!
}

type Query {
  workloads(namespace: String, limit: Int, offset: Int): [Workload!]!
  workload(id: ID!): Workload
  namespaces: [Namespace!]!
  namespace(id: ID!): Namespace
}

type Namespace {
  id: ID!
  name: String!
  labels: [Label!]!
  createdAt: Time!
  updatedAt: Time!
}

type Mutation {
  deployWorkload(spec: WorkloadSpecInput!): Workload!
  createNamespace(name: String!, labels: [LabelInput!]): Namespace!
}

input WorkloadSpecInput {
  name: String!
  namespace: String
  image: String!
  replicas: Int!
  env: [EnvVarInput!]!
  command: String
  resources: ResourceSpecInput
  ports: [PortSpecInput!]!
  backend: String!
  composeFile: String
}

input ResourceSpecInput {
  cpuRequest: String
  cpuLimit: String
  memoryRequest: String
  memoryLimit: String
}

input PortSpecInput {
  name: String
  containerPort: Int!
  protocol: String
}

input LabelInput {
  key: String!
  value: String!
}

input EnvVarInput {
  key: String!
  value: String!
}
`
