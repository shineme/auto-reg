from pathlib import Path
file = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs')
src = file.read_text(encoding='utf-8')
backup = file.with_suffix('.mjs.bak-parallel')
if not backup.exists():
    backup.write_text(src, encoding='utf-8')

def replace_once(label, old, new):
    global src
    if old not in src:
        raise SystemExit(f'missing chunk: {label}')
    src = src.replace(old, new, 1)
    print('patched', label)

replace_once('workers-const',
'''const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
''',
'''const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
const workers = Math.max(1, Math.min(8, Number(process.env.WORKERS || process.env.MINIMAX_WORKERS || 1) || 1));
const upstreamProxyUrl = process.env.MINIMAX_UPSTREAM_PROXY_URL || "";
const bridgeServerPath = process.env.PROXY_BRIDGE_SERVER || path.join(workspace, "app", "cmd", "proxy-bridge-server", "proxy-bridge-server.exe");
''')

replace_once('pending-global',
'''const panelCookies = new Map();
const pendingCommands = new Map();
let nextCommandId = 1;
let progressState = {
''',
'''const panelCookies = new Map();
const panelQueue = { chain: Promise.resolve() };
const stateQueue = { chain: Promise.resolve() };
let progressState = {
''')

file.write_text(src, encoding='utf-8')
print('phase1-basic ok')
