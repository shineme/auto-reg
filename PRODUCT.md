# Product

<!-- impeccable:product-schema 1 -->

## Platform

desktop

## Stack

Inferred from brief: Go + Wails v2, plain HTML/CSS/JS frontend. The existing protocol client and batch scripts are Node.js helpers invoked by the GUI.

## Users

The operator who maintains a magpie MiniMax Code (Global) account pool. They need a local desktop tool to configure credentials, domain mail, inbox settings, proxy exit IP, and batch size without editing environment variables or running several shell commands.

## Product Purpose

Make the validated MiniMax registration protocol usable as a repeatable desktop workflow. The tool configures inputs, persists them locally, starts a proxy bridge for HTTP/SOCKS5 URLs, runs the fast registration pipeline, and exposes progress, logs, failures, and per-account results.

Success means a configured run produces accounts whose MiniMax flow is `approved`, magpie sign-in is `done`, and the magpie provider list contains the new account user.

## Positioning

The mechanism is a protocol-first batch runner: it performs panel, device-code, email-code, login, and OAuth calls over HTTP, while using isolated browser contexts only where MiniMax requires a real reCAPTCHA Enterprise ticket and Akamai cookies. This is not a browser macro or a generic form filler.

## Operating Context

- Local Windows desktop, with Go, Node.js, Google Chrome, and the repository's `scripts/` protocol helpers available.
- User supplies magpie panel key, domain email suffix, catch-all inbox address, IMAP authorization code, proxy URL, and batch parameters.
- Alternatively, the user can import Hotmail / Outlook Graph accounts in `email----password----clientId----refreshToken` form and use Graph mail receive for verification codes.
- Proxy URLs must accept `socks5://user:pass@host:port`, `http://user:pass@host:port`, compact `scheme://user:pass:host:port`, and escaped `http\://...`; all forms normalize to one local bridge.
- A local HTTP CONNECT bridge is needed because downstream Node helpers and CDP page launches can route through a single local proxy URL even when the configured upstream is SOCKS5.

## Capabilities and Constraints

- GUI configuration for panel URL/key, domain email prefix/suffix, IMAP host/port/user/authorization code, proxy, batch count, prewarm concurrency, and output directory.
- GUI support for Hotmail Graph import, pool status, receive testing, and registration using claimed Graph accounts.
- Local, machine-readable config persistence. Sensitive values stay on the user's machine and are not emitted in logs.
- Start, stop, dry-run, configuration reload, result inspection, and live event streaming.
- HTTP/SOCKS5 proxy normalization and local bridge support.
- External runtime requirements are shown in the UI rather than silently assumed.

## Brand Commitments

No existing brand assets confirmed. The product voice is precise, operational, and Chinese-first where it reflects the user's workflow.

## Evidence on Hand

Validated protocol implementation and final successful run are documented in `PROTOCOL.md`, `artifacts/minimax-flow.json`, `artifacts/api-signin-verify.json`, `artifacts/api-providers-verify.json`, and `artifacts/provider-modal.jpg`.

## Product Principles

- Trust the protocol state, not visual guesses: require `approved`, `done`, and provider-list evidence.
- Keep credentials local and masked in logs.
- Isolate per-account browser context and protocol state.
- Prefer explicit controls and actionable errors over opaque automation.
- Show one clear operational state at a time: idle, running, success, failure.

## Accessibility & Inclusion

Keyboard-operable controls, visible focus, readable contrast, and Chinese labels with technical values left in monospace where they are real identifiers.
