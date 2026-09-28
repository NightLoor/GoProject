const { app, BrowserWindow, dialog } = require('electron');
const { spawn } = require('node:child_process');
const http = require('node:http');
const path = require('node:path');
const fs = require('node:fs');

let backend = null;

function projectRoot() {
  return path.resolve(__dirname, '..');
}

function checkBackend() {
  return new Promise((resolve) => {
    const req = http.get('http://localhost:8080/api/health', (res) => {
      res.resume();
      resolve(res.statusCode >= 200 && res.statusCode < 500);
    });
    req.setTimeout(800, () => { req.destroy(); resolve(false); });
    req.on('error', () => resolve(false));
  });
}

async function startBackend() {
  if (await checkBackend()) {
    console.log('[backend] Existing backend detected at http://localhost:8080; skip starting another server.');
    return false;
  }
  const root = projectRoot();
  const candidates = process.platform === 'win32'
    ? [path.join(root, 'bin', 'server.exe'), path.join(root, 'server.exe')]
    : [path.join(root, 'bin', 'server'), path.join(root, 'server')];
  const executable = candidates.find((p) => fs.existsSync(p));
  if (!executable) return false;
  backend = spawn(executable, [], { cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
  backend.stdout?.on('data', (data) => console.log(`[backend] ${data}`));
  backend.stderr?.on('data', (data) => console.error(`[backend] ${data}`));
  backend.on('exit', (code, signal) => console.error('Backend exited:', code, signal));
  backend.on('error', (err) => console.error('Backend start failed:', err));
  return true;
}

function createWindow() {
  const win = new BrowserWindow({
    width: 1440,
    height: 920,
    minWidth: 1100,
    minHeight: 720,
    backgroundColor: '#f3f6fb',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    }
  });

  const indexFile = path.join(projectRoot(), 'frontend', 'dist', 'index.html');
  win.webContents.on('console-message', (_event, level, message, line, sourceId) => {
    console.log(`[renderer:${level}] ${message} (${sourceId}:${line})`);
  });
  win.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL) => {
    console.error('Renderer load failed:', errorCode, errorDescription, validatedURL);
  });

  if (fs.existsSync(indexFile)) {
    win.loadFile(indexFile);
  } else {
    win.loadURL('http://127.0.0.1:5173');
    dialog.showMessageBox(win, {
      type: 'info',
      title: '前端构建提示',
      message: '未找到 frontend/dist/index.html',
      detail: '请先执行 npm run build，或启动 Vite 开发服务器。'
    });
  }
}

app.whenReady().then(() => {
  startBackend().finally(() => createWindow());
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (backend && !backend.killed) backend.kill();
  if (process.platform !== 'darwin') app.quit();
});

app.on('before-quit', () => {
  if (backend && !backend.killed) backend.kill();
});
