# Roadmap & known gaps

A short list of things `bf` does **not** do today, ordered roughly by how
often they come up.

## Likely next

- **Webhook mode.** Today the bot only does long-polling via
  `tgbotapi.GetUpdatesChan`. tgbotapi already supports webhooks; bf needs an
  `Start(...) WithWebhook(...)`-style option and a `http.Handler`.
- **Rich message senders.** `SendPhoto`, `SendDocument`, `SendVoice`,
  `SendLocation`. Right now you fall back to the raw tgbotapi via your own
  wrapper. The framework should ship the common ones.
- **Edit / delete helpers.** `EditMessageText`, `DeleteMessage`,
  `AnswerCallbackQuery`. Required for any UI more polished than a chain of
  fresh messages.
- **Graceful shutdown with timeout.** `Stop()` is immediate; a
  `Shutdown(ctx)` that waits for in-flight handlers up to a deadline (like
  `http.Server.Shutdown`) is the standard Go shape.

## Worth doing eventually

- **Rate limiting.** Telegram caps at ~30 msgs/s globally and 1 msg/s per
  chat. We do not enforce that — heavy senders will hit `429 Too Many
  Requests`. A per-chat token bucket would be small and useful.
- **Set-my-commands.** Telegram lets bots register a list of commands that
  shows up in the typing UI. Today users have to call tgbotapi directly.
- **Per-chat / per-user middleware.** The current middleware chain is
  global. Filters (admin-only, group-only, etc.) are a common ask.
- **State store.** Layers live in-process memory. A pluggable interface
  (`LayerStore` with `Get` / `Put` / `Delete`) would let users swap in
  Redis / Postgres for multi-instance deployments.
- **Inline-mode (`InlineQuery`).** Currently dropped on the floor.
- **Group/channel events.** New chat members, left members, pinned
  messages — none of these are normalised into `Event`.

## Probably not

- **FSM library on top of layers.** Tempting, but the layer model is
  already a small state machine. Adding another abstraction layer would
  trade flexibility for boilerplate.
- **Reflection-based command routing.** Go programmers expect to register
  handlers explicitly; reflection magic is more pain than gain at this
  scale.

## Coverage gaps in the current test suite

- `NewBot` happy path (the variant talking to api.telegram.org) — needs
  network. `NewBotWithEndpoint` covers the same code paths against a stub.
- A few `error`-return branches inside `LoaderButton` /
  `defaultEventHandler` / `Event.json` that require a JSON marshaler that
  fails — unreachable with the current struct shapes.

If you need any of the above, please open an issue first so the API can
be discussed before you write the PR.
