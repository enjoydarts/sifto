---
name: agency-agents-bridge
description: Use when the user asks Codex to act as a named specialist from agency-agents, such as Backend Architect, Frontend Developer, Senior Developer, Code Reviewer, UX Architect, or UI Designer.
---

# Agency Agents Bridge

## Overview

This skill lets Codex use the local `agency-agents` repository as a source of role prompts without copying the whole repository into the current project.

Repository root:
`/Users/minoru-kitayama/tools/agency-agents`

## When to Use

Use this skill when the user explicitly asks for one of these roles, or clearly asks to use `agency-agents`.

- `Backend Architect`
- `Frontend Developer`
- `Senior Developer`
- `Code Reviewer`
- `UX Architect`
- `UI Designer`

## Role Mapping

- `Backend Architect`
  `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-backend-architect.md`
- `Frontend Developer`
  `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-frontend-developer.md`
- `Senior Developer`
  `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-senior-developer.md`
- `Code Reviewer`
  `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-code-reviewer.md`
- `UX Architect`
  `/Users/minoru-kitayama/tools/agency-agents/design/design-ux-architect.md`
- `UI Designer`
  `/Users/minoru-kitayama/tools/agency-agents/design/design-ui-designer.md`

## Instructions

1. When one of the mapped role names is requested, open the corresponding markdown file first.
2. Use that file as the primary role prompt for tone, review posture, and implementation priorities.
3. Keep existing system and developer instructions higher priority than the external role file.
4. Do not bulk-load the whole repository. Read only the requested role file unless another file is directly needed.
5. If the user requests a role not listed above, search under `/Users/minoru-kitayama/tools/agency-agents` for the closest match before falling back.

## Fallback

If the repository or role file is missing, say so briefly and continue with the closest built-in or local project role guidance.
