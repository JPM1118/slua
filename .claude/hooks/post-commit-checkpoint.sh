#!/bin/bash
# Auto-create Sprite checkpoint after successful git commits.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')
EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_response.exit_code // empty')

# Only trigger on successful git commit commands
if echo "$COMMAND" | grep -q "git commit" && [ "$EXIT_CODE" = "0" ]; then
  HASH=$(git log -1 --format=%h 2>/dev/null)
  MSG=$(git log -1 --format=%s 2>/dev/null)
  sprite checkpoint create -comment "commit: $HASH $MSG" >/dev/null 2>&1 || true
fi
