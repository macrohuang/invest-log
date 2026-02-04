# TODO

## 优化建议
- [x] 交易列表支持真实分页 + 总数：后端已有计数能力，前端目前固定拉取再过滤，建议改为分页接口并返回 total，减少大数据集卡顿与内存占用。
- [x] Holdings 相关计算复用/缓存：多个接口重复全表聚合与排序，建议引入缓存（写入/价格更新时失效）或汇总视图/快照表，降低重复扫描成本。
- [x] 批量价格更新并发 + 跳过近期更新：更新流程目前串行且不判断是否刚更新，建议加并发 worker pool 并基于 latest_prices.updated_at 做 TTL 跳过。
- [x] 数据一致性加强：增加外键约束并启用 PRAGMA foreign_keys=ON，避免 transactions/symbols/accounts 产生孤儿数据。
- [x] HTTP 服务稳健性：补齐 Server 超时配置并启用响应压缩，降低卡死风险与带宽消耗。
