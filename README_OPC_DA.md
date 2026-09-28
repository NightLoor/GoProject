# OPC DA Driver 版本说明

本版本将 OPC DA 做成与 Modbus TCP 同级的独立驱动：

```text
IO Manager
├── Modbus Driver
│   └── Device
│       └── Register/Coil IO
└── OPC DA Driver
    └── Device
        └── OPC DA ItemID
```

HTTP Service 只依赖统一 `driver.Port`，写入时根据点名自动路由到 Modbus 或 OPC DA。

## OPC DA 与 OPC UA 的区别

OPC DA 是 Windows COM/DCOM 技术，不使用 `opc.tcp://` Endpoint。配置使用：

- `prog_id`: OPC DA Server 的 Windows ProgID
- `node`: OPC DA 服务所在 Windows 主机，通常本机为 `localhost`
- `item_id`: OPC DA ItemID，例如 `Channel1.Device1.Tag1`

本版本使用 `github.com/huskar-t/opcda`，支持 Windows amd64/386，并提供 OPC Server 连接、Item 读取和写入能力。

## 配置

编辑 `configs/opcda.json`：

```json
{
  "enabled": true,
  "poll_interval_ms": 1000,
  "devices": [
    {
      "name": "opcda_server_1",
      "prog_id": "Kepware.KEPServerEX.V6",
      "node": "localhost",
      "timeout_ms": 10000,
      "reconnect_interval_ms": 5000,
      "group_name": "GoMonitor",
      "tags": [
        {
          "name": "Tag1",
          "item_id": "Channel1.Device1.Tag1",
          "data_type": "bool",
          "writable": true,
          "scale": 1,
          "offset": 0
        }
      ]
    }
  ]
}
```

`Kepware.KEPServerEX.V6` 只是示例，必须替换成你实际 OPC DA Server 的 ProgID。可以在 Windows 的 OPC 客户端/Server 管理工具里确认。

## Windows 环境要求

OPC DA Client 必须运行在 Windows。若 OPC DA Server 与 Go 服务不在同一台 Windows 主机，需要正确配置 DCOM 权限以及 RPC/动态端口；用户名、密码和权限属于 Windows/DCOM 配置，不属于 OPC UA 风格的应用层 Token。

项目采用 `//go:build windows` 的真实 OPC DA 实现；非 Windows 环境只保留空实现用于开发和测试，不会模拟 OPC DA 连接。

## 启动时应看到

成功时：

```text
opcda initialized: device=opcda_server_1 server=... group=GoMonitor items=...
opcda device connected: opcda_server_1 (...)
```

失败时：

```text
opcda connect failed: device=opcda_server_1 prog_id=... node=... err=...
```

API `/api/devices` 会同时返回 Modbus TCP 与 OPC DA Device，并带有 `protocol` 字段。
