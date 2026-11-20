const cfgPanel = process.env.PANEL_BASE;
const key = process.env.PANEL_KEY;
const url = new URL('/?k=' + encodeURIComponent(key) + '&view=providers&edit=minimax-code-global', cfgPanel);
console.log('url', url.toString().replace(key, '***'));
console.log('proxy', process.env.HTTP_PROXY);
try {
  const res = await fetch(url, { method: 'GET', redirect: 'manual' });
  console.log('status', res.status);
  console.log('set-cookie', res.headers.getSetCookie ? res.headers.getSetCookie().length : 0);
  const text = await res.text();
  console.log('body', text.slice(0, 180).replace(/\s+/g, ' '));
} catch (error) {
  console.log('error', error.message);
  console.log('cause', error.cause && (error.cause.code || error.cause.message || String(error.cause)));
}
