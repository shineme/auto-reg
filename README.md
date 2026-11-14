# auto-reg

Go + Wails GUI and Node protocol helpers for an authorized MiniMax Code (Global) account-registration workflow.

This repository contains source only. It does not contain panel keys, IMAP app passwords, proxy credentials, generated EXEs, browser profiles, runtime artifacts, logs, or local result files. All credentials and site-specific protocol constants are supplied locally and are ignored by Git.

## Components

- `app/`: Go + Wails desktop GUI, proxy bridge, and supervisor launcher.
- `scripts/`: Node protocol helpers, IMAP polling, batch runner, supervisor, and offline tests.
- `PROTOCOL.md`: sanitized protocol notes with placeholders.
- `PRODUCT.md`, `DESIGN.md`, `.impeccable/design.json`: UI design notes.

## Local configuration

Use `scripts/run-minimax-batch.env.example.ps1` as a template. Fill, at minimum:

- `PANEL_BASE`, `PANEL_KEY`
- `IMAP_HOST`, `IMAP_PORT`, `IMAP_USER`, `IMAP_PASSWORD`, `MAIL_DOMAIN`
- `MINIMAX_PROXY_URL` when a proxy is required
- `MINIMAX_SIGNATURE_SALT`, `MINIMAX_YY_SUFFIX`, `MINIMAX_RECAPTCHA_SITE_KEY` from the operator's own authorized integration or provider-published client assets

The GUI stores panel, mail, IMAP, proxy, and batch settings in `%APPDATA%\minimax-registrar\config.json`; that file is not tracked. Site-specific protocol constants are read from the process environment, so set them in the Windows user environment before launching the EXE, or export them in the shell before running the CLI scripts.

## Safety and scope

- No reCAPTCHA or risk-control bypass, token forging, CAPTCHA solving, fingerprint spoofing, or proxy rotation for evasion is implemented.
- reCAPTCHA Enterprise tickets are generated only by a real Chrome session that loads the provider's own page and executes its official widget with the matching action.
- `1200063` / `Verification failed, please try again.` is treated as a controlled mail domain: the batch stops immediately and asks the operator to change the mail domain. It does not attempt to evade risk controls.
- Closing the desktop window stops the supervisor, detached Node helpers, Chrome/CDP processes, and the local proxy bridge; the supervisor status is marked `stopped` and its lock is removed.
- Every account gets a fresh, ephemeral Chrome profile and an OS-assigned CDP port; the Chrome process tree and profile directory are removed after that account finishes or fails.
- Use only panel access, domains, mailboxes, proxies, and accounts you are authorized to operate, and comply with the provider's terms.
- For bulk operations, prefer an official/authorized API or provider test environment. If rate-limited, reduce frequency, verify one consistent egress/session, or contact the provider.

## Validation

```powershell
Get-ChildItem scripts\*.mjs | ForEach-Object { node --check $_.FullName }
node scripts/minimax-resume-test.mjs
node scripts/minimax-domain-risk-test.mjs
node scripts/minimax-profile-lifecycle-test.mjs
cd app
gofmt -w .
go test ./...
```

See `app/README.md` for Wails build instructions and `PROTOCOL.md` for the sanitized protocol description.
