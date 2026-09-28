# Modbus Vue Frontend

前端已拆分为三个监控页面：

- `/overview` 数据总览：实时点位、设备状态、点位写入、最近趋势
- `/alarms` 设备变量报警：读取 `/api/events` 展示异常/恢复事件
- `/history` 历史数据查询：按变量查询 `/api/io/:name/history`

## 安装

```bash
npm install
```

## 开发

```bash
npm run dev
```

默认通过 `vite.config.js` 将 `/api` 代理到 `http://localhost:8080`。

## 构建

```bash
npm run build
```

说明：当前历史数据接口来自 Go 后端 `io.Manager` 的内存历史，因此适合最近数据查询；如果需要按日期、月份、年度查询，需要后端增加 SQLite/PostgreSQL 等持久化历史库和新的查询 API。
