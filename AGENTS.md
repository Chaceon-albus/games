# AGENTS.md

## Scope

These instructions apply to the entire repository. A more deeply nested `AGENTS.md`, if one is added later, may provide more specific rules for its subtree.

## Project Overview

Crystal Games is a small multiplayer game platform with a Go backend, two independent Vue frontends, and a Docker/Caddy deployment layer.

- `backend/` contains the Go 1.26.3 HTTP and WebSocket server. Gin provides HTTP routing, and Melody manages WebSocket sessions.
- `backend/cmd/server/` contains the executable entry point. Debug mode is the default; the `release` build tag enables Gin release mode.
- `backend/internal/gomoku/` contains the in-memory player registry, room state, Gomoku rules, protocol payloads, and connection lifecycle.
- `frontend/lobby/` is the lightweight Vue 3 and Vite landing page.
- `frontend/gomoku/` is the Vue 3, TypeScript, Nuxt UI, Tailwind CSS, and Vue Router game client.
- `deployment/` contains the multi-stage frontend/Caddy image, Caddy routing and rate limiting, and interactive deployment scripts.
- `docker-compose.yml` runs the backend and the Caddy-based frontend ingress together.

There is currently no database. Players, rooms, matches, and sessions are process-local and are lost when the backend restarts.

## Mandatory Language and Comment Policy

- Write all source-code comments in English.
- This rule includes Go documentation comments, inline comments, `TODO` and `FIXME` notes, Vue template comments, TypeScript comments, CSS comments, shell and batch comments, Dockerfile comments, Caddy comments, and comments in YAML or other configuration files.
- Do not add Chinese or other non-English comments. When modifying code near an existing non-English comment, translate that comment to clear English as part of the same change.
- Use comments to explain intent, invariants, compatibility constraints, or non-obvious tradeoffs. Do not narrate code that is already self-explanatory.
- User-facing text is not a code comment. The Chinese game UI, chat messages, and localized server messages may remain Chinese unless the task explicitly changes product copy or localization.
- Keep identifiers, API documentation, log keys, commit-ready documentation, and developer-facing error text in English unless an external protocol requires otherwise.

## Repository Working Rules

- Keep changes focused. Do not reformat or rewrite unrelated files.
- Preserve unrelated local and untracked work. Inspect `git status` and the relevant diff before finishing.
- Do not commit secrets or a real `.env` file. Add new deployment settings to `.env.example` with safe sample values.
- Treat `frontend/lobby/` and `frontend/gomoku/` as separate pnpm projects. Run package commands from the package being changed.
- Change a package's lockfile only when its `package.json` or resolved dependency graph changes.
- Do not edit or commit generated artifacts such as `node_modules/`, `dist/`, TypeScript build metadata, generated auto-import declarations, coverage files, or compiled binaries.
- Run `go mod tidy` only when Go dependencies change, and review both `go.mod` and `go.sum` afterward.

## Git and Commit Workflow

- Do not leave a large, undifferentiated set of agent-owned changes in the working tree after completing a task.
- Divide substantial work into independent, coherent units. After each unit is implemented and validated, proactively create a local commit before starting the next unit.
- A commit must represent a complete logical change, not an arbitrary time checkpoint. Do not commit broken builds, knowingly incomplete migrations, debug code, or temporary instrumentation.
- Do not create a separate commit for every tiny edit. Small related edits that form one logical change should stay together.
- Before every commit, inspect `git status` and the staged diff. Stage only the exact files that belong to that unit; never absorb unrelated user changes or untracked files.
- At task completion, the agent-owned work should be committed. Any remaining changes must be pre-existing, user-owned, intentionally excluded, or clearly reported as uncommitted with a concrete reason.
- Never push commits, force-push, publish a branch, or open a pull request unless the user explicitly requests that separate action.
- Do not rewrite, amend, squash, or rebase commits created by the user or another agent. Amend the agent's own latest local commit only when it is clearly part of the same unfinished unit and has not been shared.

Use concise English commit messages. Prefer Conventional Commit syntax while retaining the repository's short, imperative style:

```text
<type>(optional-scope): <imperative summary>
```

- Use a lowercase type such as `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `build`, `ci`, `perf`, or `style`.
- Keep the summary specific, imperative, and normally no longer than 72 characters. Do not end it with a period.
- Describe the user-visible or architectural outcome, not the editing process. Avoid vague summaries such as `update files`, `misc changes`, or `fix stuff`.
- Add a body when the reason, compatibility impact, migration, or non-obvious tradeoff needs explanation. Explain why the change was made and wrap body lines at approximately 72 characters.
- Add a footer for breaking changes or tracked issue references when applicable.

Examples:

```text
feat(gomoku): add spectator auto-join controls
fix(backend): preserve seats during reconnect grace period
docs: add repository agent workflow
```

## Architecture and Compatibility Boundaries

### Backend

- Keep the backend authoritative. Validate membership, role, turn order, coordinates, occupied cells, room permissions, and game status on the server even when the UI already prevents an action.
- The server currently owns the 15-by-15 board, move history, win detection, draw detection, readiness, color assignment, retract requests, resignation, spectator promotion, and reconnect handling.
- Preserve the established lock ownership model: `Manager.mu` protects manager maps and session bookkeeping, `Room.mu` protects mutable room state, and `PlayerRegistry` protects its own player map.
- Never perform slow work, sleeps, or avoidable network writes while holding a mutex. Be careful about lock ordering and calls that acquire another lock.
- Reconnection keeps a playing seat for 60 seconds. Changes to disconnect handling must account for stale timers and successful reconnects.
- Keep Go files formatted with `gofmt`. Follow standard Go naming, error handling, and documentation conventions.
- Keep debug and release behavior in the existing build-tag files. Production builds must continue to use `-tags release`.

### HTTP and WebSocket Protocol

The protocol is a compatibility boundary shared by `backend/internal/gomoku/`, `frontend/gomoku/src/types.ts`, and `frontend/gomoku/src/composables/useGameState.ts`.

- REST routes are rooted at `/api/v1`; Gomoku registration and verification are under `/api/v1/gomoku`.
- The Gomoku WebSocket endpoint is `/ws/gomoku` and requires a UUID returned by registration.
- Active client actions include `create_room`, `join_room`, `leave_room`, `toggle_ready`, `place_stone`, `request_retract`, `retract_respond`, `resign`, `configure_room`, `send_chat`, and `list_rooms`.
- Server message types include `room_list`, `room_state`, `chat_message`, and `error_message`.
- JSON field names use the existing camelCase wire format. Do not rename fields, actions, statuses, routes, or message types on only one side.
- When changing a payload or room field, update the Go structs and validation, the WebSocket handler, TypeScript types, state synchronization, and affected UI in the same change.
- Do not expose private player UUIDs or live WebSocket session objects in room-state JSON.
- Server room statuses and frontend display statuses are intentionally mapped rather than identical. Review `syncRoomState` before changing status or winner semantics.

### Frontends

- Use Vue 3 Composition API and `<script setup lang="ts">` for component logic.
- Keep transport, reconnection, server-state synchronization, and game actions centralized in `frontend/gomoku/src/composables/useGameState.ts` unless there is a clear reason to extract a smaller composable.
- Keep reusable wire and UI shapes in `frontend/gomoku/src/types.ts`. Prefer precise types over new `any` values.
- Treat server room state as the source of truth. Optimistic UI must be reversible and must not bypass server validation.
- Preserve the endpoint resolver in `useGameState.ts`: Vite development talks to port 8080, while production uses the current origin through Caddy.
- Preserve the Gomoku Vite base path `/gomoku/` and Caddy's SPA fallback unless the routing design is intentionally changed across both layers.
- Keep the lobby simple and independent from Gomoku runtime state.
- Follow each frontend's committed Prettier configuration: no semicolons, single quotes, no trailing commas, and a 100-column print width.
- Maintain keyboard usability, visible focus states, disabled states, responsive layouts, and meaningful alternative text when changing the UI.

### Deployment

- Caddy serves the lobby at `/`, serves the Gomoku SPA under `/gomoku`, and proxies `/api/*` and `/ws/*` to the backend.
- The Caddy image includes `github.com/mholt/caddy-ratelimit`; keep the directive ordering compatible with that plugin.
- Keep the Docker build reproducible. Use the committed lockfile with `pnpm install --frozen-lockfile`.
- If ports, paths, service names, health behavior, or environment variables change, update all affected Docker, Caddy, script, and example-environment files together.
- Keep `deployment/run.sh` and `deployment/run.bat` behavior aligned when changing the deployment menu.

## Common Commands

Run backend commands from `backend/`:

```sh
go run ./cmd/server -host 127.0.0.1 -port 8080 -level debug
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build -tags release ./cmd/server
```

For concurrency-sensitive backend changes, also run this when the local Go toolchain supports the race detector:

```sh
go test -race ./...
```

Run frontend commands separately from either `frontend/lobby/` or `frontend/gomoku/`:

```sh
pnpm install --frozen-lockfile
pnpm dev
pnpm format
pnpm build
```

Use the non-writing Prettier check when only verification is needed:

```sh
pnpm exec prettier --check "src/**/*.{ts,vue,css}" "*.html"
```

Run deployment commands from the repository root:

```sh
docker compose config
docker compose up --build
docker compose down
```

The repository does not currently contain a frontend test runner or committed automated tests. Do not claim a test suite passed if only a build was run.

## Validation Expectations

- Backend-only change: run `gofmt` on changed Go files, then `go test ./...` and `go vet ./...`.
- Frontend-only change: run Prettier on changed frontend files and `pnpm build` in every changed frontend package.
- Shared protocol change: run backend checks and the Gomoku frontend build, then verify the affected flow end to end.
- Deployment change: run `docker compose config`; build the affected image when practical.
- Concurrency or game-rule change: add focused table-driven Go tests and run the race detector when supported.
- UI or multiplayer-flow change: manually test with at least two isolated browser sessions. Cover registration, room creation and joining, readiness, legal and illegal moves, win or draw state, retract, resignation, leaving, and reconnect behavior as relevant.

## Before Finishing

- Review `git diff --check`, `git status --short`, and the final scoped diff.
- Confirm that no generated files, secrets, local binaries, or unrelated formatting changes were added.
- Confirm that all new or modified code comments are written in English.
- Report which checks were run and call out any checks that could not be run.
