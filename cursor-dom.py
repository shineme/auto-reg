import re
import subprocess
import sys

CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"
URL = ("https://cursor.com/loginDeepControl?challenge=abc&uuid=11112222-3333-4444-5555-666677778888"
       "&mode=login&redirectTarget=cli")

out = subprocess.run(
    [CHROME, "--headless=new", "--disable-gpu", "--virtual-time-budget=12000",
     "--dump-dom", URL],
    capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=120)
html = out.stdout
print("dom len:", len(html))
open(r"C:\Users\dhl\Desktop\m3\.uibuild\cursor-dom.html", "w", encoding="utf-8").write(html)

for m in re.findall(r"<iframe[^>]*>", html):
    print("IFRAME:", m[:200])

for kw in ("Sign in", "Continue with", "Log in", "sign up", "Sign up"):
    i = html.find(kw)
    if i >= 0:
        print("CTX", kw, ":", re.sub(r"\s+", " ", html[max(0, i - 260):i + 260])[:520])

body = html[html.find("<body"):]
body = re.sub(r"<script.*?</script>", " ", body, flags=re.S)
body = re.sub(r"<style.*?</style>", " ", body, flags=re.S)
text = re.sub(r"<[^>]+>", "\n", body)
lines = [l.strip() for l in text.split("\n") if l.strip()]
print("--- visible text (first 60):")
for l in lines[:60]:
    print("   ", l[:120])
print("--- type=email:", 'type="email"' in html, "| placeholder email:",
      "placeholder" in html and "mail" in html.lower())

inputs = re.findall(r"<input[^>]*>", html)
print("--- inputs:", len(inputs))
for i in inputs[:20]:
    print("  ", i[:220])

btns = re.findall(r"<button[^>]*>.{0,120}", html, re.S)
print("--- buttons:", len(btns))
for b in btns[:20]:
    print("  ", re.sub(r"\s+", " ", b)[:180])

for kw in ("Sign in", "Log in", "Continue", "email", "verification code", "sign up"):
    if kw.lower() in html.lower():
        print("HAS:", kw)
