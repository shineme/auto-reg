const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const cfg = JSON.parse(fs.readFileSync(path.join(process.env.APPDATA, 'minimax-registrar', 'config.json'), 'utf8'));
const raw = String(cfg.proxyUrl || '').trim().replace(/\\:/g, ':');
const match = /^([a-z][a-z0-9+.-]*):\/\/([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$/i.exec(raw);
const upstream = match
  ? `${match[1]}://${encodeURIComponent(match[2])}:${encodeURIComponent(match[3])}@${match[4]}:${match[5]}${match[6] || ''}`
  : raw;
const child = spawn(process.execPath, ['scripts/minimax-fast-batch.mjs'], {
  cwd: 'C:\\Users\\dhl\\Desktop\\m3',
  env: {
    ...process.env,
    DRY_RUN: '1',
    MINIMAX_UPSTREAM_PROXY_URL: upstream,
    CAPTCHA_READY_ATTEMPTS: '120',
    BATCH_ROOT: 'C:\\Users\\dhl\\Desktop\\m3\\artifacts\\gui-batch-200',
    BATCH_ID: 'dryrun',
    CHROME_PROFILE_ROOT: 'C:\\Users\\dhl\\Desktop\\m3\\artifacts\\gui-batch-200\\chrome-profiles',
  },
  stdio: 'inherit',
});
child.on('close', (code) => process.exit(code));

