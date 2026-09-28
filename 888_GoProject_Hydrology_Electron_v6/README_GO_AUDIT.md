# Go 后端修复说明

本版本以 TagsCSV 双协议后端稳定版为基线重新合并水雨情前端，避免上一版在 `internal/api` 目录混入多个 Go package 导致工程无法编译。

## 当前结构
- `cmd/server/main.go`: 分别读取 `configs/modbus.json`、`configs/opcda.json` 和 `configs/Tags.csv`。
- `internal/config`: JSON 只管理设备连接参数，Tags.csv 统一管理点位。
- `internal/driver`: 统一 `Port` 接口和 Composite。
- `internal/modbus`: Modbus TCP Driver。
- `internal/opcda`: Windows OPC DA COM/DCOM Driver。
- `internal/io`: 当前值、Quality、内存历史和事件。
- `internal/service`: 业务层。
- `internal/api`: Gin API；仅 `api` package，Handler 位于 `internal/api/handler`。
- `frontend`: 三站水雨情、渗压分析页面。

## 点表
当前 `configs/Tags.csv` 为 38 个点：8 个 Modbus + 30 个 OPC DA。OPC DA 中文 description 会进入 API 配置。

## 启用开关
- `configs/modbus.json` 的 `enabled` 控制 Modbus Driver。
- `configs/opcda.json` 的 `enabled` 控制 OPC DA Driver。

## Windows 启动
在安装有 OPC DA Server 的 Windows 环境中运行，OPC DA Driver 会要求 COM 初始化，并按配置的 ProgID/Node 建立连接。
