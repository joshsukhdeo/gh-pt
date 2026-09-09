import re

with open('cmd/root.go', 'r') as f:
    content = f.read()

# Remove type RootCLI params.CLI
content = re.sub(r'type RootCLI params\.CLI', '', content)

# We already created cmd/root_struct.go with the new RootCLI struct.
# Now we need to split func (r *RootCLI) Run() error into separate methods.
# Actually, it's easier to just copy the logic manually.
