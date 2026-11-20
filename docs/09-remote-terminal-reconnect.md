# Remote terminal reconnect and idle recovery

## Behavior

- The main **Reconnect** action resumes the existing shell. **Reset Shell &
  Reconnect** remains an explicit action: it terminates that shell and starts a
  new one (working directory, in-memory shell history and foreground jobs reset).
- Server-side WebSocket pings run independently of SSH keepalive writes. Browser
  heartbeat/focus handling detects stale OPEN sockets after sleep/network changes
  and reconnects. No browser timer is assumed immune to suspension.
- A disconnected browser leaves its shell alive for the existing 15-minute grace
  period. A stale connection/timer cannot expire a successfully reattached shell.
- Output produced while detached is retained in backend memory, capped at 1 MiB
  per shell. Sequence cursors replay missed output without duplicating output
  already displayed. A recreated browser display requests the retained transcript.
  Truncation is reported explicitly. Terminal output is not written to localStorage
  or to server disk because it can contain credentials and other sensitive data.
- Reattach does not inject Enter; it must not execute a partly typed command.
- Closing a terminal tab explicitly closes its shell. Leaving/reloading the page
  detaches the socket and releases browser workers without reconnecting a disposed
  terminal behind the scenes.

## Limits

This is not tmux/screen or durable session recording. Backend restarts, SSH server
disconnects, shell logout/TMOUT policies, **Reset Shell**, or expiry of the grace
period cannot restore a previous live shell. Already displayed browser scrollback
is retained on ordinary reconnect; only the bounded backend transcript can be
replayed into a newly created display. Browser/OS suspension or external proxies
can still sever a transport; the fix makes recovery non-destructive, not a promise
that every network connection stays open indefinitely.

## Validation

`internal/services/ws_terminal_service_test.go` covers missed-output replay,
bounded history/truncation, stale detach/timer safety, server heartbeats without
browser timers, no injected shell input, and a real loopback SSH channel reused
across two WebSocket connections. Full Go tests, staticcheck/vet and frontend
typecheck/build are required; Linux CI supplies race instrumentation. Isolated
tests do not certify a customer's SSH idle policy or reverse-proxy configuration.

Manual deployment checks: emit numbered output, disconnect/reconnect, verify no
missing/duplicate retained lines; leave the tab idle/backgrounded, then resume;
check `pwd`, a shell variable and Up-arrow history survive a normal reconnect.
Test an explicit shell reset separately and confirm it is not mistaken for resume.
