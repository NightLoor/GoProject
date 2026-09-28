# 水利监测系统（Electron v6）

基于 **Go、Gin、Vue 3、SQLite 与 Electron** 的水利监测应用。后端统一管理 Modbus TCP 和 OPC DA 采集、实时点位、历史数据及报警；Vue 提供 Web 监控界面，Electron 提供 Windows 桌面运行入口。

> 本文根据当前仓库代码和配置整理。现场 OPC DA / Kepware 连接、Windows 安装包运行和设备写入仍需在目标环境验证。

## 功能概览

- 通过配置启用 Modbus TCP、OPC DA 驱动，并以统一点位模型管理采集数据。
- 通过 HTTP API 查询健康状态、配置、实时点位、设备状态、历史数据、事件与报警。
- 将启用历史记录的点位采样写入 SQLite；支持保留、归档和查询点数配置。
- 从 `Alarm.csv` 加载报警配置，并通过 API 查询报警、报警历史和更新配置。
- 支持可写点位写入接口。
- Vue 3 + Vite 前端支持开发服务器和后端静态托管。
- Electron 启动时检查本机 API，并尝试启动随应用提供的 Go 后端程序。

## 架构

```text
Vue 3（浏览器 / Electron）
          │ HTTP / JSON
          ▼
      Gin REST API
          │
          ▼
 Monitor Service ── IO Manager
      │                    │
      ├─ Modbus TCP         ├─ 实时点位与报警
      └─ OPC DA             └─ SQLite 历史存储
                                  │
                         data/history.db
```

## 目录结构

```text
888_GoProject_Hydrology_Electron_v6/
├── cmd/server/       # Go 服务入口
├── internal/
│   ├── api/          # Gin 路由、HTTP Handler、前端静态托管
│   ├── config/       # JSON / CSV 配置加载与校验
│   ├── driver/       # 多驱动组合
│   ├── history/      # SQLite 历史存储与维护
│   ├── io/           # 实时点位、历史策略和报警状态
│   ├── modbus/       # Modbus TCP 驱动
│   ├── opcda/        # OPC DA 驱动
│   ├── model/        # 配置及数据模型
│   └── service/      # 监控业务服务
├── configs/          # Modbus、OPC DA、点位、报警、历史配置
├── frontend/         # Vue 3 + Vite 前端
├── electron/         # Electron 主进程、preload、打包配置
├── bin/              # Electron 启动时查找的后端可执行文件
├── data/             # SQLite 数据库及归档目录
├── go.mod
└── README.md
```

## 配置文件

服务从项目根目录的 `configs/` 读取配置：

| 文件 | 用途 |
|---|---|
| `modbus.json` | Modbus TCP 启用状态、轮询周期、设备地址、超时和批量读取参数 |
| `opcda.json` | OPC DA 启用状态、ProgID、节点、超时、重连周期和组名 |
| `Tags.csv` | Modbus 与 OPC DA 点位定义，含地址、类型、读写权限、比例/偏移、描述及历史开关 |
| `Alarm.csv` | 点位报警启用状态、报警值或上下限、单位和消息 |
| `history.json` | 历史采样、数据保留、归档维护和查询点数设置 |

当前提交中的默认值：Modbus TCP 未启用；OPC DA 已启用，ProgID 为 `Kepware.KEPServerEX.V6`，连接节点为 `localhost`。历史存储已启用，采样间隔 1 分钟、保留期 730 天、归档已启用、单次查询上限 2000 点。实际运行行为以部署环境中的配置文件为准。

`Tags.csv` 中 `history=true` 的点位参与历史持久化；`false` 的点位不写历史。请确认 CSV 的点位名称、协议设备名称、类型和现场地址与设备配置匹配。只有确实允许远程写入的点位才应设置为可写。

程序会在当前工作目录下使用 `data/history.db`，并按历史配置管理归档。部署时应确保该目录存在或可创建且对运行账户可写；Electron 安装后的可写路径需在目标机器验证。

## API

默认监听 `http://localhost:8080`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 服务健康状态 |
| GET | `/api/config` | 当前 Modbus、OPC DA 和点位配置 |
| GET | `/api/io` | 查询全部实时点位 |
| GET | `/api/io/:name` | 查询单个点位 |
| GET | `/api/io/:name/history?limit=60` | 查询点位历史；也支持 `start_date`、`end_date` 参数 |
| GET | `/api/devices` | 查询设备状态 |
| GET | `/api/events?limit=50` | 查询报警/恢复事件 |
| GET | `/api/alarm-history` | 查询报警历史；前端传入 `tag`、`start_date`、`end_date` |
| GET | `/api/alarms` | 查询当前报警 |
| GET | `/api/alarm-config` | 查询报警配置 |
| POST | `/api/alarm-config` | 更新报警配置 |
| POST | `/api/io/write` | 写入可写点位 |

写入请求示例：

```http
POST /api/io/write
Content-Type: application/json

{"tag":"Pump1_Run","value":true}
```

具体可写点位取决于 `Tags.csv` 中的权限配置和驱动支持。写入前请确认目标设备、点位和数据值。

## 开发运行

需要 Go（模块声明为 Go 1.22）、Node.js / npm；使用 OPC DA 时还需要 Windows 上可用的 OPC DA Server（当前配置示例为 Kepware）。

在项目目录 `888_GoProject_Hydrology_Electron_v6` 下运行：

### 启动后端

```powershell
go run ./cmd/server
```

后端读取 `configs/` 下配置，默认监听 `:8080`。如果 `frontend/dist/index.html` 存在，Gin 同时提供前端页面。

### 启动 Vue 开发服务器

另开终端：

```powershell
cd frontend
npm install
npm run dev
```

打开 `http://localhost:5173`。Vite 将 `/api` 请求代理到 `http://localhost:8080`。

### 构建 Web 前端

```powershell
cd frontend
npm install
npm run build
```

构建结果位于 `frontend/dist/`。启动 Go 服务后可通过 `http://localhost:8080` 访问。

## Electron 桌面版

Electron 主进程位于 `electron/main.cjs`，使用 `preload.cjs` 向渲染进程提供桌面端 API 地址。启动时它会：

1. 检查 `http://localhost:8080/api/health`；若已有服务响应，则不再启动第二个后端。
2. 否则查找 `bin/server.exe` 或项目根目录的 `server.exe` 并尝试启动。
3. 加载 `frontend/dist/index.html`；若文件不存在，则尝试加载 Vite 开发地址 `http://127.0.0.1:5173` 并显示构建提示。

在项目根目录构建前端和 Go 后端：

```powershell
cd frontend
npm install
npm run build
cd ..
go build -o bin/server.exe ./cmd/server
```

然后安装 Electron 依赖并启动：

```powershell
cd electron
npm install
npm start
```

`electron/package.json` 定义了 `npm run dist` 打包脚本及 Windows NSIS 安装包目标。打包前应先完成前端构建和 Go 后端编译。请在实际安装包中确认 `frontend/dist`、`configs`、`bin/server.exe` 能被 Electron 正确定位，并验证数据库目录可写；开发目录下运行成功不等同于安装包已验证。

## 历史数据与报警

- SQLite 数据库默认路径为 `data/history.db`，服务启动时打开数据库并启动维护任务。
- `history.json` 控制历史采样开关、采样间隔、保留期、归档目录、清理周期和查询上限。
- `Tags.csv` 的 `history` 字段控制点位是否写入历史库。
- `Alarm.csv` 定义每个点位的报警配置；后端 API 提供报警、报警历史和配置读取/更新能力。
- 前端历史接口支持按点位查询采样记录；报警历史接口支持按标签和日期范围查询。

## 常见排查

- **后端启动时报配置错误**：检查五个配置文件是否存在、CSV 表头和字段是否符合当前格式，以及设备/点位名称是否匹配。
- **前端无法访问 API**：确认 Go 服务监听 `8080`；开发模式确认 Vite 使用 `5173`；Electron 模式检查启动日志和已有的 `localhost:8080` 服务。
- **Electron 显示构建提示或空页面**：先执行 `npm run build`，确认 `frontend/dist/index.html` 存在。
- **Modbus 无数据**：当前仓库默认关闭 Modbus；检查 `configs/modbus.json` 中启用状态、设备地址和网络连通性。
- **OPC DA 无数据**：检查 Kepware ProgID、节点、OPC DA 服务状态及 Windows COM/DCOM 环境。
- **历史记录未增长**：检查 `history.json` 的 `enabled`、点位 `history` 字段，以及 `data/` 目录的写权限。


