# WebSocket Resilience — Design

**Status:** Approved
**Scope:** `ctech-dfe` (`ui`, `api`, `cdk`) + new shared package `@aoctech/ws-client` (new repo
`ctech-ws-client`) consumed by `ctech-dfe/ui` and `ctech-wallet/ui`.
**Out of scope:** visual design of the connection indicator (colors, animation, placement) — a
separate `/impeccable` pass once this spec ships. Server-side `ws.go` extraction into
`api-commons/ws` — flagged as a future candidate, not done here (the two handlers still differ in
domain specifics beyond the ping/pong skeleton).

---

## Problem

Reported by the user, observed across `ctech-dfe` (and structurally shared with `ctech-wallet`,
which has an identical WebSocket client):

1. Frontend doesn't reliably detect a dead connection after a server-side restart/redeploy — the
   UI can sit in an apparently-connected state with no way for the user to tell otherwise.
2. Ping/pong exists in name only. The server sends an app-level `{"type":"ping"}` message every
   30s and the client replies `{"type":"pong"}`, but **neither side checks the reply ever
   arrives** — there's no deadline, no enforcement. A half-open connection (TCP reset never
   delivered, e.g. across a proxy chain) lingers indefinitely on both ends.
3. Client resilience gaps: retry/backoff already exists and is solid, but the access token
   refreshing silently in the background never triggers a socket reconnect — nothing observes the
   change (see Root Cause below).
4. Suspicion that ALB sticky sessions are involved. Investigated and ruled out (see below);
   real contributor identified instead (Target Group `deregistrationDelay`).
5. `@aoctech/ws-client` doesn't exist yet, but `ui/src/lib/hooks/useWebSocket.ts` is
   **byte-identical** (whitespace aside) between `ctech-dfe` and `ctech-wallet` — confirmed
   duplication, not suspected. Precedent for extraction already exists: `@aoctech/auth-client`,
   published from a standalone repo (`ctech-oauth-client`), consumed by both apps.
6. No connection indicator exists in the UI today. `useRealtimeUpdates()`'s `wsStatus` return
   value is computed and discarded by `RealtimeProvider` — nothing consumes it.

## Root Cause Notes

- **"Unknown" state is not a real code state.** `WSStatus` only has
  `disconnected | connecting | connected | error`. The reported "unknown" is the *absence* of any
  visible indicator combined with the lack of any liveness enforcement — the last known state
  (`connected`) can go stale for an unbounded time after a silent mid-stream break.
- **Sticky sessions are not the mechanism and should not be added.** A WebSocket connection is
  pinned to whichever EC2 instance served its upgrade handshake for the connection's entire
  lifetime — that's inherent to a persistent TCP connection, not something a Target Group
  stickiness cookie provides. Reconnects are free to land on any instance: broadcast fan-out
  already goes through Redis pub/sub (`ws.Registry`), so any replica holding the client's socket
  receives the event regardless of which replica processed the originating SQS message. Adding
  cookie-based stickiness would only introduce a new failure mode (a reconnect cookie pointing at
  a draining/terminated instance stalls until the cookie expires).
- **Real contributor: Target Group `deregistrationDelay`.** `cdk/lib/api-v2-stack.ts` does not set
  `deregistrationDelay` (AWS default: 300s) or the ALB `idleTimeout` (AWS default: 60s). During a
  rolling deploy, a draining instance can keep existing WebSocket connections open for up to 5
  minutes before the ALB force-closes them — and with no heartbeat on either side, nothing detects
  the connection is already dead in practice during that window.
- **Browser platform constraint discovered mid-design:** the WHATWG WebSocket API does not expose
  protocol-level ping/pong control frames to JavaScript. A page cannot send a raw WS ping frame
  (unlike Node's `ws` library, which has `.ping()`) — the browser answers a server-sent ping
  automatically at the network layer, transparently to page code, per RFC 6455. This makes
  server→client liveness free (native frames, zero client code) but forces client→server liveness
  to stay at the application level (JSON heartbeat) — there is no alternative.
- **Token-refresh reconnect requires new plumbing.** `_accessToken` in `ui/src/lib/api/client.ts`
  is a bare module-level variable with no subscription mechanism. `useRealtimeUpdates` reads it
  once per render; a silent background refresh (triggered by an unrelated 401 elsewhere in the
  app) does not itself cause a re-render, so the hook can hold a stale token indefinitely. A
  reconnect-on-refresh feature needs an actual notification, not a hope that some other render
  happens to pick up the new value.

## Architecture

### New package: `@aoctech/ws-client` (new repo `ctech-ws-client`)

Mirrors the existing `ctech-oauth-client` → `@aoctech/auth-client` pattern: standalone repo, TS,
`tsc` build, `node --test`, published under the `@aoctech` npm scope, `react` as a peer dependency.

Exports the resilient `useWebSocket` hook (see below) and the `WSStatus` type. `ctech-dfe/ui` and
`ctech-wallet/ui` both drop their local `lib/hooks/useWebSocket.ts` in favor of importing it.
Domain-specific message handling (`useRealtimeUpdates`, `useWalletRealtime`) stays in each app —
those aren't duplicated, they just consume the shared hook.

### `useWebSocket` changes

Same public surface (`url`, `onMessage`, `enabled`, `authToken`), plus:

- **`subscribeToken?: (cb: (token: string) => void) => () => void`** — optional. Each app passes
  its own `client.ts`'s new `subscribeAccessToken` export. On a genuinely new token, the hook
  closes the current socket, resets `attemptsRef` to `0`, and reconnects immediately (no backoff
  delay — a refresh is a healthy, deliberate event, not a failure).
- **Client heartbeat:** every `CLIENT_PING_INTERVAL_MS` (20s) sends `{"type":"ping"}`; arms a
  `CLIENT_PONG_TIMEOUT_MS` (10s) timer. An incoming `{"type":"pong"}` clears it. On timeout, the
  hook calls `ws.close()` — the existing `onclose` → backoff-reconnect path handles the rest
  unchanged.
- **Ping handling removed from `onmessage`:** the current `if (data.type === 'ping') sock.send(pong)`
  hack is deleted. The server's keepalive is now a native protocol frame the browser answers
  without page code ever seeing it.
- **New status value `reconnecting`**, distinct from `connecting`: `connect()` sets `connecting`
  only when `attemptsRef.current === 0` (first attempt), `reconnecting` otherwise. Existing
  `disconnected | connected | error` values are unchanged. This is plumbing only — no visual
  treatment decided here.

### `client.ts` changes (both apps, same shape)

Add a minimal pub/sub around the existing module-level token variable:

```ts
const tokenListeners = new Set<(token: string) => void>()

export function subscribeAccessToken(cb: (token: string) => void): () => void {
  tokenListeners.add(cb)
  return () => tokenListeners.delete(cb)
}
```

Every place `_accessToken` is assigned a new value (`doRefresh()`, login) now also notifies
`tokenListeners`. This is additive — the single silent-refresh mechanism `ui/CLAUDE.md` mandates
is untouched; this only makes its outcome observable.

### `api/internal/api/v1/ws.go` changes (same patch shape in `ctech-dfe`; recommend porting to
`ctech-wallet` separately since it's a different repo/deploy)

- Ping loop: replace the JSON `WriteMessage(TextMessage, {"type":"ping"})` with
  `conn.WriteControl(fws.PingMessage, nil, deadline)`.
- `conn.SetPongHandler(...)` resets a read deadline on every native pong.
- `conn.SetReadDeadline(now + pongWait)` set once after auth and refreshed by the pong handler.
  `pongWait = wsPingInterval + 15s` (45s) — standard margin over the 30s ping interval.
- Read loop: parse each inbound text frame. If it's the client's app-level
  `{"type":"ping"}` heartbeat, reply `{"type":"pong"}` immediately. Everything else behaves as
  today (loop breaks — and now also naturally breaks on read-deadline-exceeded — cleanup via the
  existing `defer reg.Unregister` runs unchanged).

### `cdk/lib/api-v2-stack.ts`

Set the Target Group `deregistrationDelay` to a short value (30–60s) now that the heartbeat makes
a long drain window unnecessary for detecting a dead connection — verify against the ALB's
default 60s `idleTimeout` (no change expected there, since heartbeat traffic on both sides already
keeps the connection well under that).

### Status Context (`ui`)

`RealtimeProvider` currently discards `wsStatus`. It now provides it through a small React Context
in the same file (no new abstraction layer beyond what's needed) so a future indicator component
can consume it via a `useRealtimeStatus()` hook — the visual indicator itself is out of scope here.

## Data Flow — Failure Paths

- **Server instance dies mid-connection:** the next scheduled native ping's `WriteControl` fails
  immediately if the socket is already reset; if the TCP layer stays falsely "open" (the reported
  bug), the read deadline (≤45s since the last native pong) trips, the read loop breaks, and
  `reg.Unregister` reclaims the slot. Independently, the client's own heartbeat (20s ping + 10s
  wait ⇒ ≤30s worst case) detects the same dead link and closes+reconnects on its own. Whichever
  side notices first wins in practice; both converge within under a minute — a hard bound where
  today there is none.
- **Background token refresh:** `subscribeAccessToken` fires → hook closes the current socket,
  resets attempts, reconnects immediately with the fresh token.
- **Proxy chain (Cloudflare → CloudFront → ALB):** assumed to pass WebSocket control/data frames
  through transparently (standard behavior for all three) — noted as a one-time deploy
  verification, not a blocking risk.

## Testing

- `@aoctech/ws-client`: hook tests — existing backoff/reconnect behavior (regression), client
  heartbeat timeout → close + reconnect, token change → immediate reconnect with reset attempt
  counter.
- `ctech-dfe/api` `ws.go`: test that a missing native pong within `pongWait` closes and
  unregisters the connection; test that an inbound app-level `{"type":"ping"}` gets an immediate
  `{"type":"pong"}` reply.
- Regression test matching the originally reported symptom: simulate a server-side hard
  disconnect with no clean close frame; assert the client transitions out of `connected` within
  the heartbeat window instead of staying stuck.
