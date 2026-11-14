import fs from "node:fs";
import https from "node:https";
const url = process.env.PAGE_URL || 'https://account.minimax.io/oauth-authorize?client_id=mcode-public&user_code=TEST-CODE';
https.get(url, { headers: { 'User-Agent': 'Mozilla/5.0', Accept: 'text/html' } }, (res) => {
  let body = '';
  res.on('data', (chunk) => { body += chunk; });
  res.on('end', () => {
    fs.mkdirSync('artifacts', { recursive: true });
    fs.writeFileSync('artifacts/oauth-authorize.html', body);
    const scripts = Array.from(body.matchAll(/src="([^"]+)"/g), (match) => match[1]).slice(0, 20);
    const text = body.replace(/<script[\s\S]*?<\/script>/g, ' ').replace(/<style[\s\S]*?<\/style>/g, ' ').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').slice(0, 1000);
    console.log(JSON.stringify({ status: res.statusCode, headers: res.headers, bytes: body.length, scripts, text }, null, 2));
  });
}).on('error', (error) => console.error(error.message));
