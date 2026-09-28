# Electron 桌面版说明

本项目在原有 Go + Vue 监测系统上增加 Electron 桌面窗口。

- Vue 页面仍然位于 `frontend/`
- Electron 主进程位于 `electron/main.cjs`
- Electron 通过 `bin/server.exe` 启动 Go 后端
- Vue 在桌面模式下自动把 API 请求指向 `http://127.0.0.1:8080`
- 配置、历史数据库和归档目录仍使用项目根目录下的 `configs/`、`data/`

详细步骤请查看 `electron/README.md`。

> Electron 使用 Vue Router Hash 路由，以兼容 `file://` 方式加载 `frontend/dist/index.html`。
