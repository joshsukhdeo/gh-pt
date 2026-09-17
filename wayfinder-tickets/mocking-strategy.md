# Mocking Strategy

Labels: `wayfinder:grilling`
Status: Open
Blocked By: `framework-selection.md`

## Question

How should we handle mocking for hundreds of parameter combinations?

Currently, `e2e_test.go` mocks the `gh` CLI by writing a temporary bash script and injecting it into `$PATH`. 
If we test all parameters, we need to mock different GitHub API responses (e.g., releases, assets, prereleases), different state files (`state.json`), and different config files.

Options:
1. **Dynamic Bash Mocks**: Pass an environment variable to the `gh` mock script to change its behavior per test case.
2. **Go-Based Mock Server/Binary**: Instead of a bash script, compile a minimal Go binary to act as `gh` that reads expected responses from a JSON file.
3. **Internal HTTP Client Mocking**: Instead of mocking the `gh` CLI executable, expose a way to inject a mock HTTP client into `go-gh` for testing (requires modifying `gh-pt` source code to accept dependency injection).

Which approach should we take?
