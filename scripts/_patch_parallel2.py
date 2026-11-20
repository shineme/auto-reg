from pathlib import Path
file = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs')
src = file.read_text(encoding='utf-8')

def replace_once(label, old, new):
    global src
    if old not in src:
        raise SystemExit(f'missing chunk: {label}')
    src = src.replace(old, new, 1)
    print('patched', label)

replace_once('cdpCall',
'''function cdpCall(socket, method, params = {}, sessionId) {
  const id = nextCommandId++;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pendingCommands.delete(id);
      reject(new Error(`CDP ${method} timed out`));
    }, 30000);
    pendingCommands.set(id, {
      resolve: (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      reject: (error) => {
        clearTimeout(timer);
        reject(error);
      },
    });
    const message = { id, method, params };
    if (sessionId) message.sessionId = sessionId;
    try {
      socket.send(JSON.stringify(message));
    } catch (error) {
      clearTimeout(timer);
      pendingCommands.delete(id);
      reject(error);
    }
  });
}
''',
'''function cdpCall(socket, pendingCommands, nextIdRef, method, params = {}, sessionId) {
  const id = nextIdRef.value++;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pendingCommands.delete(id);
      reject(new Error(`CDP ${method} timed out`));
    }, 30000);
    pendingCommands.set(id, {
      resolve: (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      reject: (error) => {
        clearTimeout(timer);
        reject(error);
      },
    });
    const message = { id, method, params };
    if (sessionId) message.sessionId = sessionId;
    try {
      socket.send(JSON.stringify(message));
    } catch (error) {
      clearTimeout(timer);
      pendingCommands.delete(id);
      reject(error);
    }
  });
}
''')

replace_once('connectCdpSocket',
'''function connectCdpSocket(version) {
  const socket = new WebSocket(version.webSocketDebuggerUrl);
  let readySettled = false;
  let rejectReady;
  const ready = new Promise((resolve, reject) => {
    rejectReady = reject;
    socket.onopen = () => {
      readySettled = true;
      resolve();
    };
    socket.onerror = () => {
      if (readySettled) return;
      readySettled = true;
      reject(new Error("CDP WebSocket connection failed"));
    };
  });
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (!message.id || !pendingCommands.has(message.id)) return;
    const { resolve, reject } = pendingCommands.get(message.id);
    pendingCommands.delete(message.id);
    if (message.error) reject(new Error(JSON.stringify(message.error)));
    else resolve(message.result);
  };
  socket.onclose = () => {
    if (!readySettled) {
      readySettled = true;
      rejectReady(new Error("CDP socket closed before open"));
    }
    for (const { reject } of pendingCommands.values()) reject(new Error("CDP socket closed"));
    pendingCommands.clear();
  };
  return { socket, ready, call: (method, params, sessionId) => cdpCall(socket, method, params, sessionId) };
}
''',
'''function connectCdpSocket(version) {
  const socket = new WebSocket(version.webSocketDebuggerUrl);
  const pendingCommands = new Map();
  const nextIdRef = { value: 1 };
  let readySettled = false;
  let rejectReady;
  const ready = new Promise((resolve, reject) => {
    rejectReady = reject;
    socket.onopen = () => {
      readySettled = true;
      resolve();
    };
    socket.onerror = () => {
      if (readySettled) return;
      readySettled = true;
      reject(new Error("CDP WebSocket connection failed"));
    };
  });
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (!message.id || !pendingCommands.has(message.id)) return;
    const { resolve, reject } = pendingCommands.get(message.id);
    pendingCommands.delete(message.id);
    if (message.error) reject(new Error(JSON.stringify(message.error)));
    else resolve(message.result);
  };
  socket.onclose = () => {
    if (!readySettled) {
      readySettled = true;
      rejectReady(new Error("CDP socket closed before open"));
    }
    for (const { reject } of pendingCommands.values()) reject(new Error("CDP socket closed"));
    pendingCommands.clear();
  };
  return {
    socket,
    ready,
    pendingCommands,
    call: (method, params, sessionId) => cdpCall(socket, pendingCommands, nextIdRef, method, params, sessionId),
  };
}
''')

replace_once('withCdpBrowser',
'''async function withCdpBrowser(owner, worker) {
  const label = String(owner?.index ?? owner?.name ?? "session").replace(/[^a-zA-Z0-9_-]/g, "-");
  const profileDir = path.join(chromeProfileRoot, `${batchId}-${label}-${process.pid}-${crypto.randomBytes(4).toString("hex")}`);
  let browser;
  let cdp;
  try {
    browser = await startCdpBrowser(profileDir);
    cdp = await connectCdpBrowser(browser.version);
    return await worker(cdp, browser);
  } finally {
    try {
      await stopCdpBrowser(browser, cdp);
    } finally {
      await removeCdpProfile(browser);
    }
  }
}
''',
'''async function withCdpBrowser(owner, worker, localProxy = proxyUrl) {
  const label = String(owner?.index ?? owner?.name ?? "session").replace(/[^a-zA-Z0-9_-]/g, "-");
  const profileDir = path.join(chromeProfileRoot, `${batchId}-${label}-${process.pid}-${crypto.randomBytes(4).toString("hex")}`);
  let browser;
  let cdp;
  try {
    browser = await startCdpBrowser(profileDir, localProxy);
    cdp = await connectCdpBrowser(browser.version);
    return await worker(cdp, browser);
  } finally {
    try {
      await stopCdpBrowser(browser, cdp);
    } finally {
      await removeCdpProfile(browser);
    }
  }
}
''')

file.write_text(src, encoding='utf-8')
print('phase2 ok')
