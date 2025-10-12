# Architecture Documentation

## Overview

The proto-nil-linter is a Go static analysis tool built on the `golang.org/x/tools/go/analysis` framework. It performs multi-pass analysis to detect nil assignments to non-optional proto3 message fields within gRPC service handlers.

## Design Philosophy

The linter follows a **focused, domain-specific approach** rather than attempting general nil safety analysis. This allows for:

1. **Higher accuracy** - Fewer false positives due to understanding proto3 semantics
2. **Better error messages** - Context-aware diagnostics specific to gRPC/proto3
3. **Simpler implementation** - Narrower scope than general nil checkers like nilaway
4. **Faster execution** - Targeted analysis only where needed

## System Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                  Analysis Orchestrator                  │
│                    (analyzer.go)                        │
│                                                         │
│  Coordinates the multi-pass analysis process:          │
│  1. Proto message identification                       │
│  2. gRPC handler detection                            │
│  3. Nil assignment checking                           │
└──────────────┬──────────────────────────────────────────┘
               │
               ├──────────────────────────────────────────┐
               │                                          │
               ▼                                          ▼
┌──────────────────────────┐              ┌──────────────────────────┐
│   Proto Analyzer         │              │    gRPC Analyzer         │
│     (proto.go)           │              │      (grpc.go)           │
│                          │              │                          │
│ • Identifies proto msgs  │              │ • Detects handlers       │
│ • Extracts field info    │◄─────────────┤ • Validates signatures   │
│ • Tracks optionality     │              │ • Maps return types      │
│ • Handles nested types   │              │                          │
└──────────────┬───────────┘              └──────────┬───────────────┘
               │                                     │
               │                                     │
               └─────────────┬───────────────────────┘
                             │
                             ▼
                 ┌───────────────────────┐
                 │    Nil Checker        │
                 │   (nilcheck.go)       │
                 │                       │
                 │ • Detects patterns    │
                 │ • Tracks nil flow     │
                 │ • Validates nested    │
                 │ • Checks repeated     │
                 │ • Reports issues      │
                 └───────────────────────┘
```

## Analysis Passes

### Pass 1: Proto Message Identification

**File:** [`proto.go`](pkg/analyzer/proto.go)

**Purpose:** Build a complete map of all proto3 message types and their field characteristics.

**Process:**
1. **Traverse AST** to find struct type definitions
2. **Identify proto messages** by looking for proto-generated markers:
   - `protoimpl.MessageState` embedded field
   - Special fields: `state`, `sizeCache`, `unknownFields`
   - `XXX_` prefixed fields
3. **Extract field information** for each message:
   - Field name and type
   - Optional vs required (via proto tags)
   - Repeated (slice) vs singular
   - Nested message vs scalar
   - Type name for nested messages
4. **Build type cache** for fast lookup during nil checking

**Data Structures:**
```go
type protoMessageInfo struct {
    typeName string
    fields   map[string]*protoFieldInfo
}

type protoFieldInfo struct {
    name       string
    typeName   string
    isOptional bool
    isRepeated bool
    isMessage  bool
    nestedType string
}
```

**Key Challenges:**
- **Heuristic detection**: Proto-generated code doesn't have explicit markers, so we use structural patterns
- **Type resolution**: Must handle both simple (`User`) and qualified (`pb.User`) type names
- **Optional detection**: Parse protobuf struct tags to identify optional fields

### Pass 2: gRPC Handler Detection

**File:** [`grpc.go`](pkg/analyzer/grpc.go)

**Purpose:** Identify which functions are gRPC service handler methods.

**Handler Signature:**
```go
func (s *Service) MethodName(
    ctx context.Context,
    req *RequestType,
) (*ResponseType, error)
```

**Detection Criteria:**
1. ✅ Has a receiver (is a method)
2. ✅ Two parameters: `context.Context` and `*ProtoMessage`
3. ✅ Two return values: `*ProtoMessage` and `error`

**Process:**
1. **Traverse function declarations** in the AST
2. **Validate signature** against gRPC handler pattern
3. **Extract response type** information
4. **Store handler metadata** for use in nil checking pass

**Data Structures:**
```go
type handlerInfo struct {
    funcDecl     *ast.FuncDecl
    responseType types.Type
    responseName string
    isHandler    bool
}
```

**Why This Matters:**
- Only checks functions that are actually gRPC handlers
- Reduces false positives by focusing on relevant code
- Provides context for better error messages

### Pass 3: Nil Assignment Detection

**File:** [`nilcheck.go`](pkg/analyzer/nilcheck.go)

**Purpose:** Find and report all nil assignments to non-optional proto fields within handler contexts.

**Detection Patterns:**

#### 1. Direct Field Assignment
```go
resp := &pb.Response{}
resp.User = nil  // ❌ DETECTED
```

**Detection Method:**
- Monitor `ast.AssignStmt` nodes
- Check if LHS is a field selector (`resp.User`)
- Check if RHS is nil
- Verify field is non-optional in proto message

#### 2. Composite Literal
```go
return &pb.Response{
    User: nil,  // ❌ DETECTED
}, nil
```

**Detection Method:**
- Monitor `ast.CompositeLit` nodes
- Extract the type being constructed
- Check each key-value pair
- Verify non-optional fields aren't set to nil

#### 3. Variable Data Flow
```go
var user *pb.User = nil
resp.User = user  // ❌ DETECTED (tracked from nil assignment)
```

**Detection Method:**
- Track variables assigned to nil in `nilVars` map
- Check if identifiers in assignments reference nil variables
- Clear tracking when variable is reassigned

#### 4. Nested Message Fields
```go
return &pb.Response{
    User: &pb.User{
        Address: nil,  // ❌ DETECTED (nested validation)
    },
}
```

**Detection Method:**
- Recursively validate nested message structures
- Check fields within nested composite literals
- Track nesting depth to prevent infinite recursion

#### 5. Repeated Fields (Lists)
```go
return &pb.ListResponse{
    Users: []*pb.User{
        {Name: "Alice"},
        nil,  // ❌ DETECTED (nil item in list)
    },
}
```

**Detection Method:**
- Identify repeated fields (slice types)
- Iterate through slice elements
- Check each element for nil values
- If elements are messages, recursively validate nested fields

## Algorithm Deep Dive

### Nested Message Validation

**Challenge:** Proto messages can be arbitrarily nested, requiring recursive validation.

**Solution:** `checkNestedMessage(expr, typeName)` function performs depth-first traversal:

```
checkNestedMessage(Message, "User")
├── Extract composite literal
├── Get message info for "User"
├── For each field in literal:
│   ├── Check if field value is nil
│   ├── If field is repeated:
│   │   └── checkRepeatedField()
│   └── If field is a message:
│       └── checkNestedMessage() [RECURSIVE]
└── Report any violations
```

**Termination:** Recursion stops when:
- No more nested messages found
- Scalar type reached
- Nil value detected (reported)

### Data Flow Analysis

**Current Implementation:**
- **Intraprocedural**: Tracks nil flow within a single function
- **Variable tracking**: Maintains `nilVars` map per function
- **Assignment sensitivity**: Updates tracking on each assignment

**Example:**
```go
var user *pb.User = nil    // nilVars["user"] = true
// ... later ...
resp.User = user           // DETECTED: user is in nilVars
// ... later ...  
user = &pb.User{}          // nilVars["user"] deleted
resp.User = user           // OK: user no longer nil
```

**Future Enhancement:**
Could be extended to interprocedural analysis using SSA form (like nilaway) to track nil across function calls.

## Error Reporting

### Diagnostic Message Format

```
<file>:<line>:<col>: <severity> <message>
    <code context>
    <pointer to issue>
<detailed explanation>
```

**Example:**
```
service/user_service.go:32:15: nil assignment to non-optional proto field GetUserResponse.User in direct assignment (gRPC handler response must not contain nil)
    resp.User = nil
              ^^^^
```

### Severity Levels

All violations are reported as **errors** because:
- Nil in gRPC responses causes runtime panics
- No legitimate reason to have nil in non-optional fields
- High confidence in detection accuracy

## Performance Characteristics

### Time Complexity

- **Pass 1 (Proto analysis)**: O(n) where n = number of struct definitions
- **Pass 2 (gRPC detection)**: O(m) where m = number of functions
- **Pass 3 (Nil checking)**: O(k × d) where k = handler body size, d = nesting depth

**Overall**: O(n + m + k×d) - linear with codebase size

### Space Complexity

- **Proto message cache**: O(p × f) where p = proto messages, f = avg fields per message
- **Handler map**: O(h) where h = number of handlers
- **Nil tracking**: O(v) where v = variables per function

**Overall**: O(p×f + h + v) - linear with analyzed code

### Optimization Opportunities

1. **Lazy loading**: Only analyze proto files when needed
2. **Incremental analysis**: Cache results for unchanged files
3. **Parallel processing**: Analyze multiple packages concurrently
4. **Pruning**: Skip non-gRPC packages early

## Integration Points

### As a Standalone Tool

```bash
proto-nil-linter ./...
```

Uses `singlechecker.Main()` to run as a standalone analyzer.

### With `go vet`

```bash
go vet -vettool=$(which proto-nil-linter) ./...
```

Integrates into standard Go tooling via `analysis.Analyzer` interface.

### In CI/CD

```yaml
- run: proto-nil-linter ./...
```

Exit code indicates pass/fail for CI integration.

### IDE Integration

Future: Language Server Protocol support for real-time linting in editors.

## Testing Strategy

### Unit Tests

**File:** [`analyzer_test.go`](pkg/analyzer/analyzer_test.go)

Uses `analysistest.Run()` to test against fixture code with `// want` comments.

**Test Cases:**
- Direct nil assignments
- Composite literal nils
- Variable flow tracking
- Nested message validation
- Repeated field validation
- Conditional nil assignments
- Edge cases and false positives

### Test Fixtures

**Directory:** `testdata/src/example/`

Contains:
- **Proto definitions** (`proto/user.pb.go`) - Generated proto code
- **Service implementations** (`service/user_service.go`) - Test cases with annotations

**Annotation Format:**
```go
resp.User = nil // want "nil assignment to non-optional proto field"
```

## Comparison with nilaway

| Aspect | nilaway | proto-nil-linter |
|--------|---------|------------------|
| **Scope** | All nil panics in Go code | Only gRPC proto responses |
| **Analysis Type** | Full interprocedural SSA | Targeted AST + limited dataflow |
| **Annotations** | Requires function contracts | Auto-detects from proto tags |
| **False Positives** | Can be high (general case) | Low (domain-specific) |
| **Performance** | Slower (comprehensive) | Faster (focused) |
| **Setup** | Complex configuration | Minimal setup |
| **Use Case** | General nil safety | API contract validation |

## Future Enhancements

### Phase 1: Current Implementation ✅
- [x] Basic nil detection
- [x] Proto message analysis
- [x] gRPC handler detection
- [x] Nested message validation
- [x] Repeated field validation

### Phase 2: Advanced Analysis
- [ ] Interprocedural data flow using SSA
- [ ] Path-sensitive analysis for conditionals
- [ ] Null-contract annotations for helper functions
- [ ] Support for proto optional keyword

### Phase 3: User Experience
- [ ] Configuration file support (`.proto-nil-linter.yaml`)
- [ ] Auto-fix suggestions
- [ ] IDE integration (LSP)
- [ ] Custom rule configuration

### Phase 4: Performance
- [ ] Incremental analysis
- [ ] Parallel package processing
- [ ] Result caching
- [ ] Smart pruning

## Contributing

When adding new detection patterns:

1. **Add test case** in `testdata/`
2. **Implement detection** in appropriate analyzer
3. **Write unit test** with `// want` annotation
4. **Update documentation** in README and ARCHITECTURE
5. **Benchmark** if performance-sensitive

## References

- [Go Analysis Framework](https://pkg.go.dev/golang.org/x/tools/go/analysis)
- [Uber's nilaway](https://github.com/uber-go/nilaway)
- [nilaway blog post](https://www.uber.com/en-DE/blog/nilaway-practical-nil-panic-detection-for-go/)
- [Protocol Buffers - Go](https://protobuf.dev/getting-started/gotutorial/)
- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)