## Question

Implement AST-based validation replacing regex validation for body.sh using mvdan.cc/sh parser.

## Decision

**Decision**: Replace regex validation with AST-based bash parser (mvdan.cc/sh).

**Implementation** (ai/parser.go):
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
- ai/parser.go: Added BodyTemplate, HeaderTemplate, FooterTemplate
- ai/parser.go: Added ValidateBodyScript function
- ai/parser.go: Updated ManifestTemplate
- ai/parser_test.go: Updated tests for new templates