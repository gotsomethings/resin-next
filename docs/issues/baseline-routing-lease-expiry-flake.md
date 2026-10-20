# Issue: routing test flake — recreated lease keeps the old expiry

**状态**：基线既有，非本次 platform-policy 改动引入  
**测试**：`internal/routing` → `TestRouteRequest_SameIPRotationMissRecreatesLease`

## 现象

```
--- FAIL: TestRouteRequest_SameIPRotationMissRecreatesLease
    router_matrix_test.go:306: recreated lease must have new expiry, still got old 1792498221650050000
```

## 复现

```bash
go test ./internal/routing/ -run TestRouteRequest_SameIPRotationMissRecreatesLease -count=5
```

本地 `-count=3` / `-count=5` 稳定失败（3/3、5/5）。

## 已确认的判断

在干净基线上复现（`git stash -u` 后同样失败），与本次改动无关。

## 疑似原因

测试断言 `newLease.ExpiryNs != oldExpiry`，但 `oldExpiry` 由 `time.Now().Add(time.Hour).UnixNano()` 生成，重建租约也用同样的表达式。

两者的时间差在纳秒精度下可能落在同一 tick：

```
oldExpiry := time.Now().Add(time.Hour).UnixNano()
... 路由重建 ...
newLease.ExpiryNs  // 与 oldExpiry 恰好相等
```

即断言本身过严：它要验证的是“租约被重建了”，而“到期时间变了”只是重建的**间接**证据，且依赖时钟精度。

## 建议修复方向（未实施）

1. 断言改为验证重建的**实质**：`newLease.NodeHash` 已更新 + `LeaseCreated == true` + IP 负载计数迁移；或
2. 若仍要比较到期时间，使用 `CreatedAtNs` 或注入可控时钟，不依赖 `time.Now()` 的分辨率。

## 影响面

仅测试；生产代码的租约重建逻辑本身正确（同 IP 轮换命中失败 → 选新节点 → 重写 lease）。
