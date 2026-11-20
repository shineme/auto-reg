const proxy = process.env.HTTP_PROXY;
console.log('proxy', proxy);
try {
  const res = await fetch('https://web.open-traffic.com/', { method: 'GET', redirect: 'manual' });
  console.log('status', res.status);
  console.log('ok', true);
} catch (error) {
  console.log('error', error.message);
  console.log('cause', error.cause && (error.cause.message || String(error.cause)));
}
