# Proto Nil Linter

A Go static analysis tool that prevents nil assignments to non-optional proto3 message fields in gRPC service handlers.

## Overview

This linter ensures that gRPC responses are always valid by detecting nil assignments to proto3 message fields within service handler methods. It helps prevent runtime panics and invalid API responses caused by unexpected nil values.

**Status**: ✅ Fully implemented, tested, and working

## Features

### Basic Detection
- ✅ Detects direct nil assignments to proto message fields
- ✅ Identifies nil values in composite literals
- ✅ Tracks nil propagation through variables and declarations
- ✅ **Validates nested messages** with non-optional fields
- ✅ **Checks repeated message fields** (list endpoints) for nil items
- ✅ Distinguishes between optional and required proto3 fields
- ✅ Focuses specifically on gRPC service handler methods

### Advanced Data Flow Analysis
- ✅ **Interprocedural tracking**: Analyzes function calls to determine if they return nil
- ✅ **Function summaries**: Caches analysis of helper functions (AlwaysNil, NeverNil, MaybeNil)
- ✅ **Path-sensitive analysis**: Understands nil guards (`if x == nil`) and tracks state through branches
- ✅ **Smart detection**: Reduces false positives by understanding defensive nil checks

### Output & Reporting
- ✅ Provides clear diagnostic messages with file locations
- ✅ Proto.Message interface checking for accurate type detection
- ✅ Context-aware error messages (direct, nested, repeated field)

## Inspired By

This project is inspired by [Uber's nilaway](https://github.com/uber-go/nilaway) but with a focused scope on gRPC response validation rather than general nil safety.

## Detection Patterns

The linter catches these problematic patterns:

### 1. Direct Field Assignment

```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    resp := &pb.Response{}
    resp.User = nil  // ❌ ERROR: nil assignment to non-optional field
    return resp, nil
}
```

### 2. Composite Literal

```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    return &pb.Response{
        User: nil,  // ❌ ERROR: nil in non-optional field
    }, nil
}
```

### 3. Variable Assignment

```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    var user *pb.User = nil
    return &pb.Response{
        User: user,  // ❌ ERROR: nil value from variable
    }, nil
}
```

### 4. Nested Messages

```go
func (s *Service) GetProfile(ctx context.Context, req *pb.Request) (*pb.ProfileResponse, error) {
    return &pb.ProfileResponse{
        User: &pb.User{
            Address: nil,  // ❌ ERROR: nil in nested non-optional field
        },
    }, nil
}
```

### 5. Repeated Messages (Lists)

```go
func (s *Service) ListUsers(ctx context.Context, req *pb.Request) (*pb.ListUsersResponse, error) {
    users := []*pb.User{
        {Name: "Alice"},
        nil,  // ❌ ERROR: nil item in repeated message field
        {Name: "Bob"},
    }
    return &pb.ListUsersResponse{
        Users: users,
    }, nil
}
```

### 6. Nested Fields in Repeated Messages

```go
func (s *Service) ListProfiles(ctx context.Context, req *pb.Request) (*pb.ListProfilesResponse, error) {
    return &pb.ListProfilesResponse{
        Profiles: []*pb.Profile{
            {
                User: &pb.User{
                    Address: nil,  // ❌ ERROR: nil in nested field within list item
                },
            },
        },
    }, nil
}
```

### 7. Advanced: Function Call Returns (Interprocedural)

```go
// Helper that always returns nil
func getUserFromDB() *pb.User {
    return nil
}

func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    user := getUserFromDB()  // Analyzes function and determines it returns AlwaysNil
    return &pb.Response{
        User: user,  // ❌ ERROR: nil from function call
    }, nil
}
```

### 8. Advanced: Path-Sensitive (Nil Guards)

```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    user := findUser(req.Id)  // May return nil
    
    if user == nil {
        return nil, errors.New("not found")
    }
    
    // ✅ OK: user is guaranteed non-nil here due to guard
    return &pb.Response{User: user}, nil
}
```

## Installation

```bash
go install github.com/nick-we/proto-nil-linter/cmd/proto-nil-linter@latest
```

## Usage

### As a standalone tool

```bash
proto-nil-linter ./...
```

### With go vet

```bash
go vet -vettool=$(which proto-nil-linter) ./...
```

### In CI/CD

```yaml
- name: Run proto-nil-linter
  run: |
    go install github.com/nick-we/proto-nil-linter/cmd/proto-nil-linter@latest
    proto-nil-linter ./...
```

## Configuration

Create a `.proto-nil-linter.yaml` file in your project root:

```yaml
# Enable/disable specific checks
checks:
  direct_assignments: true
  composite_literals: true
  data_flow: true
  nested_messages: true      # Check nested message fields
  repeated_messages: true    # Check repeated message fields (lists)

# Patterns to exclude
exclude:
  - "*_test.go"
  - "mock_*.go"

# Proto file paths for field analysis
proto_paths:
  - "./proto"
  - "./api/proto"

# Recursion depth for nested message validation
nested_depth: 5
```

## Architecture

```
┌─────────────────────────────────┐
│     Analysis Entry Point        │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Proto3 Field Analyzer          │
│  • Detect proto message types   │
│  • Identify field optionality   │
│  • Map nested message structure │
│  • Track repeated fields        │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  gRPC Handler Detector          │
│  • Find service methods         │
│  • Extract response types       │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Nil Assignment Detector        │
│  • AST traversal                │
│  • Pattern matching             │
│  • Recursive nested validation  │
│  • List item validation         │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Diagnostic Reporter            │
│  • Format errors                │
│  • Suggest fixes                │
└─────────────────────────────────┘
```

## Development

### Prerequisites

- Go 1.21 or higher
- Protocol Buffers compiler (protoc) - optional for generating test fixtures

### Building

```bash
# Using make
make build

# Using go
go build -o proto-nil-linter ./cmd/proto-nil-linter
```

### Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Quick verification
make quick
```

### Running on example code

```bash
# Run on demo example
cd examples/demo && ../../proto-nil-linter .

# Run on test fixtures
cd pkg/analyzer/testdata/src/example && ../../../../../proto-nil-linter ./...
```

### Test Results

All tests pass ✅:
```bash
$ go test -v ./pkg/analyzer/...
=== RUN   TestAnalyzer
--- PASS: TestAnalyzer (0.25s)
PASS
ok      github.com/nick-we/proto-nil-linter/pkg/analyzer       0.425s
```

## Project Structure

```
.
├── cmd/
│   └── proto-nil-linter/     # CLI entry point
│       └── main.go
├── pkg/
│   ├── analyzer/             # Core analysis logic
│   │   ├── analyzer.go       # Main analyzer
│   │   ├── proto.go          # Proto field detection
│   │   ├── grpc.go           # gRPC handler detection
│   │   ├── nilcheck.go       # Nil assignment detection
│   │   ├── nested.go         # Nested message validation
│   │   └── repeated.go       # Repeated field validation
│   └── config/               # Configuration handling
│       └── config.go
├── testdata/                 # Test fixtures
│   ├── proto/
│   └── services/
├── go.mod
├── go.sum
└── README.md
```

## Validation Depth

The linter performs deep validation to ensure complete response integrity:

1. **Direct fields**: Validates all fields on the response message
2. **Nested messages**: Recursively validates nested message fields (configurable depth)
3. **Repeated fields**: Checks each item in repeated/list fields
4. **Nested + Repeated**: Validates nested fields within each list item

Example proto that will be fully validated:

```protobuf
message ListOrdersResponse {
  repeated Order orders = 1;  // Validates each order
}

message Order {
  User user = 1;              // Non-optional, checked
  Address shipping = 2;       // Non-optional, checked
  repeated Item items = 3;    // Each item checked
}

message User {
  string name = 1;
  Profile profile = 2;        // Nested, checked recursively
}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Roadmap

- [x] Basic nil assignment detection
- [x] Proto3 field analysis
- [x] gRPC handler identification
- [x] Nested message validation
- [x] Repeated message field validation
- [ ] Advanced data flow analysis
- [ ] Performance optimizations

## References

- [Uber's nilaway](https://github.com/uber-go/nilaway)
- [Uber's nilaway blog post](https://www.uber.com/en-DE/blog/nilaway-practical-nil-panic-detection-for-go/)
- [Go static analysis framework](https://pkg.go.dev/golang.org/x/tools/go/analysis)