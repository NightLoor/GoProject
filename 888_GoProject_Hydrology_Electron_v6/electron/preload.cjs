const { contextBridge } = require('electron');

contextBridge.exposeInMainWorld('desktopApp', {
  platform: process.platform,
  isElectron: true,
  apiBase: 'http://localhost:8080'
});
