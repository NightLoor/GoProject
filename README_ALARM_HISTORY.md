# 历史报警记录

报警事件现在和点位历史数据共用 `data/history.db` SQLite 数据库。

## 数据库表

### alarm_events

保存变量报警触发和恢复事件：

- `timestamp`：事件时间
- `type`：`alarm_high` / `alarm_low` / `alarm_bool_true` / `alarm_bool_false` / `alarm_recovered`
- `tag`：变量名
- `message`：报警说明
- `value_json`：事件发生时的变量值
- `limit_value`：数值变量的触发限值
- `expected_value`：布尔变量的报警目标值
- `unit`：单位

通讯质量事件 `quality_bad` / `quality_recovered` 仍然只保存在运行内存事件列表，不进入历史报警表。

## API

### 查询历史报警

```http
GET /api/alarm-history?tag=BZ_Point1&start_date=2026-09-01&end_date=2026-09-07
```

参数：

- `tag`：可选。为空时返回日期范围内所有报警点位。
- `start_date`：必填，`YYYY-MM-DD`。
- `end_date`：必填，`YYYY-MM-DD`，结束日期当天数据会包含在结果中。

前端“设备变量报警”页面中的“历史报警查询”直接调用该接口。
