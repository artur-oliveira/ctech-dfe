# WebSocket Resilience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make WebSocket connections in `ctech-dfe` and `ctech-wallet` detect a dead peer within a bounded time on both sides (native ping/pong on the server, app-level heartbeat on the client), reconnect immediately on a token refresh, and expose connection status through a small shared package instead of two duplicated hooks.

**Architecture:** New standalone repo `ctech-ws-client` publishes `@aoctech/ws-client` (mirrors the existing `ctech-oauth-client` → `@aoctech/auth-client` pattern: TS, `tsc` build, `node --test`, published under the `@aoctech` npm scope). It exports a resilient `useWebSocket` hook with a client-side app-level heartbeat, immediate reconnect-on-token-change, and a new `reconnecting` status. `ctech-dfe/ui` and `ctech-wallet/ui` both drop their local `useWebSocket.ts` in favor of this package. `ctech-dfe/api`'s `ws.go` gets real native-frame ping/pong (`WriteControl`/`SetPongHandler`/`SetReadDeadline`) so a half-open connection is detected within ~45s instead of never. `ctech-wallet/api`'s `ws.go` gets a smaller patch — just replying to the client's app-level ping — since its full native-frame port is out of scope here (different repo/deploy cadence, flagged in the spec as a separate follow-up).

**Tech Stack:** Go (Fiber v3, `fasthttp/websocket`), TypeScript (Next.js 16 hooks, `node:test`), AWS CDK (no changes needed — see Global Constraints).

## Global Constraints

- `CLIENT_PING_INTERVAL_MS = 20_000`, `CLIENT_PONG_TIMEOUT_MS = 10_000` — client-side heartbeat cadence (spec-mandated values).
- `wsPingInterval = 30 * time.Second` (existing, unchanged) — server-side native ping cadence.
- `wsPongWait = wsPingInterval + 15*time.Second` = 45s — server-side read deadline after the last native pong.
- Browser JS cannot send native WS ping frames (WHATWG constraint) — client heartbeat MUST stay app-level JSON; only the server can use native control frames.
- `@aoctech/ws-client` peer-depends on `react` (`>=18`) — do not pin the exact React version consuming apps use.
- No new abstractions beyond what each task needs — do not build a generic pub/sub library, a generic retry library, etc.
- **Correction to the approved spec:** the spec's CDK action item (set `deregistrationDelay` on the Target Group, claimed unset/defaulting to 300s) is based on incomplete investigation — `deregistrationDelay` is already hardcoded to 30s in the shared `@aoctech/cdk` construct (`ctech-cdk/lib/private-ipv4-ec2-service.ts:168`), which `ctech-dfe/cdk/lib/api-v2-stack.ts` uses. **No CDK task in this plan.** ALB `idleTimeout` is also unset anywhere in `ctech-cdk` (AWS default 60s applies), which was already the spec's expected/no-change conclusion.
- ctech-wallet/ui has no test tooling configured (no `test` script, no Vitest) — do not introduce one as a side effect of this plan; ctech-wallet tasks are import-swap + minimal server patch only, matching its existing conventions.
- The visual connection indicator (colored dot near the user icon) is explicitly out of scope for this plan — a separate `/impeccable` pass, once `useRealtimeStatus()` (Task 4) exists for it to consume.

---

### Task 1: Scaffold `ctech-ws-client` repo + pure heartbeat helpers

**Files:**
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/package.json`
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/tsconfig.json`
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/.gitignore`
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/src/heartbeat.ts`
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/test/heartbeat.test.js`

**Interfaces:**
- Produces: `nextBackoffDelay(attempt: number): number`, `isPongMessage(data: unknown): boolean`, and constants `BASE_DELAY_MS`, `MAX_DELAY_MS`, `MAX_RECONNECT_ATTEMPTS`, `CLIENT_PING_INTERVAL_MS`, `CLIENT_PONG_TIMEOUT_MS` — all consumed by Task 2's `useWebSocket.ts`.

- [ ] **Step 1: Create the GitHub repo**

```bash
gh repo create artur-oliveira/ctech-ws-client --public \
  --description "Shared resilient WebSocket React hook for apps built on ctech-account/ctech-dfe/ctech-wallet infra." \
  --clone
```

This clones it to `~/Documents/Projects/Ctech/ctech-ws-client` (run the command from `~/Documents/Projects/Ctech/`).

- [ ] **Step 2: Write `package.json`** (mirrors `ctech-oauth-client/package.json`)

```json
{
  "name": "@aoctech/ws-client",
  "version": "1.0.0",
  "description": "Resilient WebSocket React hook shared across CTech apps: app-level heartbeat, backoff reconnect, reconnect-on-token-refresh.",
  "repository": {
    "type": "git",
    "url": "https://github.com/artur-oliveira/ctech-ws-client"
  },
  "type": "module",
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    }
  },
  "files": [
    "dist"
  ],
  "publishConfig": {
    "access": "public"
  },
  "peerDependencies": {
    "react": ">=18"
  },
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "npm run build && node --test"
  },
  "keywords": [
    "websocket",
    "react",
    "hook",
    "ctech"
  ],
  "author": "Artur Oliveira",
  "license": "MIT",
  "devDependencies": {
    "@types/react": "^19.2.0",
    "react": "^19.2.4",
    "typescript": "^5.9.0"
  },
  "engines": {
    "node": ">=24"
  }
}
```

- [ ] **Step 3: Write `tsconfig.json`** (mirrors `ctech-oauth-client/tsconfig.json`, adds `jsx`/DOM lib for the hook)

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["ES2020", "DOM"],
    "jsx": "react-jsx",
    "declaration": true,
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src"]
}
```

- [ ] **Step 4: Write `.gitignore`**

```
node_modules/
dist/
```

- [ ] **Step 5: Write the failing test for the pure helpers**

`test/heartbeat.test.js`:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { nextBackoffDelay, isPongMessage, BASE_DELAY_MS, MAX_DELAY_MS } from "../dist/index.js";

test("nextBackoffDelay doubles each attempt starting at BASE_DELAY_MS", () => {
  assert.equal(nextBackoffDelay(1), BASE_DELAY_MS);
  assert.equal(nextBackoffDelay(2), BASE_DELAY_MS * 2);
  assert.equal(nextBackoffDelay(3), BASE_DELAY_MS * 4);
});

test("nextBackoffDelay caps at MAX_DELAY_MS", () => {
  assert.equal(nextBackoffDelay(10), MAX_DELAY_MS);
  assert.equal(nextBackoffDelay(30), MAX_DELAY_MS);
});

test("isPongMessage matches only {type: 'pong'}", () => {
  assert.equal(isPongMessage({ type: "pong" }), true);
  assert.equal(isPongMessage({ type: "ping" }), false);
  assert.equal(isPongMessage(null), false);
  assert.equal(isPongMessage("pong"), false);
  assert.equal(isPongMessage({}), false);
});
```

- [ ] **Step 6: Run the test to verify it fails**

```bash
cd ~/Documents/Projects/Ctech/ctech-ws-client && npm install && npm test
```

Expected: build fails (no `src/heartbeat.ts`, no `src/index.ts` yet) — `error TS2307` or similar module-not-found.

- [ ] **Step 7: Write `src/heartbeat.ts`**

```ts
export const BASE_DELAY_MS = 1_000
export const MAX_DELAY_MS = 30_000
export const MAX_RECONNECT_ATTEMPTS = 10

// Client-side app-level heartbeat cadence. The server answers native WS ping
// frames transparently at the browser layer (WHATWG gives JS no way to send
// those itself), so the client's own liveness check has to be a JSON
// heartbeat the server replies to explicitly.
export const CLIENT_PING_INTERVAL_MS = 20_000
export const CLIENT_PONG_TIMEOUT_MS = 10_000

export function nextBackoffDelay(attempt: number): number {
  return Math.min(BASE_DELAY_MS * 2 ** (attempt - 1), MAX_DELAY_MS)
}

export function isPongMessage(data: unknown): boolean {
  return typeof data === 'object' && data !== null && (data as {type?: unknown}).type === 'pong'
}
```

- [ ] **Step 8: Write `src/index.ts`** (placeholder export so the test can build; `useWebSocket` is added in Task 2)

```ts
export * from './heartbeat.js'
```

- [ ] **Step 9: Run the test to verify it passes**

```bash
npm test
```

Expected: all 5 assertions pass, `tsc` build produces `dist/index.js` + `dist/heartbeat.js`.

- [ ] **Step 10: Commit**

```bash
git add package.json tsconfig.json .gitignore src test
git commit -m "$(cat <<'EOF'
feat: scaffold ctech-ws-client with pure heartbeat helpers

Mirrors the ctech-oauth-client -> @aoctech/auth-client pattern: standalone
repo, tsc build, node --test. Backoff and pong-detection logic land first,
independent of the React hook, so they're testable without any DOM/render
harness.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Implement the resilient `useWebSocket` hook + publish the package

**Files:**
- Create: `~/Documents/Projects/Ctech/ctech-ws-client/src/useWebSocket.ts`
- Modify: `~/Documents/Projects/Ctech/ctech-ws-client/src/index.ts`

**Interfaces:**
- Consumes: `nextBackoffDelay`, `isPongMessage`, `MAX_RECONNECT_ATTEMPTS`, `CLIENT_PING_INTERVAL_MS`, `CLIENT_PONG_TIMEOUT_MS` from `./heartbeat.js` (Task 1).
- Produces: `useWebSocket(options: UseWebSocketOptions): {status: WSStatus}`, `type WSStatus = 'disconnected' | 'connecting' | 'reconnecting' | 'connected' | 'error'`, `interface UseWebSocketOptions {url, onMessage, enabled?, authToken?, subscribeToken?}` — consumed by Task 4 (`ctech-dfe/ui`) and Task 6 (`ctech-wallet/ui`).

This hook has no automated test in this repo (see Global Constraints note on why: rendering a hook needs either jsdom+RTL, adding real testing infra to a repo whose only precedent (`ctech-oauth-client`) has none, or `react-test-renderer`, which React 19 deprecates). Task 4 adds the actual behavioral test using `ctech-dfe/ui`'s existing Vitest+RTL setup once the package is installed there — that's the right place for it, not a new harness invented here.

- [ ] **Step 1: Write `src/useWebSocket.ts`**

```ts
'use client'

import {useEffect, useLayoutEffect, useRef, useState} from 'react'
import {
  nextBackoffDelay,
  isPongMessage,
  MAX_RECONNECT_ATTEMPTS,
  CLIENT_PING_INTERVAL_MS,
  CLIENT_PONG_TIMEOUT_MS,
} from './heartbeat.js'

export type WSStatus = 'disconnected' | 'connecting' | 'reconnecting' | 'connected' | 'error'

export interface UseWebSocketOptions {
  url: string | null
  onMessage: (data: unknown) => void
  enabled?: boolean
  /** JWT sent as the first frame after the upgrade (M3) so it never appears in the URL. */
  authToken?: string
  /**
   * Subscribes to access-token changes (e.g. a silent OAuth refresh). Each app
   * passes its own client.ts's token pub/sub. On a genuinely new token, the
   * hook closes the current socket and reconnects immediately with no backoff
   * delay — a refresh is a healthy, deliberate event, not a failure.
   */
  subscribeToken?: (cb: (token: string) => void) => () => void
}

export function useWebSocket({
  url,
  onMessage,
  enabled = true,
  authToken,
  subscribeToken,
}: UseWebSocketOptions): {status: WSStatus} {
  const [status, setStatus] = useState<WSStatus>('disconnected')
  const attemptsRef = useRef(0)
  const onMessageRef = useRef(onMessage)
  const authTokenRef = useRef(authToken)
  const reconnectNowRef = useRef<(() => void) | null>(null)

  useLayoutEffect(() => {
    onMessageRef.current = onMessage
  })

  useLayoutEffect(() => {
    authTokenRef.current = authToken
  })

  useEffect(() => {
    if (!subscribeToken) return
    return subscribeToken((token) => {
      authTokenRef.current = token
      reconnectNowRef.current?.()
    })
  }, [subscribeToken])

  useEffect(() => {
    if (!url || !enabled) return

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let pingTimer: ReturnType<typeof setInterval> | null = null
    let pongTimer: ReturnType<typeof setTimeout> | null = null
    let ws: WebSocket | null = null

    function clearHeartbeat() {
      if (pingTimer) clearInterval(pingTimer)
      if (pongTimer) clearTimeout(pongTimer)
      pingTimer = null
      pongTimer = null
    }

    function startHeartbeat(sock: WebSocket) {
      pingTimer = setInterval(() => {
        if (sock.readyState !== WebSocket.OPEN) return
        sock.send(JSON.stringify({type: 'ping'}))
        pongTimer = setTimeout(() => sock.close(), CLIENT_PONG_TIMEOUT_MS)
      }, CLIENT_PING_INTERVAL_MS)
    }

    function connect() {
      if (cancelled) return

      setStatus(attemptsRef.current === 0 ? 'connecting' : 'reconnecting')
      const sock = new WebSocket(url!)
      ws = sock

      sock.onopen = () => {
        if (ws !== sock) return
        attemptsRef.current = 0
        setStatus('connected')
        if (authTokenRef.current) {
          try {
            sock.send(JSON.stringify({token: authTokenRef.current}))
          } catch {
            // ignore — server closes the socket if auth is missing
          }
        }
        startHeartbeat(sock)
      }

      sock.onmessage = (evt) => {
        if (ws !== sock) return
        try {
          const data = JSON.parse(evt.data as string)
          if (isPongMessage(data)) {
            if (pongTimer) clearTimeout(pongTimer)
            return
          }
          onMessageRef.current(data)
        } catch {
          // malformed frame — ignore
        }
      }

      sock.onerror = () => {
        if (ws === sock) setStatus('error')
      }

      sock.onclose = () => {
        // A newer connection (e.g. token-refresh reconnect) already replaced
        // this one — this is a stale close event, ignore it.
        if (ws !== sock) return
        clearHeartbeat()
        ws = null
        if (cancelled) return
        setStatus('disconnected')

        attemptsRef.current++
        if (attemptsRef.current > MAX_RECONNECT_ATTEMPTS) return

        timer = setTimeout(connect, nextBackoffDelay(attemptsRef.current))
      }
    }

    reconnectNowRef.current = () => {
      attemptsRef.current = 0
      if (timer) clearTimeout(timer)
      const stale = ws
      ws = null // makes the stale socket's onclose guard (ws !== sock) a no-op
      stale?.close()
      connect()
    }

    connect()

    return () => {
      cancelled = true
      reconnectNowRef.current = null
      if (timer) clearTimeout(timer)
      clearHeartbeat()
      ws?.close(1000)
      ws = null
    }
  }, [url, enabled])

  return {status}
}
```

- [ ] **Step 2: Update `src/index.ts`**

```ts
export * from './heartbeat.js'
export * from './useWebSocket.js'
```

- [ ] **Step 3: Build and run the existing tests to confirm nothing broke**

```bash
cd ~/Documents/Projects/Ctech/ctech-ws-client && npm test
```

Expected: same 5 assertions from Task 1 pass; build now also emits `dist/useWebSocket.js` + `.d.ts`.

- [ ] **Step 4: Commit**

```bash
git add src/useWebSocket.ts src/index.ts
git commit -m "$(cat <<'EOF'
feat: add resilient useWebSocket hook

App-level client heartbeat (20s ping / 10s pong timeout) replaces the old
"reply to server's ping" hack — the server's own keepalive is now a native
protocol frame the browser answers transparently, so the client never sees
it. New reconnecting status distinct from the first-attempt connecting.
subscribeToken lets a consumer force an immediate, backoff-free reconnect
when a silent token refresh produces a new value.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Publish the package**

```bash
cd ~/Documents/Projects/Ctech/ctech-ws-client
npm publish --access public
```

Expected: `+ @aoctech/ws-client@1.0.0` published. Confirm with `npm view @aoctech/ws-client version`.

- [ ] **Step 6: Push the repo**

```bash
git push -u origin main
```

---

### Task 3: `ctech-dfe/api` — native ping/pong in `ws.go`

**Files:**
- Modify: `ctech-dfe/api/internal/api/v1/ws.go`
- Create: `ctech-dfe/api/internal/api/v1/ws_test.go`

**Interfaces:**
- Produces: `startHeartbeat(conn *fws.Conn, done <-chan struct{}, pingInterval, pongWait time.Duration, checkAlive func() bool)`, `isClientPing(msg []byte) bool` — both package-private, used only inside `ws.go`/`ws_test.go`.

The existing ping loop sends a JSON `{"type":"ping"}` text frame with no reply verification, and the read loop has no deadline — a half-open connection blocks `ReadMessage()` forever. This task replaces that with native `WriteControl(PingMessage)` + `SetPongHandler` + `SetReadDeadline`, and makes the read loop reply to the client's own app-level `{"type":"ping"}` heartbeat (added in Task 2).

No existing test infra covers `RegisterWS`'s full auth flow (it needs a live JWKS-backed RS256 `Verifier` — no fixture for that exists anywhere in `api/`). Rather than building that from scratch here (real scope creep for a ping/pong fix), the heartbeat mechanics are extracted into `startHeartbeat`/`isClientPing`, testable directly against a bare `fws.Upgrader` over `httptest.Server` with no auth involved at all.

- [ ] **Step 1: Write the failing test**

`ctech-dfe/api/internal/api/v1/ws_test.go`:

```go
package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fws "github.com/fasthttp/websocket"
)

func TestIsClientPing(t *testing.T) {
	if !isClientPing([]byte(`{"type":"ping"}`)) {
		t.Error("expected {\"type\":\"ping\"} to be detected as a client ping")
	}
	if isClientPing([]byte(`{"type":"pong"}`)) {
		t.Error("expected {\"type\":\"pong\"} not to be detected as a client ping")
	}
	if isClientPing([]byte(`not json`)) {
		t.Error("expected malformed input not to be detected as a client ping")
	}
}

func TestStartHeartbeat_MissingPongClosesReadLoop(t *testing.T) {
	const pingInterval = 20 * time.Millisecond
	const pongWait = 60 * time.Millisecond

	serverDone := make(chan struct{})
	upgrader := fws.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("server upgrade: %v", err)
			return
		}
		defer conn.Close()
		heartbeatDone := make(chan struct{})
		go startHeartbeat(conn, heartbeatDone, pingInterval, pongWait, nil)
		_, _, _ = conn.ReadMessage() // blocks until the read deadline trips
		close(heartbeatDone)
		close(serverDone)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := (&fws.Dialer{}).Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer client.Close()
	// Swallow the native ping instead of answering it, simulating a client
	// stuck without the browser's automatic pong reply.
	client.SetPingHandler(func(string) error { return nil })

	select {
	case <-serverDone:
		// expected: the server's read loop unblocked once the deadline tripped
	case <-time.After(2 * time.Second):
		t.Fatal("server did not close a connection that never sent a pong within pongWait")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ctech-dfe/api && go test ./internal/api/v1/... -run 'TestIsClientPing|TestStartHeartbeat' -v
```

Expected: FAIL — `undefined: isClientPing`, `undefined: startHeartbeat`.

- [ ] **Step 3: Modify `ws.go`**

Replace lines 20-22 (the two existing const declarations):

```go
const wsPingInterval = 30 * time.Second

const wsAuthTimeout = 5 * time.Second
```

with:

```go
const wsPingInterval = 30 * time.Second

const wsAuthTimeout = 5 * time.Second

// wsPongWait is the read deadline after the last native pong — a standard
// margin (15s) over wsPingInterval so one slow tick doesn't false-positive.
const wsPongWait = wsPingInterval + 15*time.Second

const wsWriteWait = 5 * time.Second
```

Replace the ping-loop goroutine + read loop (originally lines 116-147):

```go
			// Ping loop. Also re-checks membership each tick (cached read, ~zero
			// cost) so a member removed after the socket opened stops receiving
			// events instead of staying subscribed forever.
			done := make(chan struct{})
			go func() {
				t := time.NewTicker(wsPingInterval)
				defer t.Stop()
				for {
					select {
					case <-t.C:
						if still, e := memberSvc.Get(ctx, orgPK, sub); e == nil && still == nil {
							send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso revogado"})
							_ = conn.Close()
							return
						}
						if e := conn.WriteMessage(fws.TextMessage, []byte(`{"type":"ping"}`)); e != nil {
							return
						}
					case <-done:
						return
					}
				}
			}()

			// Read loop — detect disconnect, accept pong
			for {
				_, _, e := conn.ReadMessage()
				if e != nil {
					break
				}
			}
			close(done)
			slog.Info("ws disconnected", "conn", connID, "org", orgPK)
```

with:

```go
			// Heartbeat: native ping/pong frames (the browser answers these
			// transparently — no client code involved) plus a re-check of
			// membership each tick (cached read, ~zero cost) so a member removed
			// after the socket opened stops receiving events instead of staying
			// subscribed forever.
			done := make(chan struct{})
			checkAlive := func() bool {
				still, e := memberSvc.Get(ctx, orgPK, sub)
				if e == nil && still == nil {
					send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso revogado"})
					return false
				}
				return true
			}
			go startHeartbeat(conn, done, wsPingInterval, wsPongWait, checkAlive)

			// Read loop — detects a dead connection via the heartbeat's read
			// deadline, and replies to the client's own app-level {"type":"ping"}
			// heartbeat (the client can't send native ping frames — WHATWG gives
			// browsers no API for that).
			for {
				_, msg, e := conn.ReadMessage()
				if e != nil {
					break
				}
				if isClientPing(msg) {
					send(map[string]any{"type": "pong"})
				}
			}
			close(done)
			slog.Info("ws disconnected", "conn", connID, "org", orgPK)
```

Add these two functions after `RegisterWS` (before the `wsConnAdapter` type):

```go
// startHeartbeat sends a native ping every pingInterval and arms/resets a read
// deadline on every native pong, so a half-open connection (no pong within
// pongWait) breaks the caller's blocking ReadMessage() instead of lingering
// forever. checkAlive lets the caller veto the next ping (e.g. revoked
// membership) — returning false closes the connection immediately.
func startHeartbeat(conn *fws.Conn, done <-chan struct{}, pingInterval, pongWait time.Duration, checkAlive func() bool) {
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))

	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if checkAlive != nil && !checkAlive() {
				_ = conn.Close()
				return
			}
			if e := conn.WriteControl(fws.PingMessage, nil, time.Now().Add(wsWriteWait)); e != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// isClientPing reports whether msg is the client's app-level heartbeat frame
// (the client can't send a native WS ping — see startHeartbeat).
func isClientPing(msg []byte) bool {
	var p struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(msg, &p) == nil && p.Type == "ping"
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/api/v1/... -run 'TestIsClientPing|TestStartHeartbeat' -v
```

Expected: PASS for both tests.

- [ ] **Step 5: Run the full api test suite + build**

```bash
go build ./... && go test ./... -race
```

Expected: no failures, no new race warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/api/v1/ws.go internal/api/v1/ws_test.go
git commit -m "$(cat <<'EOF'
fix(api): enforce native WS ping/pong instead of an unverified JSON ping

The old ping loop sent {"type":"ping"} and never checked for a reply, and
the read loop had no deadline, so a half-open connection (TCP reset lost
somewhere in the proxy chain) blocked ReadMessage() forever — the reported
"stuck in connected" symptom. Native WriteControl/SetPongHandler/
SetReadDeadline give the server a real ~45s bound, and the read loop now
answers the client's own app-level heartbeat ping (added by @aoctech/ws-
client), since a browser can't send native ping frames itself.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `ctech-dfe/ui` — consume `@aoctech/ws-client`, reconnect-on-refresh, status Context

**Files:**
- Modify: `ctech-dfe/ui/package.json`
- Delete: `ctech-dfe/ui/src/lib/hooks/useWebSocket.ts`
- Modify: `ctech-dfe/ui/src/lib/hooks/useRealtimeUpdates.ts`
- Modify: `ctech-dfe/ui/src/lib/api/client.ts`
- Modify: `ctech-dfe/ui/src/lib/providers/RealtimeProvider.tsx`
- Test: `ctech-dfe/ui/src/__tests__/lib/useRealtimeUpdates.test.tsx`

**Interfaces:**
- Consumes: `useWebSocket`, `WSStatus` from `@aoctech/ws-client` (Task 2); `CLIENT_PING_INTERVAL_MS`, `CLIENT_PONG_TIMEOUT_MS` from the same package, for the test's fake timers.
- Produces: `subscribeAccessToken(cb: (token: string) => void): () => void` (in `client.ts`), `useRealtimeStatus(): WSStatus` (in `RealtimeProvider.tsx`) — both are new public surface other files may use later (e.g. the future connection indicator).

- [ ] **Step 1: Add the dependency**

In `ctech-dfe/ui/package.json`, in `dependencies`, add (alphabetically, right after `@aoctech/auth-client`):

```json
    "@aoctech/ws-client": "^1.0.0",
```

Then:

```bash
cd ctech-dfe/ui && npm install
```

- [ ] **Step 2: Add `subscribeAccessToken` to `client.ts`**

In `ctech-dfe/ui/src/lib/api/client.ts`, after the `_refreshFn`/`registerRefreshFn` block (after line 60, before `export function getAccessToken`):

```ts
const tokenListeners = new Set<(token: string) => void>()

// Notified on every genuinely new access token (login, silent refresh) — lets
// a WebSocket consumer force an immediate reconnect instead of holding a
// stale token indefinitely (see @aoctech/ws-client's subscribeToken).
export function subscribeAccessToken(cb: (token: string) => void): () => void {
  tokenListeners.add(cb)
  return () => tokenListeners.delete(cb)
}

function notifyTokenListeners(token: string): void {
  tokenListeners.forEach((cb) => cb(token))
}
```

Then change the interceptor's refresh branch (originally around line 129-130):

```ts
        if (newToken) {
          _accessToken = newToken
```

to:

```ts
        if (newToken) {
          _accessToken = newToken
          notifyTokenListeners(newToken)
```

And change `setToken` (originally around line 162-164):

```ts
  setToken(token: string | null): void {
    _accessToken = token
  }
```

to:

```ts
  setToken(token: string | null): void {
    _accessToken = token
    if (token) notifyTokenListeners(token)
  }
```

- [ ] **Step 3: Delete the local hook**

```bash
rm ctech-dfe/ui/src/lib/hooks/useWebSocket.ts
```

- [ ] **Step 4: Update `useRealtimeUpdates.ts`**

In `ctech-dfe/ui/src/lib/hooks/useRealtimeUpdates.ts`, change the import (line 7):

```ts
import {useWebSocket, type WSStatus} from './useWebSocket'
```

to:

```ts
import {useWebSocket, type WSStatus} from '@aoctech/ws-client'
```

Change the `getAccessToken` import (line 9) to also bring in `subscribeAccessToken`:

```ts
import {getAccessToken, subscribeAccessToken} from '@/lib/api/client'
```

Change the final `useWebSocket` call (originally lines 96-101):

```ts
  const {status: wsStatus} = useWebSocket({
    url: wsUrl,
    onMessage: handleMessage,
    enabled: !!wsUrl,
    authToken: token ?? undefined,
  })
```

to:

```ts
  const {status: wsStatus} = useWebSocket({
    url: wsUrl,
    onMessage: handleMessage,
    enabled: !!wsUrl,
    authToken: token ?? undefined,
    subscribeToken: subscribeAccessToken,
  })
```

- [ ] **Step 5: Add status Context to `RealtimeProvider.tsx`**

Replace the full contents of `ctech-dfe/ui/src/lib/providers/RealtimeProvider.tsx`:

```tsx
'use client'

import {createContext, useContext} from 'react'
import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'
import type {WSStatus} from '@aoctech/ws-client'
import React from 'react'

const RealtimeStatusContext = createContext<WSStatus>('disconnected')

/** Current WebSocket connection status — for a future connection indicator. */
export function useRealtimeStatus(): WSStatus {
  return useContext(RealtimeStatusContext)
}

export function RealtimeProvider({children}: {children: React.ReactNode}) {
  const {wsStatus} = useRealtimeUpdates()
  return <RealtimeStatusContext.Provider value={wsStatus}>{children}</RealtimeStatusContext.Provider>
}
```

- [ ] **Step 6: Write the failing hook-behavior test**

`ctech-dfe/ui/src/__tests__/lib/useRealtimeUpdates.test.tsx` (new file):

```tsx
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import {act, renderHook, waitFor} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {CLIENT_PING_INTERVAL_MS, CLIENT_PONG_TIMEOUT_MS} from '@aoctech/ws-client'
import {useRealtimeUpdates} from '@/lib/hooks/useRealtimeUpdates'

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_00000000000191'}}),
}))
vi.mock('@/lib/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/client')>()
  return {...actual, getAccessToken: () => 'test-token'}
})

class FakeWebSocket {
  static OPEN = 1
  static instances: FakeWebSocket[] = []
  readyState = FakeWebSocket.OPEN
  onopen: (() => void) | null = null
  onmessage: ((evt: {data: string}) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  sent: string[] = []

  constructor(public url: string) {
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.onclose?.()
  }
}

function wrapper({children}: {children: ReactNode}) {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useRealtimeUpdates', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('closes the socket when a client heartbeat pong never arrives', async () => {
    renderHook(() => useRealtimeUpdates(), {wrapper})
    const sock = FakeWebSocket.instances[0]
    act(() => sock.onopen?.())

    act(() => vi.advanceTimersByTime(CLIENT_PING_INTERVAL_MS))
    expect(sock.sent).toContain(JSON.stringify({type: 'ping'}))

    let closed = false
    sock.close = () => {
      closed = true
    }
    act(() => vi.advanceTimersByTime(CLIENT_PONG_TIMEOUT_MS))
    expect(closed).toBe(true)
  })

  it('reconnects immediately (no backoff) when the access token changes', async () => {
    const {apiClient} = await import('@/lib/api/client')
    renderHook(() => useRealtimeUpdates(), {wrapper})
    const first = FakeWebSocket.instances[0]
    act(() => first.onopen?.())

    let firstClosed = false
    first.close = () => {
      firstClosed = true
    }

    // Real production path: apiClient.setToken -> notifyTokenListeners ->
    // subscribeAccessToken's subscriber (wired in Task 4 Step 2/4).
    act(() => apiClient.setToken('new-token'))

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(2))
    expect(firstClosed).toBe(true)
  })
})
```

- [ ] **Step 7: Run the test to verify it fails**

```bash
cd ctech-dfe/ui && npx vitest run src/__tests__/lib/useRealtimeUpdates.test.tsx
```

Expected: FAIL — before Task 3/4's code changes are all in place this may fail for various reasons (missing export, wrong mock shape); confirm the failure is about the assertions, not an import error unrelated to this task. If `getAccessToken`/`apiClient` mock shape mismatches the real module, adjust the `vi.mock` factory to match `client.ts`'s actual exports.

- [ ] **Step 8: Run the test to verify it passes**

```bash
npx vitest run src/__tests__/lib/useRealtimeUpdates.test.tsx
```

Expected: both tests pass.

- [ ] **Step 9: Run the full ui checks**

```bash
npx eslint src --ext .ts,.tsx
npm test
```

Expected: zero ESLint errors/warnings; full Vitest suite passes.

- [ ] **Step 10: Commit**

```bash
git add package.json package-lock.json src/lib/hooks/useRealtimeUpdates.ts src/lib/api/client.ts src/lib/providers/RealtimeProvider.tsx src/__tests__/lib/useRealtimeUpdates.test.tsx
git rm src/lib/hooks/useWebSocket.ts
git commit -m "$(cat <<'EOF'
feat(ui): consume @aoctech/ws-client, reconnect on token refresh

Drops the local useWebSocket.ts (byte-identical duplicate of ctech-wallet's)
in favor of the shared package. client.ts now notifies subscribers on every
new access token (login + silent refresh), so a background token refresh
triggers an immediate socket reconnect instead of the hook holding a stale
token indefinitely. RealtimeProvider exposes wsStatus via a small Context
(useRealtimeStatus) for the connection indicator to consume later.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: `ctech-wallet/api` — minimal patch: reply to the client's app-level ping

**Files:**
- Modify: `ctech-wallet/api/internal/api/v1/ws.go`
- Create: `ctech-wallet/api/internal/api/v1/ws_test.go`

**Interfaces:**
- Produces: `isClientPing(msg []byte) bool` (package-private, duplicated from `ctech-dfe/api` — the two `ws.go` files are in different repos/modules and the spec explicitly deferred unifying them).

Full native-frame ping/pong is **not** done here (separate follow-up, per the spec and the earlier scoping decision) — this task only makes the server reply `{"type":"pong"}` to the client's own heartbeat ping, since swapping `ctech-wallet/ui` to `@aoctech/ws-client` (Task 6) means the client now expects that reply. Without this, every wallet connection would spuriously close every ~30s (client heartbeat times out waiting for a pong the server never sends).

- [ ] **Step 1: Write the failing test**

`ctech-wallet/api/internal/api/v1/ws_test.go`:

```go
package v1

import "testing"

func TestIsClientPing(t *testing.T) {
	if !isClientPing([]byte(`{"type":"ping"}`)) {
		t.Error("expected {\"type\":\"ping\"} to be detected as a client ping")
	}
	if isClientPing([]byte(`{"type":"pong"}`)) {
		t.Error("expected {\"type\":\"pong\"} not to be detected as a client ping")
	}
	if isClientPing([]byte(`not json`)) {
		t.Error("expected malformed input not to be detected as a client ping")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ctech-wallet/api && go test ./internal/api/v1/... -run TestIsClientPing -v
```

Expected: FAIL — `undefined: isClientPing`.

- [ ] **Step 3: Modify `ws.go`**

Replace the read loop (originally lines 133-137):

```go
			for {
				if _, _, e := conn.ReadMessage(); e != nil {
					break
				}
			}
```

with:

```go
			for {
				_, msg, e := conn.ReadMessage()
				if e != nil {
					break
				}
				if isClientPing(msg) {
					send(map[string]any{"type": "pong"})
				}
			}
```

Add this function after `RegisterWS` (before the `wsConnAdapter` type):

```go
// isClientPing reports whether msg is the client's app-level heartbeat frame.
// The client can't send a native WS ping (WHATWG gives browsers no API for
// that), so it uses a JSON heartbeat instead — this server has to reply
// explicitly or every client connection times out waiting for a pong.
func isClientPing(msg []byte) bool {
	var p struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(msg, &p) == nil && p.Type == "ping"
}
```

`encoding/json` is already imported (line 4) — no import changes needed.

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/api/v1/... -run TestIsClientPing -v
```

Expected: PASS.

- [ ] **Step 5: Run the full api test suite + build**

```bash
go build ./... && go test ./... -race
```

- [ ] **Step 6: Commit**

```bash
git add internal/api/v1/ws.go internal/api/v1/ws_test.go
git commit -m "$(cat <<'EOF'
fix(api): reply to the client's app-level WS heartbeat ping

ctech-wallet/ui is moving to @aoctech/ws-client, whose client-side heartbeat
sends {"type":"ping"} and closes the socket if no {"type":"pong"} arrives
within 10s. The read loop previously discarded every inbound message, so
every wallet connection would have cycled every ~30s. Full native-frame
ping/pong (like ctech-dfe/api) stays a separate follow-up.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: `ctech-wallet/ui` — consume `@aoctech/ws-client`

**Files:**
- Modify: `ctech-wallet/ui/package.json`
- Delete: `ctech-wallet/ui/src/lib/hooks/useWebSocket.ts`
- Modify: `ctech-wallet/ui/src/lib/hooks/useWalletRealtime.ts`
- Modify: `ctech-wallet/ui/src/lib/api/client.ts`

No new test in this task — `ctech-wallet/ui` has no test tooling configured at all (no `test` script, no Vitest/Jest), and this plan doesn't introduce one as a side effect (see Global Constraints).

- [ ] **Step 1: Add the dependency**

In `ctech-wallet/ui/package.json`, in `dependencies`, add (alphabetically, right after `@aoctech/auth-client`):

```json
    "@aoctech/ws-client": "^1.0.0",
```

```bash
cd ctech-wallet/ui && npm install
```

- [ ] **Step 2: Add `subscribeAccessToken` to `client.ts`**

In `ctech-wallet/ui/src/lib/api/client.ts`, after the `registerRefreshFn`/`getAccessToken` block (after line 31):

```ts
const tokenListeners = new Set<(token: string) => void>()

export function subscribeAccessToken(cb: (token: string) => void): () => void {
    tokenListeners.add(cb)
    return () => tokenListeners.delete(cb)
}

function notifyTokenListeners(token: string): void {
    tokenListeners.forEach((cb) => cb(token))
}
```

(Indentation matches this file's existing 4-space style, unlike `ctech-dfe`'s 2-space.)

Change the interceptor's refresh branch (originally line 79):

```ts
                if (newToken) {
                    _accessToken = newToken
```

to:

```ts
                if (newToken) {
                    _accessToken = newToken
                    notifyTokenListeners(newToken)
```

Change `setToken` (originally line 111-113):

```ts
    setToken(token: string | null): void {
        _accessToken = token
    }
```

to:

```ts
    setToken(token: string | null): void {
        _accessToken = token
        if (token) notifyTokenListeners(token)
    }
```

- [ ] **Step 3: Delete the local hook**

```bash
rm ctech-wallet/ui/src/lib/hooks/useWebSocket.ts
```

- [ ] **Step 4: Update `useWalletRealtime.ts`**

Change the import (line 7):

```ts
import {useWebSocket, type WSStatus} from './useWebSocket'
```

to:

```ts
import {useWebSocket, type WSStatus} from '@aoctech/ws-client'
```

Change the `getAccessToken` import (line 8):

```ts
import {getAccessToken} from '@/lib/api/client'
```

to:

```ts
import {getAccessToken, subscribeAccessToken} from '@/lib/api/client'
```

Change the final `useWebSocket` call (originally lines 78-83):

```ts
    const {status: wsStatus} = useWebSocket({
        url: wsUrl,
        onMessage: handleMessage,
        enabled: !!wsUrl,
        authToken: token ?? undefined,
    })
```

to:

```ts
    const {status: wsStatus} = useWebSocket({
        url: wsUrl,
        onMessage: handleMessage,
        enabled: !!wsUrl,
        authToken: token ?? undefined,
        subscribeToken: subscribeAccessToken,
    })
```

- [ ] **Step 5: Run the lint/build check**

```bash
npx eslint src --ext .ts,.tsx
npx tsc --noEmit
```

Expected: zero errors.

- [ ] **Step 6: Manual smoke check**

```bash
npm run dev
```

Open the app, confirm the dashboard loads and the wallet WebSocket connects (network tab shows a `101` upgrade on `/v1.0/ws`, no repeated reconnect loop within the first minute).

- [ ] **Step 7: Commit**

```bash
git add package.json package-lock.json src/lib/hooks/useWalletRealtime.ts src/lib/api/client.ts
git rm src/lib/hooks/useWebSocket.ts
git commit -m "$(cat <<'EOF'
feat(ui): consume @aoctech/ws-client, reconnect on token refresh

Drops the local useWebSocket.ts (byte-identical duplicate of ctech-dfe's)
in favor of the shared package, matching the api-side patch in the same
plan (ws.go now replies to the client's app-level heartbeat ping).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Update `ctech-dfe` docs

**Files:**
- Modify: `ctech-dfe/CONDUCT.md`
- Modify: `ctech-dfe/DOCS.md`

**Interfaces:** none — documentation only.

- [ ] **Step 1: Add a CONDUCT.md entry**

Add a bullet under the constraint/workaround section (match existing formatting in the file) documenting: the WS heartbeat protocol (server native ping/45s pong-wait deadline; client app-level `{"type":"ping"}`/10s pong-timeout every 20s); that `@aoctech/ws-client` is now the canonical hook for both `ctech-dfe` and `ctech-wallet`; and that `ctech-wallet/api`'s native-frame port remains an open follow-up.

- [ ] **Step 2: Add a DOCS.md entry**

Document the new `@aoctech/ws-client` package (what it replaces, where it's published) and the `ws.go` heartbeat behavior change, in whichever section already documents the WebSocket endpoint.

- [ ] **Step 3: Commit**

```bash
cd ctech-dfe && git add CONDUCT.md DOCS.md
git commit -m "$(cat <<'EOF'
docs: document the WS heartbeat protocol and @aoctech/ws-client

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```
