# granola-mcp

![Coverage](coverage-badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/granola-mcp)](https://goreportcard.com/report/github.com/felixgeelhaar/granola-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/felixgeelhaar/granola-mcp)](https://github.com/felixgeelhaar/granola-mcp/releases)

Granola meeting intelligence for the MCP ecosystem. A Go CLI and MCP server that exposes [Granola](https://granola.ai) meetings, transcripts, summaries, and action items as structured MCP resources and tools — enabling AI agents to reason over meeting knowledge.

## Features

- **MCP Server** — Typed tools and resources for meetings, transcripts, action items, notes, and embeddings
- **CLI** — Authenticate, sync, search, export, annotate, and manage meetings from the terminal
- **Write-Back** — Agent-generated notes and action item updates persisted locally with outbox pattern for future upstream sync
- **Embedding Export** — Chunk meeting content by speaker turn, time window, or token limit and export as JSONL
- **Agent Policies** — Per-meeting ACL (allow/deny by tool + tags) and content redaction (emails, speakers, keywords, patterns)
- **Resilient** — Circuit breaker, retry with backoff, rate limiting, and timeouts on every API call via [Fortify](https://github.com/felixgeelhaar/fortify)
- **Cached** — SQLite local cache reduces API calls and enables offline access
- **Multi-Workspace** — Query meetings across multiple Granola workspaces
- **Event Streaming** — Real-time meeting events via domain event dispatcher
- **Webhook Support** — Push-based sync with HMAC-SHA256 signature validation

## Installation

### Homebrew

```bash
brew tap felixgeelhaar/tap
brew install granola-mcp
```

### Go install

```bash
CGO_ENABLED=1 go install github.com/felixgeelhaar/granola-mcp/cmd/granola-mcp@latest
```

> CGO is required for the SQLite driver.

### From source

```bash
git clone https://github.com/felixgeelhaar/granola-mcp.git
cd granola-mcp
make build
```

## Quick Start

```bash
# Authenticate with Granola
export GRANOLA_MCP_GRANOLA_API_TOKEN=gra_xxxxx
granola-mcp auth login --method api_token

# List recent meetings
granola-mcp list meetings

# Export a meeting as markdown
granola-mcp export meeting <meeting-id> --format md

# Add an agent note to a meeting
granola-mcp note add <meeting-id> "Key insight from analysis"

# Export meeting chunks for embedding
granola-mcp export embeddings --meetings <id1>,<id2> --strategy speaker_turn

# Start as MCP server (stdio, for Claude Code)
granola-mcp serve
```

## CLI Commands

```
granola-mcp
  auth
    login         Authenticate with Granola (--method oauth|api_token)
    status        Show current authentication status
  list
    meetings      List meetings (--format table|json, --source, --limit, --since, --until)
  export
    meeting       Export a meeting (--format json|md|text)
    embeddings    Export meeting chunks as JSONL (--meetings, --strategy, --max-tokens)
  note
    add           Add an agent note to a meeting
    list          List agent notes for a meeting (--format table|json)
    delete        Delete an agent note
  action
    complete      Mark an action item as completed
    update        Update an action item's text
  sync            Sync meetings from Granola API (--since)
  serve           Start MCP server on stdio
  version         Show version information
```

## MCP Server

When running as an MCP server (`granola-mcp serve`), the following tools and resources are exposed:

### Tools

| Tool | Description |
|------|-------------|
| `list_meetings` | Search and filter meetings with date, source, and text filters |
| `get_meeting` | Get full meeting details including summary and action items |
| `get_transcript` | Get the transcript with speaker utterances |
| `search_transcripts` | Full-text search across all meeting transcripts |
| `get_action_items` | Get action items from a specific meeting |
| `meeting_stats` | Aggregated meeting statistics with interactive D3.js dashboard |
| `list_workspaces` | List all Granola workspaces |
| `add_note` | Add an agent note to a meeting |
| `list_notes` | List agent notes for a meeting |
| `delete_note` | Delete an agent note |
| `complete_action_item` | Mark an action item as completed |
| `update_action_item` | Update an action item's text |
| `export_embeddings` | Export meeting content as chunks for embedding generation |

### Resources

| URI Pattern | Description |
|-------------|-------------|
| `meeting://{id}` | Full meeting details as JSON |
| `transcript://{meeting_id}` | Transcript utterances as JSON |
| `note://{meeting_id}` | Agent notes for a meeting as JSON |
| `workspace://{id}` | Workspace details as JSON |
| `ui://meeting-stats` | Interactive meeting statistics dashboard (HTML) |

### Claude Code Integration

Add to your Claude Code MCP configuration (`~/.claude/mcp.json`):

```json
{
  "mcpServers": {
    "granola": {
      "command": "granola-mcp",
      "args": ["serve"],
      "env": {
        "GRANOLA_MCP_GRANOLA_API_TOKEN": "gra_xxxxx"
      }
    }
  }
}
```

## Agent Policies

Control what data AI agents can access and how sensitive content is handled. Create a YAML policy file and set the `GRANOLA_MCP_POLICY_FILE` environment variable:

```yaml
default_effect: allow
rules:
  - name: block-confidential-transcripts
    effect: deny
    tools: [get_transcript, export_embeddings]
    conditions:
      meeting_tags: [confidential]

redaction:
  enabled: true
  rules:
    - type: emails
      replacement: "[EMAIL]"
    - type: speakers
      replacement: "Speaker {n}"
    - type: keywords
      keywords: [salary, confidential]
      replacement: "[REDACTED]"
    - type: patterns
      pattern: '\d{3}-\d{2}-\d{4}'
      replacement: "[SSN]"
```

**ACL** — First-match-wins rule evaluation. Deny rules block tool execution for meetings matching tag conditions.

**Redaction** — Applied to all tool responses. Emails replaced by regex, speakers anonymized consistently (same person always maps to same "Speaker N"), keywords matched case-insensitively with word boundaries, custom regex patterns supported.

## Configuration

Configuration uses 12-factor principles: sensible defaults with environment variable overrides.

| Variable | Default | Description |
|----------|---------|-------------|
| `GRANOLA_MCP_GRANOLA_API_URL` | `https://api.granola.ai` | Granola API base URL |
| `GRANOLA_MCP_GRANOLA_API_TOKEN` | — | API token for authentication |
| `GRANOLA_MCP_MCP_TRANSPORT` | `stdio` | MCP transport (`stdio` or `http`) |
| `GRANOLA_MCP_MCP_HTTP_PORT` | `8080` | HTTP port when using HTTP transport |
| `GRANOLA_MCP_CACHE_TTL` | `15m` | Local cache time-to-live |
| `GRANOLA_MCP_LOGGING_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `GRANOLA_MCP_LOGGING_FORMAT` | `console` | Log format (`console` or `json`) |
| `GRANOLA_MCP_WEBHOOK_SECRET` | — | HMAC secret for webhook signature validation |
| `GRANOLA_MCP_POLICY_FILE` | — | Path to YAML policy file (enables ACL + redaction) |

## Architecture

The project follows strict Domain-Driven Design with hexagonal architecture:

```
cmd/granola-mcp/main.go              Composition root (DI wiring)

internal/
  domain/                             Pure business logic, zero dependencies
    meeting/                          Meeting aggregate, value objects, events, chunks
    annotation/                       Agent notes bounded context
    policy/                           ACL rules, redaction config value objects
    auth/                             Token, credential, auth service port
    workspace/                        Workspace aggregate

  application/                        Use cases (one per file)
    meeting/                          ListMeetings, GetMeeting, CompleteActionItem, ...
    annotation/                       AddNote, ListNotes, DeleteNote
    embedding/                        ExportEmbeddings, chunking strategies
    auth/                             Login, CheckStatus
    workspace/                        ListWorkspaces, GetWorkspace
    export/                           ExportMeeting

  infrastructure/                     External adapters
    granola/                          Granola API client + repository (anti-corruption layer)
    resilience/                       Fortify: circuit breaker, retry, rate limit, timeout
    cache/                            SQLite local cache (repository decorator)
    localstore/                       SQLite local store for notes + action item overrides
    outbox/                           Outbox dispatcher for write events
    policy/                           YAML loader, redaction engine
    events/                           Domain event dispatcher + MCP notifier
    sync/                             Background polling sync manager
    webhook/                          HMAC-SHA256 webhook handler
    auth/                             File-based token storage
    config/                           12-factor configuration

  interfaces/                         Inbound adapters
    mcp/                              MCP server with tools, resources, policy middleware
    cli/                              CLI commands (cobra)
```

### Decorator Chain

```
Read path:   Granola API → Resilient Repo → Cached Repo → Use Cases
Write path:  Use Cases → Local SQLite Store → Outbox Dispatcher → Event Dispatcher
```

### Key Libraries

| Library | Purpose |
|---------|---------|
| [felixgeelhaar/mcp-go](https://github.com/felixgeelhaar/mcp-go) | MCP server framework with typed tools, resources, and multi-transport |
| [felixgeelhaar/fortify](https://github.com/felixgeelhaar/fortify) | Resilience patterns: circuit breaker, retry, rate limit, timeout |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |
| [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | SQLite driver for local cache and local store |
| [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | YAML parsing for policy files |

## Development

```bash
# Run tests (requires CGO for SQLite)
make test

# Run tests with race detection and coverage
make test-race

# Build
make build

# Lint
make lint

# Clean
make clean
```

### Test Coverage

377 tests with 0 race conditions across all packages:

| Layer | Coverage |
|-------|----------|
| Domain (meeting, annotation, policy, auth, workspace) | 94-100% |
| Application (meeting, annotation, embedding, auth, workspace) | 86-100% |
| Infrastructure (granola, resilience, cache, policy, localstore, outbox) | 85-97% |
| Interfaces (mcp, cli) | 74-81% |

## License

[MIT](LICENSE)
