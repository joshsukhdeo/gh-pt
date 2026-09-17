# Framework Selection

Labels: `wayfinder:grilling`
Status: Open
Blocked By: None

## Question

How should we scale the integration test suite to handle "all params and combinations"? 

Currently, `e2e_test.go` builds the binary and uses standard Go `testing` functions with manual `exec.Command` calls.
Options:
1. **Extend Table-Driven Tests in `e2e_test.go`**: Keep the current approach but build a massive table-driven matrix. (Pros: No new dependencies. Cons: Verbose, hard to read test failures.)
2. **`go-testscript`**: Use the script-based testing framework used by the Go toolchain itself for CLI testing (e.g., `testscript.Run(t, testscript.Params{...})`). (Pros: Highly readable `txtar` files, great for CLI I/O testing. Cons: New dependency, requires learning a mini-language.)
3. **Property-Based Testing (e.g., `gopter`)**: Generate random combinations of flags to fuzz the CLI routing. (Pros: True combinatorial coverage. Cons: Overkill for most CLI routing, difficult to assert specific output strings.)

Which approach should we use?
