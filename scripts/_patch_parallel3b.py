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
'function chromeProxyArgument() {\n  if (!proxyUrl) return "";\n  const parsed = new URL(proxyUrl);',
'function chromeProxyArgument(localProxy = proxyUrl) {\n  if (!localProxy) return "";\n  const parsed = new URL(localProxy);')

replace_once('startCdpBrowser-sig',
'async function startCdpBrowser(profileDir) {',
'async function startCdpBrowser(profileDir, localProxy = proxyUrl) {')

replace_once('proxyArgument-call',
'  const proxyArgument = chromeProxyArgument();',
'  const proxyArgument = chromeProxyArgument(localProxy);')

file.write_text(src, encoding='utf-8')
print('phase3 ok')
