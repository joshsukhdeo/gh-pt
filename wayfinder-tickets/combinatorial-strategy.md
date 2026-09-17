# Combinatorial Strategy

Labels: `wayfinder:grilling`
Status: Open
Blocked By: `framework-selection.md`

## Question

What constitutes "all params and combinations" for this test suite?

The CLI has many flags (e.g., `--global`, `--target-path`, `--allow-root`, `--verbose`, `--log-level`, etc.) and many commands (`install`, `upgrade`, `ls`, `show`, `rm`, etc.). True exhaustive testing of every combination is practically infinite ($O(2^N)$).

Options:
1. **Pairwise Testing (All-Pairs)**: Test every parameter against every other parameter at least once, but not all combinations of 3+.
2. **Context-Specific Matrices**: Group parameters by command (e.g., "Install Flags", "State Flags") and test an exhaustive matrix *within* that group.
3. **Core Scenarios + Edge Cases**: Test the core happy-paths for all commands, plus specific known edge-cases where parameters interact (e.g., `--global` with `--target-path`).

How should we constrain the combinatorial explosion?
