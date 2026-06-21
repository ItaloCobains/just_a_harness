# just_a_harness

A small, dependency-light coding agent harness written in Go. It runs a tool-using
agent loop against a local [Ollama](https://ollama.com) model, with a streaming terminal
chat UI and a single-shot CLI. Built to stay reliable even with smaller local models.

## Features

- **Agent loop with tools** — `read_file`, `list_dir`, `write_file`, `edit_file`,
  `run_bash`, `grep`, `glob`, `web_search` (DuckDuckGo, no key), `web_fetch`, and a
  read-only `task` subagent.
- **Streaming TUI** — markdown rendering, an animated thinking spinner with elapsed time,
  mouse-wheel scrollback, and live token usage (context size, generated tokens, tok/s).
- **Tool approval** — mutating tools (`write_file`, `edit_file`, `run_bash`, `web_fetch`)
  prompt `[y/N/a]` with a colored diff preview; "always allow" persists per project.
- **Session persistence** — conversations are saved to `~/.harness/sessions/` and can be
  resumed (`--resume`, `/resume`, `/sessions`).
- **Context management** — token-based compaction, capped tool output, an injected project
  file tree, and `read_file` line ranges keep small context windows from overflowing.
- **Resilient networking** — connection/header timeout, retry with backoff, an idle-stream
  timeout, and a retry for streams that stall before the first token.
- **Tolerant of stubborn models** — recovers tool calls emitted as text (fenced, unfenced,
  with trailing junk, or after prose), validates required arguments, and breaks tool-call
  loops instead of spinning for 25 turns.

## Requirements

- Go 1.26+
- [Ollama](https://ollama.com) running locally with a tool-capable model pulled:

  ```bash
  ollama pull qwen2.5-coder:7b
  ```

  Tool calling is more reliable on larger models (14B+). To run against one:

  ```bash
  ollama pull qwen3:14b
  ```

## Install

```bash
git clone https://github.com/ItaloCobains/just_a_harness.git
cd just_a_harness
make build   # builds bin/chat and bin/agent
```

## Usage

Interactive chat (TUI):

```bash
make chat
# or pick a model:
make chat MODEL=qwen3:14b
# resume the last conversation:
make chat RESUME=latest
```

Single-shot CLI:

```bash
make agent TASK="list the files and tell me what this project does"
```

Inside the chat, slash commands: `/help`, `/tools`, `/clear`, `/resume [name]`, `/sessions`.

## Configuration

All settings come from the environment, with sensible defaults:

| Variable                   | Default                  | Description                                   |
|----------------------------|--------------------------|-----------------------------------------------|
| `HARNESS_MODEL`            | `qwen2.5-coder:7b`       | Ollama model name                             |
| `HARNESS_ENDPOINT`         | `http://localhost:11434` | Ollama base URL                               |
| `HARNESS_TEMPERATURE`      | `0`                      | Sampling temperature (low steadies tool use)  |
| `HARNESS_HTTP_TIMEOUT`     | `30s`                    | Connection / response-header timeout          |
| `HARNESS_HTTP_MAX_RETRIES` | `3`                      | Retries for transient request failures        |

The `make` targets accept `MODEL`, `ENDPOINT`, and `TEMP` overrides, e.g.
`make chat MODEL=qwen3:14b TEMP=0.1`.

### Ollama in Docker

If Ollama runs in a container, expose its port (`-p 11434:11434`) and the default endpoint
works. For a non-default host port, set `HARNESS_ENDPOINT` accordingly.

## Development

```bash
make test    # run the test suite
make check   # fmt + vet + test
make help    # list all targets
```

## Layout

```
agent/      core agent loop, tool dispatch, compaction
agentkit/   tools, approval, sessions, project context, slash commands
model/ollama Ollama HTTP backend (streaming, retries, tool-call parsing)
config/     environment-based settings
cmd/chat    interactive TUI
cmd/agent   single-shot CLI
internal/   shared terminal helpers
```

## License

[MIT](LICENSE)
