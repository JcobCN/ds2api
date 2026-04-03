# Architecture

DS Coder runs two local services:

- `ds2api`: DeepSeek login, token refresh, PoW, OpenAI-compatible chat API
- `kilo` CLI: coding-agent orchestration, tool execution, sessions, worktrees

The VS Code extension starts the local Kilo backend, injects a runtime config via
`KILO_CONFIG_CONTENT`, and points the default provider at:

- provider ID: `ds2api`
- model: `deepseek-chat`
- base URL: `http://127.0.0.1:<port>/v1`

Bootstrap flow:

1. Extension checks `ds2api /healthz`
2. Extension optionally auto-starts `ds2api` from `ds-coder.server.ds2apiPath`
3. Extension calls `POST /plugin/bootstrap`
4. Extension injects the returned plugin API key into Kilo runtime config
5. Kilo serves the Coding Agent UI and tool/session orchestration
