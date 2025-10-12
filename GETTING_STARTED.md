# Getting Started with Proto Nil Linter

This guide will help you get up and running with the proto-nil-linter tool.

## Prerequisites

- Go 1.21 or higher
- Protocol Buffers compiler (protoc) - optional, only if you want to generate proto code
- Basic understanding of gRPC and Protocol Buffers

## Installation

### Option 1: Install from source (recommended for development)

```bash
# Clone the repository
git clone https://github.com/nick-we/proto-nil-linter.git
cd proto-nil-linter

# Build and install
make install

# Verify installation
proto-nil-linter --help
```

### Option 2: Go install (when published)

```bash
go install github.com/nick-we/proto-nil-linter/cmd/proto-nil-linter@latest
```

### Option 3: Build binary

```bash
# Build for current platform
make build

# Binary will be in bin/proto-nil-linter
./bin/proto-nil-linter --help
```

## Quick Start

### 1. Run on Your Project

```bash
# From your project root
proto-nil-linter ./...

# Or run on specific packages
proto-nil-linter ./internal/services/...
```

### 2. Run with go vet

```bash
go vet -vettool=$(which proto-nil-linter) ./...
```

### 3. Run on Example Code

```bash
# Use the provided examples
make run-example
```

## Understanding the Output

When the linter finds issues, you'll see output like this:

```
testdata/src/example/service/user_service.go:30:15: nil assignment to non-optional proto field GetUserResponse.User in direct assignment (gRPC handler response must not contain nil)
testdata/src/example/service/user_service.go:36:9: nil assignment to non-optional proto field GetUserResponse.User in composite literal (gRPC handler response must not contain nil)
testdata/src/example/service/user_service.go:102:17: nil item at index 1 in repeated field ListUsersResponse.Users
```

Each line contains:
- **File and location**: Where the issue was found
- **Error type**: What kind of nil assignment was detected
- **Field path**: Which proto field is affected
- **Context**: Where in the code (direct assignment, composite literal, etc.)

## Common Issues and Solutions

### Issue 1: Nil in Direct Assignment

**Problem:**
```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    resp := &pb.Response{}
    resp.User = nil  // ❌ ERROR
    return resp, nil
}
```

**Solution:**
```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    return &pb.Response{
        User: &pb.User{
            Name: "Unknown",
            Email: "unknown@example.com",
            Address: &pb.Address{
                Street: "Unknown",
                City: "Unknown",
                Country: "Unknown",
            },
        },
    }, nil
}
```

### Issue 2: Nil in Composite Literal

**Problem:**
```go
return &pb.Response{
    User: nil,  // ❌ ERROR
}, nil
```

**Solution:**
```go
return &pb.Response{
    User: &pb.User{
        // Initialize with default values
        Name: "",
        Email: "",
        Address: &pb.Address{},
    },
}, nil
```

### Issue 3: Nil in Nested Messages

**Problem:**
```go
return &pb.Response{
    User: &pb.User{
        Name: "John",
        Address: nil,  // ❌ ERROR
    },
}, nil
```

**Solution:**
```go
return &pb.Response{
    User: &pb.User{
        Name: "John",
        Address: &pb.Address{
            Street: "",  // Use empty values instead of nil
            City: "",
            Country: "",
        },
    },
}, nil
```

### Issue 4: Nil Items in Lists

**Problem:**
```go
return &pb.ListResponse{
    Users: []*pb.User{
        {Name: "Alice"},
        nil,  // ❌ ERROR
        {Name: "Bob"},
    },
}, nil
```

**Solution:**
```go
return &pb.ListResponse{
    Users: []*pb.User{
        {Name: "Alice", Address: &pb.Address{}},
        // Remove nil entry or replace with valid user
        {Name: "Bob", Address: &pb.Address{}},
    },
}, nil
```

## Design Patterns for Valid Responses

### Pattern 1: Builder Functions

```go
func newDefaultUser() *pb.User {
    return &pb.User{
        Name: "Unknown",
        Email: "unknown@example.com",
        Address: &pb.Address{
            Street: "Unknown",
            City: "Unknown",
            Country: "Unknown",
        },
    }
}

func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    user := newDefaultUser()
    // Populate user from database...
    return &pb.Response{User: user}, nil
}
```

### Pattern 2: Early Returns with Defaults

```go
func (s *Service) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    user, err := s.db.GetUser(req.UserId)
    if err != nil {
        // Return default instead of nil
        return &pb.Response{
            User: newDefaultUser(),
        }, nil
    }
    
    return &pb.Response{User: user}, nil
}
```

### Pattern 3: Safe List Construction

```go
func (s *Service) ListUsers(ctx context.Context, req *pb.Request) (*pb.ListResponse, error) {
    dbUsers, err := s.db.ListUsers()
    if err != nil {
        return &pb.ListResponse{
            Users: []*pb.User{}, // Empty slice instead of nil
            Total: 0,
        }, nil
    }
    
    // Convert with validation
    users := make([]*pb.User, 0, len(dbUsers))
    for _, dbUser := range dbUsers {
        if dbUser != nil {  // Skip nil items
            users = append(users, convertToProto(dbUser))
        }
    }
    
    return &pb.ListResponse{
        Users: users,
        Total: int32(len(users)),
    }, nil
}
```

## Proto3 Optional Fields

If your proto file uses the `optional` keyword (proto3 syntax), those fields CAN be nil:

```protobuf
message User {
  string name = 1;
  optional string nickname = 2;  // Can be nil
  Address address = 3;            // Cannot be nil
}
```

The linter automatically detects optional fields and allows nil for them:

```go
// This is OK
return &pb.Response{
    User: &pb.User{
        Name: "John",
        Nickname: nil,  // ✓ OK - optional field
        Address: &pb.Address{},
    },
}, nil
```

## Development Workflow

If you're contributing or customizing the linter:

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run all verification
make verify

# Build and test quickly
make quick
```

## Troubleshooting

### Issue: "undefined: newProtoAnalyzer"

**Cause:** Missing dependencies or build issues

**Solution:**
```bash
make clean
make deps
make build
```

### Issue: False positives

**Cause:** The linter might not correctly identify optional fields in some cases

**Solution:**
1. Check if your proto field uses the `optional` keyword
2. Verify the struct tags in generated code contain "optional"
3. Open an issue with your proto definition and generated code

### Issue: Not detecting issues in my code

**Cause:** Your functions might not match the gRPC handler signature

**Solution:**
The linter only checks functions with this exact signature:
```go
func (s *Service) MethodName(ctx context.Context, req *RequestProto) (*ResponseProto, error)
```

Verify your methods match this pattern.

## Getting Help

- 📖 **Documentation**: See [README.md](README.md) and [ARCHITECTURE.md](ARCHITECTURE.md)
- 🐛 **Issues**: Report bugs on GitHub Issues
- 💬 **Discussions**: Ask questions in GitHub Discussions
- 📧 **Email**: Contact maintainers for security issues

## Next Steps

- Read the [Architecture Documentation](ARCHITECTURE.md) to understand how the linter works
- Check out the [example code](testdata/src/example/) for more patterns
- Contribute! See open issues for ways to help improve the tool

## Example Projects

Check out these example projects using proto-nil-linter:

- [Example User Service](testdata/src/example/) - Demonstrates all detection patterns
- More examples coming soon!

## Performance Tips

For large codebases:

1. **Run on specific packages**: `proto-nil-linter ./services/...` instead of `./...`
2. **Use in CI only**: Run full linting in CI, quick checks locally
3. **Parallel execution**: The linter automatically uses multiple cores
4. **Incremental mode**: (Future feature) Only analyze changed files

## FAQ

**Q: Does this replace general nil checkers like nilaway?**
A: No, it's complementary. Use nilaway for general nil safety and proto-nil-linter specifically for gRPC response validation.

**Q: Can I use this with proto2?**
A: The linter is designed for proto3, but most checks work with proto2 as well.

**Q: What about streaming RPCs?**
A: Currently, the linter focuses on unary RPCs.

**Q: Can I configure which fields to check?**
A: For now, all non-optional fields are checked.