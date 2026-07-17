# ai-chat-bot

A Telegram AI-agent bot in Go. It chats with a whitelisted set of users and acts
as a task-solving agent: it answers simple questions immediately, and for longer
tasks it works in the background and replies when done.

Built with **clean / hexagonal architecture**. The only third-party dependencies
are [envconfig](https://github.com/kelseyhightower/envconfig) and
[godotenv](https://github.com/joho/godotenv) for configuration; everything else
is the standard library.

## How it works

```
Telegram user ──▶ Poller ──▶ Handler ──▶ Queue ──▶ Worker ──▶ Agent ──▶ OpenAI
                    (authorize + enqueue)         (background)  (tool loop)
                                                       │
                              reply / "🛠 working…" ◀──┘
```

- **Immediate vs. later.** Every message is queued and processed by a worker.
  Fast answers arrive right away. If a task is still running after
  `ack_after_seconds`, the user gets a *"🛠 Working on it…"* note and the final
  answer follows when ready — so nothing blocks the bot and long tasks are fine.
- **Agent tool loop.** The worker runs an agent that lets the model call tools
  (see `internal/agent/tools`) over several steps to actually *solve* a task
  before replying.
- **Memory.** Bounded per-chat conversation history makes it multi-turn. Send
  `/reset` to clear it.
- **Language.** `/language` shows a 🇬🇧/🇷🇺 button menu. The choice localizes all
  bot messages *and* is passed to the model so it answers in that language.
  Strings live in `internal/i18n`; add a language by extending that table.

### Architecture

| Layer      | Package                    | Responsibility                                   |
|------------|----------------------------|--------------------------------------------------|
| domain     | `internal/domain`          | Pure entities (User, Task, Message). No deps.    |
| port       | `internal/port`            | Interfaces the core depends on.                  |
| usecase    | `internal/usecase`         | Agent loop, request handler, worker pool.        |
| adapter    | `internal/adapter/*`       | Telegram, OpenAI, in-memory queue/repo/history.  |
| auth       | `internal/auth`            | User whitelist.                                  |
| main       | `cmd/bot`                  | Composition root (wires adapters into use cases).|

Dependencies point inward only, so swapping OpenAI for another LLM, or the
in-memory queue for Redis, means writing one new adapter — no core changes.

## Setup

### 1. Create a Telegram bot
Message [@BotFather](https://t.me/BotFather), run `/newbot`, and copy the token.

### 2. Find your Telegram user ID
Message [@userinfobot](https://t.me/userinfobot); it replies with your numeric ID.
Put it (and any other allowed users) in `allowed_user_ids`.

### 3. Get an OpenAI API key
> **Note:** a ChatGPT Pro subscription does **not** include API access. The API
> is billed separately. Create a key at <https://platform.openai.com/api-keys>.

### 4. Configure
All configuration is via **environment variables**. For local development,
copy the example and fill it in:
```bash
cp .env.example .env
# edit .env — set TELEGRAM_TOKEN, OPENAI_API_KEY, ALLOWED_USER_IDS
```
On **Railway** (or similar), don't ship a file — set the same variables in the
service's **Variables** tab. Real platform env vars always win over `.env`.

### 5. Run
```bash
make run          # or: go run ./cmd/bot
make build        # produces bin/bot
make test
```

## Configuration reference

Set via `.env` (local) or the platform's env vars (hosted).

| Env var                          | Default                     | Meaning                                    |
|----------------------------------|-----------------------------|--------------------------------------------|
| `TELEGRAM_TOKEN`                 | — **(required)**            | Bot token from BotFather.                  |
| `OPENAI_API_KEY`                 | — **(required)**            | OpenAI API key.                            |
| `ALLOWED_USER_IDS`               | — **(required)**            | Comma-separated Telegram user IDs.         |
| `OPENAI_BASE_URL`                | `https://api.openai.com/v1` | Override for Azure/proxies/compatible APIs.|
| `OPENAI_MODEL`                   | `gpt-4o`                    | Model name.                                |
| `OPENAI_TEMPERATURE`             | `0`                         | Sampling temperature.                      |
| `OPENAI_MAX_TOKENS`              | `2000`                      | Max completion tokens.                     |
| `TELEGRAM_POLL_TIMEOUT_SECONDS`  | `30`                        | Long-poll timeout.                         |
| `AGENT_SYSTEM_PROMPT`            | built-in                    | Assistant persona/instructions.            |
| `AGENT_MAX_STEPS`                | `8`                         | Max tool-call rounds per task.             |
| `AGENT_HISTORY_LIMIT`            | `20`                        | Messages of memory kept per chat.          |
| `AGENT_ACK_AFTER_SECONDS`        | `4`                         | Delay before sending the "working…" note.  |
| `WORKERS`                        | `2`                         | Concurrent task workers.                   |

## Adding your own tools

Implement `port.Tool` (see `internal/agent/tools/time.go` for a minimal example)
and register it in `cmd/bot/main.go`:

```go
agentTools := []port.Tool{
    tools.CurrentTime{},
    tools.NewHTTPGet(),
    myteam.NewJiraTool(...),  // your task-solving capability
}
```

The agent advertises every registered tool to the model automatically.

## Notes & next steps

- Storage is **in-memory**: tasks, queue, and history reset on restart. For
  durability, add Redis/Postgres adapters implementing the same ports.
- To swap the LLM backend (e.g. Anthropic, or a subscription-based CLI), write a
  new adapter implementing `port.LLM` — nothing else changes.
