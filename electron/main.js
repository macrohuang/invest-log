const { app, BrowserWindow, dialog } = require('electron');
const path = require('path');
const fs = require('fs');
const os = require('os');
const net = require('net');
const http = require('http');
const { spawn } = require('child_process');

let mainWindow;
let backendProcess;
let backendPort;
let quitting = false;

function getDataDir() {
  if (process.platform === 'darwin') {
    return path.join(os.homedir(), 'Library', 'Application Support', 'com.investlog.InvestLog');
  }
  if (process.platform === 'win32') {
    return path.join(process.env.APPDATA || path.join(os.homedir(), 'AppData', 'Roaming'), 'InvestLog');
  }
  return path.join(os.homedir(), '.config', 'investlog');
}

function resolveBackendPath() {
  if (process.env.INVEST_LOG_BACKEND_PATH) {
    return process.env.INVEST_LOG_BACKEND_PATH;
  }
  const archNames = [
    'invest-log-backend-aarch64-apple-darwin',
    'invest-log-backend-x86_64-apple-darwin',
    'invest-log-backend-x86_64-pc-windows-msvc.exe',
    'invest-log-backend-x86_64-unknown-linux-gnu',
  ];
  if (app.isPackaged) {
    const base = process.resourcesPath;
    for (const name of archNames) {
      const candidate = path.join(base, 'backend', name);
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    }
    if (process.platform === 'win32') {
      const winCandidate = path.join(base, 'backend', 'invest-log-backend.exe');
      if (fs.existsSync(winCandidate)) {
        return winCandidate;
      }
      return path.join(base, 'invest-log-backend.exe');
    }
    const fallback = path.join(base, 'backend', 'invest-log-backend');
    if (fs.existsSync(fallback)) {
      return fallback;
    }
    return path.join(base, 'invest-log-backend');
  }
  const devCandidates = [
    path.join(__dirname, '..', 'backend-dist', 'invest-log-backend'),
    path.join(__dirname, '..', 'backend-dist', 'invest-log-backend.exe'),
    ...archNames.map((name) => path.join(__dirname, '..', 'backend-dist', name)),
  ];
  for (const candidate of devCandidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  return null;
}

function isPortFree(port) {
  return new Promise((resolve) => {
    const server = net.createServer()
      .once('error', () => resolve(false))
      .once('listening', () => {
        server.close(() => resolve(true));
      })
      .listen(port, '127.0.0.1');
  });
}

async function pickPort(preferred) {
  if (await isPortFree(preferred)) {
    return preferred;
  }
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
  });
}

function requestHealth(port) {
  return new Promise((resolve) => {
    const req = http.get({
      host: '127.0.0.1',
      port,
      path: '/api/health',
      timeout: 2000,
    }, (res) => {
      res.resume();
      resolve(res.statusCode && res.statusCode < 500);
    });
    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
  });
}

async function waitForBackend(port, timeoutMs) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const ok = await requestHealth(port);
    if (ok) {
      return true;
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  return false;
}

function startBackend(port) {
  const backendPath = resolveBackendPath();
  if (!backendPath) {
    dialog.showErrorBox('Backend Not Found', 'Cannot find backend binary. Please build it first.');
    return;
  }
  const dataDir = getDataDir();
  fs.mkdirSync(dataDir, { recursive: true });

  const args = ['--data-dir', dataDir, '--port', String(port)];
  backendProcess = spawn(backendPath, args, {
    env: {
      ...process.env,
      INVEST_LOG_DATA_DIR: dataDir,
      INVEST_LOG_PARENT_WATCH: '1',
    },
    stdio: 'ignore',
    detached: true,
  });

  backendProcess.on('exit', (code) => {
    if (quitting) return;
    dialog.showErrorBox('Backend Exited', `Backend process exited (code ${code}).`);
  });
}

function stopBackend() {
  if (!backendProcess || backendProcess.killed) {
    return;
  }
  try {
    process.kill(-backendProcess.pid, 'SIGTERM');
  } catch (err) {
    try {
      backendProcess.kill('SIGTERM');
    } catch (_) {}
  }
  setTimeout(() => {
    try {
      process.kill(-backendProcess.pid, 'SIGKILL');
    } catch (err) {
      try {
        backendProcess.kill('SIGKILL');
      } catch (_) {}
    }
  }, 2000);
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    show: true,
    backgroundColor: '#f8fafc',
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  });

  mainWindow.loadFile(path.join(__dirname, 'loading.html'));

  mainWindow.on('close', () => {
    if (!quitting) {
      app.quit();
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

async function boot() {
  createWindow();
  backendPort = await pickPort(8000);
  startBackend(backendPort);

  const ok = await waitForBackend(backendPort, 60000);
  if (!ok) {
    dialog.showErrorBox('Startup Timeout', 'Backend did not respond in time.');
    return;
  }

  const uiPath = path.join(__dirname, '..', 'static', 'index.html');
  mainWindow.loadFile(uiPath, {
    query: {
      api: `http://127.0.0.1:${backendPort}`,
      t: String(Date.now()),
    },
  });
}

app.on('ready', boot);

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  if (!mainWindow) {
    boot();
  }
});

app.on('before-quit', () => {
  quitting = true;
  stopBackend();
});
