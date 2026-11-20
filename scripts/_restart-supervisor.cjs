const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const cfg = JSON.parse(fs.readFileSync(path.join(process.env.APPDATA, 'minimax-registrar', 'config.json'), 'utf8'));
const proxyUrl = fs.readFileSync('C:/Users/dhl/Desktop/m3/artifacts/gui-batch-200/local-proxy.url', 'utf8').trim();

function normalizeProxyUrl(raw) {
  let value = String(raw || '').trim().replace(/\\:/g, ':');
  const compact = /^([a-z][a-z0-9+.-]*):\/\/([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$/i.exec(value);
  if (compact) {
    value = `${compact[1]}://${encodeURIComponent(compact[2])}:${encodeURIComponent(compact[3])}@${compact[4]}:${compact[5]}${compact[6] || ''}`;
  }
  return value;
}

const upstreamProxyUrl = normalizeProxyUrl(cfg.proxyUrl);
const env = {
  ...process.env,
  PANEL_BASE: String(cfg.panelBase || ''),
  PANEL_KEY: String(cfg.panelKey || ''),
  MAIL_MODE: 'hotmail',
  MAIL_DOMAIN: String(cfg.mailDomain || ''),
  MAIL_PREFIX: String(cfg.mailPrefix || ''),
  HOTMAIL_POOL_PATH: String(cfg.hotmailPool || ''),
  COUNT: String(process.env.TARGET_SUCCESS_COUNT || '1000'),
  TARGET_SUCCESS_COUNT: String(process.env.TARGET_SUCCESS_COUNT || '1000'),
  WORKERS: String(process.env.WORKERS || cfg.workers || 8),
  MINIMAX_UPSTREAM_PROXY_URL: process.env.NO_PER_ACCOUNT_PROXY === '1' ? '' : upstreamProxyUrl,
  CHROME_PATH: String(cfg.chromePath || ''),
  CHROME_PROFILE_ROOT: 'C:\\Users\\dhl\\Desktop\\m3\\artifacts\\gui-batch-200\\chrome-profiles',
  BATCH_ROOT: 'C:\\Users\\dhl\\Desktop\\m3\\artifacts\\gui-batch-200',
  BATCH_ID: 'resume-200',
  NODE_NO_WARNINGS: '1',
  NODE_USE_ENV_PROXY: '1',
  HTTP_PROXY: proxyUrl,
  HTTPS_PROXY: proxyUrl,
  ALL_PROXY: proxyUrl,
  http_proxy: proxyUrl,
  https_proxy: proxyUrl,
  all_proxy: proxyUrl,
  MINIMAX_PROXY_URL: proxyUrl,
  NO_PROXY: 'localhost,127.0.0.1,::1',
  no_proxy: 'localhost,127.0.0.1,::1',
  HOTMAIL_CLAIM_COOLDOWN_MS: '180000',
  MINIMAX_CAPTCHA_FAIL_BACKOFF_MS: '90000',
  MINIMAX_SOFT_FAIL_BACKOFF_MS: '30000',
  CAPTCHA_READY_ATTEMPTS: String(process.env.CAPTCHA_READY_ATTEMPTS || '60'),
};
const out = fs.openSync('C:/Users/dhl/Desktop/m3/artifacts/gui-batch-200/supervisor.restart.log', 'w');
const err = fs.openSync('C:/Users/dhl/Desktop/m3/artifacts/gui-batch-200/supervisor.restart.err.log', 'w');
const child = spawn('C:\\Program Files\\nodejs\\node.exe', ['scripts\\minimax-supervisor.mjs'], {
  cwd: 'C:\\Users\\dhl\\Desktop\\m3',
  env,
  detached: true,
  stdio: ['ignore', out, err],
  windowsHide: true,
});
child.unref();
console.log(JSON.stringify({ startedPid: child.pid, proxyUrl }, null, 2));
