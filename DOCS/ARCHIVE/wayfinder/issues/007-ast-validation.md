## Question

Define AST-based validation replacing regex validation for body.sh using mvdan.cc/sh parser.

## Decision

**Decision**: Replace regex validation with AST-based bash parser (mvdan.cc/sh).

**Rationale**:
- Regex validation trivially bypassable (base64, heredocs, obfuscation)
- AST parser understands bash semantics, not just text patterns
- Can validate actual command structure, not just text patterns
- More precise: allows legitimate code, blocks malicious patterns

**Implementation**:
- Use mvdan.cc/sh (Go bash parser)
- Parse body.sh into AST
- Walk AST, check against allowlist:
  - Allowed: build commands (cmake, make, gcc, etc.)
  - Allowed: ghpt helper --install
  - Forbidden: network commands (curl, wget, ssh)
  - Forbidden: privilege escalation (sudo, su)
  - Forbidden: shell expansions (eval, exec)
  - Forbidden: obfuscation (base64, hex escapes)

**Implementation**:
```go
import "mvdan.cc/sh/v3/syntax"
parser := syntax.NewParser()
file, err := parser.Parse(strings.NewReader(bodyContent), "")
if err != nil {
    return err
}
walker := func(n syntax.Node) bool {
    // Check node against allowlist
    return true
}
syntax.Walk(file, walker)
```

**Validation**:
- Parser correctly identifies all commands
- Allowlist blocks forbidden patterns
- Obfuscated code detected and blocked
- Build tools allowed but monitored