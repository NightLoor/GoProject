# Modbus TCP Driver + Gin + Vue 3

一个面向工业设备监控的 Go 项目，负责 Modbus TCP / OPC DA 数据采集、历史持久化与写入，并通过 Gin 提供 HTTP API，Vue 3 提供 Web 监控界面。

## 分层结构

```text
Vue 3
  ↓ HTTP/JSON
Gin Router
  ↓
Handler
  ↓
Monitor Service
  ↓
Modbus / OPC DA Driver
  ↓
IO Manager
  ↓
SQLite History Store
  ↓
Modbus TCP / OPC DA Device
```

- `internal/api/`：Gin 路由与 HTTP Handler，只处理 HTTP/JSON。
- `internal/service/`：业务层，隔离 Handler 与协议驱动。
- `internal/modbus/`：Modbus TCP 连接、批量读取、数据转换、写入、设备状态。
- `internal/opcda/`：OPC DA COM/DCOM 连接、Group/Item 读取写入、设备状态。
- `internal/io/`：实时点位、历史开关、报警/事件与历史存储接口。
- `internal/history/`：SQLite 历史数据库连接、写入和查询。
- `internal/config/`：配置加载与校验。
- `internal/model/`：配置模型。
- `frontend/`：Vue 3 + Vite。

## API

| Method | Path | Description |
|---|---|---|
| GET | `/api/health` | 服务健康状态 |
| GET | `/api/config` | Modbus / OPC DA / 点位配置 |
| GET | `/api/io` | 全部实时点位 |
| GET | `/api/io/:name` | 单个点位 |
| GET | `/api/io/:name/history?limit=60` | 点位历史样本 |
| GET | `/api/devices` | 设备在线状态 |
| GET | `/api/events?limit=50` | 报警/恢复事件 |
| POST | `/api/io/write` | 写入可写点位 |

## 启动

### 开发模式

终端 1：

```bash
go run ./cmd/server
```

终端 2：

```bash
cd frontend
npm install
npm run dev
```

浏览器访问 `http://localhost:5173`。

### 生产模式

```bash
cd frontend
npm install
npm run build
cd ..
go run ./cmd/server
```

然后访问 `http://localhost:8080`。Gin 会直接提供 `frontend/dist`。

## 写入示例

```http
POST /api/io/write
Content-Type: application/json

{"tag":"Pump1_Run","value":true}
```

写入操作仍由 Modbus Driver 执行，Gin 不直接操作 Modbus。

## 测试

```bash
go test ./...
```

实时点位仍保存在内存中；启用 `history=true` 的变量会同时持久化到 `data/history.db` SQLite 数据库，服务重启后历史数据仍保留。`history=false` 的变量只保留当前实时值，不写入历史数据库。

## 当前配置方式

- `configs/modbus.json`：Modbus TCP 是否启用及设备连接参数。
- `configs/opcda.json`：OPC DA 是否启用及设备连接参数。
- `configs/Tags.csv`：Modbus 与 OPC DA 的统一点位表。
- `configs/Alarm.csv`：设备变量报警配置；布尔点设置 True/False 报警，数值点设置上下限，前端可直接修改并保存。

前端已移除三站总览页面，默认进入水雨情监测。设备变量报警由后端阈值判定，历史数据页面用于趋势与采样明细查询。


## 历史数据库

程序启动后会自动创建 `data/history.db` SQLite 数据库。`configs/Tags.csv` 的 `history` 列决定单个变量是否持久化：`true` 写入历史数据库，`false` 不写入。历史查询 API `/api/io/:name/history` 直接从数据库读取。当前 `Tags.csv` 中已有点位默认设置为 `true`，可按需改为 `false`。


## 数据库依赖

历史数据库使用 `modernc.org/sqlite`。该驱动是纯 Go 实现，不需要 CGO，并支持 Windows amd64。首次在新环境构建时如依赖尚未缓存，执行 `go mod tidy` 或 `go mod download`。
