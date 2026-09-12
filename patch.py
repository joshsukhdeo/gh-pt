import sys

with open('cmd/router.go', 'r') as f:
    lines = f.readlines()

for i, line in enumerate(lines):
    if 'return RemoveApp(cli.Rm.Target, cli.Rm.Purge)' in line:
        insert_idx = i
        break

code_to_insert = """
			if cli.Rm.StateOnly {
				return RmStateOnly(cli.Rm.Target)
			}
"""

lines.insert(insert_idx, code_to_insert)

with open('cmd/router.go', 'w') as f:
    f.writelines(lines)
