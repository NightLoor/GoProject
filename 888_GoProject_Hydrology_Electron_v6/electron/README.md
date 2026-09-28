# Electron 桌面前端

## 1. 构建 Vue 前端

```powershell
cd frontend
npm install
npm run build
cd ..
```

## 2. 编译 Go 后端

在项目根目录执行：

```powershell
go build -o bin/server.exe ./cmd/server
```

## 3. 安装 Electron 依赖并启动

```powershell
cd electron
npm install
npm start
```

Electron 会自动尝试启动 `bin/server.exe`，然后加载 `frontend/dist/index.html`。

如果找不到后端可执行文件，窗口仍会打开，但实时数据接口不可用。
