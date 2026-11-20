import re, glob
for f in sorted(glob.glob('711-proxy-*')):
    html = open(f, encoding='utf-8', errors='ignore').read()
    m = re.search(r'<article[^>]*>(.*?)</article>', html, flags=re.S) or re.search(r'<main[^>]*>(.*?)</main>', html, flags=re.S)
    seg = m.group(1) if m else html
    text = re.sub(r'<script[^>]*>.*?</script>', '', seg, flags=re.S)
    text = re.sub(r'<style[^>]*>.*?</style>', '', text, flags=re.S)
    text = re.sub(r'<h[1-6][^>]*>', '\n## ', text)
    text = re.sub(r'<li[^>]*>', '\n- ', text)
    text = re.sub(r'<br[^>]*>', '\n', text)
    text = re.sub(r'</(p|div|tr|pre)>', '\n', text)
    text = re.sub(r'<[^>]+>', ' ', text)
    text = re.sub(r'&nbsp;', ' ', text)
    text = re.sub(r'&amp;', '&', text)
    text = re.sub(r'&lt;', '<', text)
    text = re.sub(r'&gt;', '>', text)
    text = re.sub(r'&quot;', '"', text)
    text = re.sub(r'[ \t]+', ' ', text)
    lines = [l.strip() for l in text.split('\n') if l.strip()]
    out = f.replace('.md', '') + '_txt.txt'
    open(out, 'w', encoding='utf-8').write('\n'.join(lines))
    print(out, len(lines), 'lines')
