# 水利监测系统（Go + Vue 3 + Electron）

面向水雨情与渗压监测的桌面及 Web 应用。Go 后端通过 Modbus TCP、OPC DA 采集点位数据，提供 Gin HTTP API 和 SQLite 历史/报警存储；Vue 3 + Vite 实现监控界面；Electron 提供 Windows 桌面入口。

> 现场设备通信及 Windows 安装包需要在目标环境验证。OPC DA 需要 Windows 和可访问的 OPC DA Server（当前配置使用 Kepware）。

## 功能

- Modbus TCP 与 OPC DA 两种采集驱动可独立启用，点位统一配置在 `configs/Tags.csv`。
- 实时查看三个站点的水位、降雨量、渗压点位和设备状态。
- 查看设备变量报警、活动报警、报警事件及历史报警记录；报警规则可由前端编辑并保存。
- 查询 SQLite 历史数据，按站点、变量类型、点位和日期范围查看趋势与采样明细。
- 对标记为可写的点位执行写入。
- 通过 Gin 提供 REST API，并可托管前端构建产物；Electron 可启动本地 Go 服务并加载 Vue 页面。

## 系统结构

```text
Vue 3 Web / Electron 界面
          │ HTTP / JSON
          ▼
      Gin REST API
          │
          ▼
   Monitor Service ── IO Manager
      │                    │
      ├─ Modbus TCP         ├─ 实时点位、质量、报警和事件
      └─ OPC DA             └─ SQLite 历史存储与维护
                                  │
                           data/history.db
```

## 目录

```text
GoProject/
├── cmd/server/             # 后端入口
├── internal/
│   ├── api/                 # Gin 路由和 HTTP Handler
│   ├── config/              # JSON/CSV 加载、校验与保存
│   ├── driver/              # 驱动接口及组合
│   ├── history/              # SQLite 历史与维护
│   ├── io/                   # 实时点位、历史采样和报警状态
│   ├── modbus/               # Modbus TCP 驱动
│   ├── opcda/                # Windows OPC DA 驱动与非 Windows stub
│   ├── model/                # 配置和数据模型
│   └── service/              # 监控业务服务
├── configs/                  # 设备、点位、报警和历史配置
├── frontend/                 # Vue 3 + Vite 页面与组件
├── electron/                 # Electron 主进程、preload 和打包配置
├── bin/                      # Electron 查找 Go 后端可执行文件的位置
├── data/                     # SQLite 数据库与归档数据
├── go.mod
└── README.md
```

## 界面

前端使用 Vue Router Hash 模式，兼容 Electron 通过 `file://` 加载构建页面。默认进入水雨情监测。

- `#/water-rain`：坝仔（BZ）、淡塘（DT）、稻元（DY）站点的水位、降雨和趋势。
- `#/seepage`：每站 8 个渗压测点、单点趋势及统计信息。
- `#/alarms`：报警规则、当前报警和事件记录。
- `#/history`：按站点、变量类别、点位和日期范围查询历史数据。

当前 Tags 点表有 38 个变量：8 个 Modbus 点和 30 个 OPC DA 点。每个站点配置 8 个渗压点、1 个水位点和 1 个降雨点。

## 配置

后端从仓库根目录的 `configs/` 加载以下文件：

| 文件 | 内容 |
|---|---|
| `modbus.json` | Modbus 启用开关、设备地址、轮询周期、超时与批量读取参数 |
| `opcda.json` | OPC DA 启用开关、ProgID、节点、超时、重连周期和组名 |
| `Tags.csv` | 两种协议共用的点位表：设备名、变量名、地址/ItemID、数据类型、读写权限、比例、偏移、字节序、说明和历史开关 |
| `Alarm.csv` | 报警启用状态、布尔报警值或数值上下限、单位和消息 |
| `history.json` | 历史采样开关、采样间隔、保留期、归档策略、清理周期和查询上限 |

`Tags.csv` 的 `device` 必须匹配相应 JSON 中配置的设备名；`tag_name` 应唯一。点位的 `history=true` 才会保存到历史库。只应将允许远程写入的变量标记为 `writable=true`。

当前仓库默认配置：Modbus 关闭；OPC DA 开启，ProgID 为 `Kepware.KEPServerEX.V6`，节点为 `localhost`。这只是仓库配置，部署时应按实际设备修改。

### OPC DA 环境

OPC DA 基于 Windows COM/DCOM，不是 OPC UA，不使用 `opc.tcp://` Endpoint。`prog_id` 是 Windows OPC DA Server ProgID，`node` 是服务器主机名；远程服务器还需要配置 DCOM 权限和网络/RPC 通信。真实驱动仅在 Windows 构建，其他平台提供 stub，不模拟实际连接。

## 历史数据

- 主数据库：`data/history.db`；归档数据库默认位于 `data/archive/history_archive.db`。
- 默认全局启用，每个启用历史的点位每分钟最多保存一条。
- 主库保留 730 天；归档启用时迁移超期数据，归档数据保留 3650 天；维护默认每 24 小时运行。
- 单点历史查询最多返回 2000 个点；超出时在数据库侧降采样并保留首尾点。
- 历史页面和后端都限制日期范围不超过 10 个自然日，结束日期当天包含在结果中。
- 数据库和归档路径相对于服务当前工作目录。运行账户需要对 `data/` 有创建和写入权限。

以上为当前仓库 `configs/history.json` 中的默认值，可按部署需求调整。

## 报警

`configs/Alarm.csv` 的列为：

```text
tag,data_type,enabled,alarm_value,low,high,unit,message
```

- 布尔变量可设置 `alarm_value=true` 或 `false`；匹配该值时触发报警。
- 整数/浮点变量使用 `low`、`high` 设定边界，可只启用其中一个。
- 前端通过报警配置 API 保存后，后端校验并更新 IOManager 规则，无需重启服务。
- 数值报警事件包括 `alarm_high`、`alarm_low`、`alarm_recovered`；布尔报警包括 `alarm_bool_true`、`alarm_bool_false`、`alarm_recovered`。
- 活动报警状态由当前采集值计算。报警触发与恢复记录保存在 SQLite 的 `alarm_events` 表；通讯质量事件 `quality_bad`、`quality_recovered` 目前仅保存在运行期事件列表，不写入历史报警表。

## HTTP API

服务默认监听 `http://localhost:8080`，响应数据采用 JSON。

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/health` | 服务和设备连接概况 |
| GET | `/api/config` | 当前配置 |
| GET | `/api/io` | 全部实时点位 |
| GET | `/api/io/:name` | 单个点位 |
| GET | `/api/io/:name/history?limit=60` | 按样本数量查询历史（兼容查询） |
| GET | `/api/io/:name/history?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD` | 按日期查询历史，跨度最多 10 天 |
| GET | `/api/devices` | 设备状态 |
| GET | `/api/events?limit=50` | 报警/恢复和运行期事件 |
| GET | `/api/alarms` | 当前活动报警 |
| GET | `/api/alarm-history?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&tag=...` | 查询历史报警；`tag` 可选 |
| GET | `/api/alarm-config` | 读取报警规则 |
| POST | `/api/alarm-config` | 校验并保存报警规则 |
| POST | `/api/io/write` | 写入允许写入的点位 |

写入示例：

```http
POST /api/io/write
Content-Type: application/json

{"tag":"Pump1_Run","value":true}
```

请在真实设备环境中谨慎使用写入接口，并确认点位权限和值符合设备要求。

## 开发运行

需要 Go 1.25.0 或兼容版本、Node.js/npm。OPC DA 采集需要 Windows 及可用的 OPC DA Server。

### 后端

在仓库根目录启动：

```powershell
go run ./cmd/server
```

后端读取 `configs/` 下的 JSON/CSV，监听 `:8080`，并使用 `data/history.db`。如果存在 `frontend/dist/index.html`，后端也会托管 Vue 页面。

### Vue 开发模式

另开终端：

```powershell
cd frontend
npm install
npm run dev
```

打开 `http://localhost:5173`。Vite 将 `/api` 请求代理到 `http://localhost:8080`。

### Web 构建

```powershell
cd frontend
npm install
npm run build
```

构建文件输出到 `frontend/dist/`。之后启动 Go 服务，可访问 `http://localhost:8080`。

## Electron 桌面版

Electron 主进程位于 `electron/main.cjs`，隔离的 preload 在渲染进程暴露桌面 API 地址。启动时先检查 `localhost:8080/api/health`；如果没有已有后端，则尝试运行 `bin/server.exe` 或根目录的 `server.exe`，然后加载 `frontend/dist/index.html`。若页面尚未构建，会尝试开发服务器地址 `http://127.0.0.1:5173`。

先在仓库根目录构建前端与 Go 后端：

```powershell
cd frontend
npm install
npm run build
cd ..
go build -o bin/server.exe ./cmd/server
```

再启动 Electron：

```powershell
cd electron
npm install
npm start
```

打包脚本为 `npm run dist`，Windows 目标为 NSIS 安装包。electron-builder 配置列出了前端构建、配置、数据和后端目录；安装后仍需验证这些资源的实际位置、SQLite 可写路径及现场 OPC DA 连接。Electron 中若检测到本机已有可响应的 8080 服务，会使用该服务而不再启动自身后端。

## 常见问题

- **后端启动即退出**：检查五个配置文件是否存在、CSV 列名和字段是否正确，以及点位设备名是否匹配。
- **Modbus 无数据**：仓库默认关闭 Modbus；检查 `configs/modbus.json` 的启用开关、IP/端口和网络连通性。
- **OPC DA 无数据**：检查 Windows 环境、Kepware ProgID、节点、Server 状态以及 COM/DCOM 权限。
- **前端访问不了 API**：确认后端监听 `8080`；开发模式确认 Vite 在 `5173`；Electron 查看主进程和后端日志。
- **历史数据不增长**：检查 `history.json` 的 `enabled`、点位 `history` 开关和 `data/` 目录写权限。
- **Electron 页面未加载**：先构建 `frontend/dist`，并检查打包后资源路径及后端可执行文件是否存在。

