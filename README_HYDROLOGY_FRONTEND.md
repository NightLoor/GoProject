# 排涝站水雨情与渗压监测前端

本版前端根据 `configs/Tags.csv` 中的 OPC DA 点位组织三个站点：

- 坝仔排涝站（BZ）
- 淡塘排涝站（DT）
- 稻元排涝站（DY）

当前 OPC DA 点表共 30 个点位：每站 8 个渗压测点 + 1 个水位 + 1 个降雨量。Modbus 点位同时保留在统一 `Tags.csv` 中。

## 页面

- `/water-rain`：水雨情监测，站点切换、当前水位、当前降雨量、水位趋势、降雨量趋势、站点点位状态。
- `/seepage`：渗压分析，8 个测点卡片、单点历史趋势、最小/平均/最大/波动范围统计、ItemID 明细。
- `/alarms`：设备变量报警，展示报警上下限、当前值、活动报警及报警/恢复事件。
- `/history`：优化后的通用历史查询，可按站点、变量类型和测点查询并查看统计与采样明细。

三站总览页面已移除，应用启动后默认进入水雨情监测页面。

## 报警配置

报警阈值由 `configs/Alarm.csv` 管理。变量规则包含：

- `enabled`：单变量报警开关。
- `low`：低限；设置为 `null` 表示不启用低限报警。
- `high`：高限；设置为 `null` 表示不启用高限报警。
- `unit`：显示单位。
- `message`：报警说明。

前端可以直接修改并保存 `Alarm.csv`；保存成功后后端会立即更新 IOManager 的报警规则，无需重启服务。

## 数据来源

页面实时数据来自现有 Gin API：

- `GET /api/io`
- `GET /api/config`
- `GET /api/devices`
- `GET /api/io/:name/history`
- `GET /api/events`
- `GET /api/alarms`
- `GET /api/alarm-config`

后端已同步保留 `Tags.csv` 的 `description` 字段，前端直接使用中文点位说明。
