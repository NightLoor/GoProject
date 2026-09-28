# Alarm.csv 变量报警配置

报警配置文件：`configs/Alarm.csv`。前端“设备变量报警”页面可直接编辑并保存该文件，后端保存后立即刷新 IOManager 的报警规则。

## CSV字段

`tag,data_type,enabled,alarm_value,low,high,unit,message`

- `tag`：必须与 IOManager 当前变量名一致。
- `data_type`：由 Tags.csv/IOManager 决定，CSV中的类型仅用于校验。
- `enabled`：是否启用该变量报警。
- 布尔变量：使用 `alarm_value=true/false`，表示变量等于该值时报警。
- 整数/浮点变量：使用 `low` / `high`，超出范围时报警；只设置一个边界也可以。
- `unit`：显示单位。
- `message`：报警说明。

## 前端修改

前端从 `/api/io` 获取 IOManager 当前变量，再与 `/api/alarm-config` 合并展示。保存时调用 `POST /api/alarm-config`，后端校验数据类型和上下限后原子写入 `configs/Alarm.csv`，并立即更新 IOManager。

## 报警事件

- 数值变量：`alarm_high` / `alarm_low` / `alarm_recovered`
- 布尔变量：`alarm_bool_true` / `alarm_bool_false` / `alarm_recovered`
- 通讯质量：`quality_bad` / `quality_recovered`

持续超限不会重复生成相同报警事件，状态恢复后只产生一次恢复事件。
