from pathlib import Path
file = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs')
src = file.read_text(encoding='utf-8')

def replace_once(label, old, new):
    global src
    if old not in src:
        raise SystemExit(f'missing chunk: {label}')
    src = src.replace(old, new, 1)
    print('patched', label)

# wrap panelFetch body with queue
old_panel = '''async function panelFetch(pathname, options = {}, expectJson = true) {
  const requestUrl = new URL(pathname, panelBase);
'''
new_panel = '''async function panelFetch(pathname, options = {}, expectJson = true) {
  const run = async () => {
  const requestUrl = new URL(pathname, panelBase);
'''
if old_panel not in src:
    raise SystemExit('missing panelFetch start')
src = src.replace(old_panel, new_panel, 1)
# close panelFetch before next function - find end of panelFetch by locating next async function after it
marker = 'async function cdpVersion(port)'
idx = src.find(marker)
if idx < 0:
    raise SystemExit('missing cdpVersion marker')
# walk back to previous closing brace of panelFetch
before = src.rfind('}', 0, idx)
# insert queue wrapper just before the closing of panelFetch function.
# Safer approach: replace whole function via regex-like manual cut.
start = src.find('async function panelFetch')
end = idx
func = src[start:end]
if 'panelQueue.chain' in func:
    print('panelFetch already queued')
else:
    # replace trailing return/throw end
    if not func.rstrip().endswith('}\n'):
        # ensure ends with closing brace line
        pass
    # inject return await panelQueue around body
    # Find first '{' after signature
    brace = func.find('{')
    body = func[brace+1:]
    # body currently starts with "\n  const run = async () => {\n..." after earlier replace, but closing missing.
    # Rebuild function cleanly.
    # Extract original body from backup for safety.
    bak = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs.bak-parallel').read_text(encoding='utf-8')
    bstart = bak.find('async function panelFetch')
    bend = bak.find('async function cdpVersion(port)')
    original = bak[bstart:bend]
    ob = original.find('{')
    original_body = original[ob+1:original.rfind('}')]
    rebuilt = (
        'async function panelFetch(pathname, options = {}, expectJson = true) {\n'
        '  const run = async () => {' + original_body + '  };\n'
        '  const next = panelQueue.chain.then(run, run);\n'
        '  panelQueue.chain = next.catch(() => {});\n'
        '  return next;\n'
        '}\n\n'
    )
    src = src[:start] + rebuilt + src[end:]
    print('patched panelFetch queue')

# insert helper functions before emitProgress if not present
if 'async function createStickyProxyBridge' not in src:
    helper = '''
function enqueueState(task) {
  const next = stateQueue.chain.then(task, task);
  stateQueue.chain = next.catch(() => {});
  return next;
}

async function createStickyProxyBridge(label = "worker") {
  if (!upstreamProxyUrl) {
    return { proxyUrl, cleanup: async () => {} };
  }
  if (!(await pathExists(bridgeServerPath))) {
    throw new Error(`proxy bridge server missing: ${bridgeServerPath}`);
  }
  const portFile = path.join(root, `proxy-bridge-${String(label).replace(/[^a-zA-Z0-9_-]/g, "-")}-${process.pid}-${crypto.randomBytes(3).toString("hex")}.url`);
  await fs.rm(portFile, { force: true });
  const child = spawn(bridgeServerPath, [upstreamProxyUrl, portFile], { stdio: "ignore", windowsHide: true });
  const deadline = Date.now() + 15000;
  let local = "";
  while (Date.now() < deadline) {
    try {
      local = (await fs.readFile(portFile, "utf8")).trim();
      if (local.startsWith("http://")) break;
    } catch {}
    if (!processIsAlive(child.pid)) throw new Error("proxy bridge exited early");
    await delay(100);
  }
  if (!local.startsWith("http://")) {
    await terminateProcessTree(child);
    throw new Error("proxy bridge did not publish local URL");
  }
  return {
    proxyUrl: local,
    cleanup: async () => {
      await terminateProcessTree(child);
      await fs.rm(portFile, { force: true });
    },
  };
}

'''
    anchor = 'function emitProgress(phase, message, extra = {})'
    pos = src.find(anchor)
    if pos < 0:
        raise SystemExit('missing emitProgress')
    src = src[:pos] + helper + src[pos:]
    print('inserted helpers')
else:
    print('helpers exist')

file.write_text(src, encoding='utf-8')
print('phase4 helpers/panel ok')
