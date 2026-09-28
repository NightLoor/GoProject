# Tags.csv 统一点表

现在 Modbus 和 OPC DA 的实际点位统一由 `configs/Tags.csv` 管理。

## 配置职责

- `configs/modbus.json`：只保存 Modbus TCP 设备连接参数与批量参数。
- `configs/opcda.json`：只保存 OPC DA Server/COM 连接参数。
- `configs/Tags.csv`：保存所有实际 IO 点位。

程序启动顺序：读取两个 JSON → 读取 `Tags.csv` → 按 `protocol + device` 把点位分配给对应 Driver → 启动 IOManager。

## CSV 字段

| 字段 | Modbus | OPC DA | 说明 |
|---|---|---|---|
| protocol | 必填 | 必填 | `Modbus` 或 `OPCDA` |
| device | 必填 | 必填 | 必须匹配对应 JSON 的设备 `name` |
| tag_name | 必填 | 必填 | 系统唯一点位名 |
| address | 必填 | 必填 | Modbus 地址或 OPC DA ItemID |
| data_type | 必填 | 必填 | `bool/int16/uint16/int32/uint32/float32/float64` |
| writable | 必填 | 必填 | `true/false` |
| slave_id | 必填 | 不用 | Modbus 从站 ID |
| register_type | 必填 | 不用 | coil/discrete_input/holding_register/input_register |
| scale | 可选 | 可选 | 默认 `1` |
| offset | 可选 | 可选 | 默认 `0` |
| byte_order | 可选 | 不用 | `ABCD/BADC/CDAB/DCBA` |
| description | 可选 | 可选 | 点位说明，当前主要用于维护和后续 UI 扩展 |

## 新增点位

以后新增一个点位只需要在 `configs/Tags.csv` 增加一行，不需要修改 `modbus.json` 或 `opcda.json` 的 tags 数组。

示例 OPC DA：

```csv
OPCDA,opcda_server_1,Motor_Current,Channel1.Device1.Motor.Current,float32,false,,,1,0,,电机电流
```

示例 Modbus：

```csv
Modbus,pump_station_1,Motor_Current,40020,float32,false,1,holding_register,0.1,0,ABCD,电机电流
```

## 启动要求

`Tags.csv` 中的 `device` 必须先存在于对应 JSON 文件，否则程序会在启动阶段直接报错，而不会悄悄忽略点位。


## 协议 Driver 开关

在各自 JSON 文件顶层设置 `enabled`：

- `configs/modbus.json`: `"enabled": true/false`
- `configs/opcda.json`: `"enabled": true/false`

关闭后不会启动对应 Driver；`Tags.csv` 中该协议的行会被忽略，其他协议继续运行。
