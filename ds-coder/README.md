# DS Coder

`ds-coder/` is the local workspace for the DS Coder VS Code plugin.

Current layout:

- `vendor/kilo/`: vendored Kilo upstream baseline used as the fork source
- `packages/vscode-extension/`: local extension workspace placeholder
- `packages/kilo-cli/`: local CLI/backend workspace placeholder
- `packages/shared/`: shared constants/docs placeholder
- `docs/`: fork notes and integration docs
- `scripts/`: local maintenance scripts

Current implementation approach:

1. Keep the upstream Kilo source in `vendor/kilo/`
2. Patch the vendored VS Code extension to default to `ds2api`
3. Use `ds2api /plugin/*` for local bootstrap and DeepSeek account login
4. Continue extracting a smaller fork into `packages/*` as the CLI/UI surfaces stabilize

## Development Notes

- The vendored extension now expects a local `ds2api` instance.
- Configure `ds-coder.server.ds2apiPath` in VS Code to auto-start a local `ds2api` binary.
- Or keep `ds2api` running separately and point `ds-coder.server.ds2apiBaseUrl` at it.
