from pathlib import Path
file = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs')
src = file.read_text(encoding='utf-8')

def replace_once(label, old, new):
    global src
    if old not in src:
        raise SystemExit(f'missing chunk: {label}')
    src = src.replace(old, new, 1)
    print('patched', label)

replace_once('chromeProxyArgument',
'''function chromeProxyArgument() {
  if (!proxyUrl) return "";
  const parsed = new URL(proxyUrl);
''',
'''function chromeProxyArgument(localProxy = proxyUrl) {
  if (!localProxy) return "";
  const parsed = new URL(localProxy);
''')

replace_once('startCdpBrowser',
'''async function startCdpBrowser(profileDir) {
  await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  await fs.mkdir(profileDir, { recursive: true });
  const port = await reserveCdpPort();
  await fs.writeFile(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");
  const args = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profileDir}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-networking",
    "--disable-features=Translate",
    "--disable-component-update",
    "--disable-domain-reliability",
    "--no-pings",
    "--window-position=-32000,-32000",
    `--window-size=${screenWidth},${screenHeight}`,
  ];
  const proxyArgument = chromeProxyArgument();
''',
'''async function startCdpBrowser(profileDir, localProxy = proxyUrl) {
  await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  await fs.mkdir(profileDir, { recursive: true });
  const port = await reserveCdpPort();
  await fs.writeFile(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");
  const args = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profileDir}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-networking",
    "--disable-features=Translate",
    "--disable-component-update",
    "--disable-domain-reliability",
    "--no-pings",
    "--window-position=-32000,-32000",
    `--window-size=${screenWidth},${screenHeight}`,
  ];
  const proxyArgument = chromeProxyArgument(localProxy);
''')

file.write_text(src, encoding='utf-8')
print('phase3 ok')
