## Question

Implement AST-based validation replacing regex validation for body.sh using mvdan.cc/sh parser.

## Decision

**Decision**: Replace regex validation with AST-based bash parser (mvdan.cc/sh).

**Implementation**:
```go
import "mvdan.cc/sh/v3/syntax"

func ValidateBodyScript(bodyPath string) error {
    parser := syntax.NewParser()
    file, err := parser.Parse(os.Open(bodyPath), "")
    if err != nil {
        return err
    }
    
    allowedCommands := map[string]bool{
        "cmake": true, "make": true, "gcc": true, "g++": true,
        "ghpt": true, "bash": true, "sh": true,
    }
    
    forbidden := []string{
        "curl", "wget", "ssh", "scp", "nc", "socat",
        "sudo", "su", "pkexec", "doas",
        "eval", "exec", "source", ".",
        "base64", "xxd", "xxd -r",
    }
    
    // Walk AST and check each command
    // ...
}
```

**Files Modified**:
- ai/parser.go: Add AST-based validation
- ai/parser.go: BodyTemplate constant updated
- ai/parser_test.go: Tests updated