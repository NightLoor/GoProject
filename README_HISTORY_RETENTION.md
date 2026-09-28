# 历史数据存储、保留、归档与降采样

## 1. 变量历史采样

`configs/Tags.csv` 中 `history=true` 的变量进入历史数据库。当前默认每 **1 分钟**最多保存该变量 1 条记录；同一分钟内的高频采集数据不会逐条写入 SQLite。

`configs/history.json` 中：

- `enabled`: 全局历史数据库开关
- `sample_interval_minutes`: 历史采样周期，默认 1 分钟
- `retention_days`: 主历史数据库保留天数，默认 730 天（2 年）
- `archive_enabled`: 是否把超过保留周期的数据迁移到归档数据库
- `archive_dir`: 归档数据库目录，默认 `data/archive`
- `archive_retention_days`: 归档数据库再次清理前的保留天数，默认 3650 天（10 年）
- `cleanup_interval_hours`: 自动清理周期，默认 24 小时
- `query_max_points`: 单变量历史查询最多返回的点数，默认 2000

## 2. 数据库

主数据库：`data/history.db`

归档数据库：`data/archive/history_archive.db`

归档前先复制到归档库并使用原始 `id` 做幂等保护，复制成功后才删除主库对应记录。

## 3. 自动维护

程序启动时和每个 `cleanup_interval_hours` 小时执行一次维护：

1. 将超过 `retention_days` 的主库历史记录迁移到归档库（若启用归档）。
2. 删除主库超期记录。
3. 删除归档库中超过 `archive_retention_days` 的记录。

因此正常运行时 `history.db` 不会无限增长。

## 4. 查询降采样

历史日期查询仍限制最多 10 天。数据库查询先统计目标点在时间范围内的记录数量：

- 少于 `query_max_points`：返回全部。
- 超过 `query_max_points`：在数据库层按时间序号抽样，并保留首尾点。
- 主库和归档库同时命中时，结果合并后再次做一次最终降采样。

这可以避免历史查询把几十万/百万条原始记录一次性返回到 Vue。

## 5. 2 年数据规模

如果 38 个点都启用 `history=true`，按 1 分钟保存：

`38 × 60 × 24 × 730 ≈ 39,945,600` 条记录。

相比每秒保存，数据量会下降约 60 倍。实际数据库大小取决于数值、质量、错误字段等内容。
