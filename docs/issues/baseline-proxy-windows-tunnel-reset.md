# Issue: proxy tunnel test fails on Windows (wsarecv reset)

**状态**：基线既有，非本次 platform-policy 改动引入  
**环境**：Windows，Go 1.26.3，`GOOS=windows`  
**测试**：`internal/proxy` → `TestPumpPreparedTunnelReader_ClientReadResetAfterIngressDoesNotFail`

## 现象

```
--- FAIL: TestPumpPreparedTunnelReader_ClientReadResetAfterIngressDoesNotFail
    tunnel_test.go:181: netOK: got false, want true
      (stage="connect_client_to_upstream_copy"
       err=writeto tcp 127.0.0.1:x->127.0.0.1:y:
       readfrom tcp ...: read tcp ...: wsarecv: An existing connection was forcibly closed by the remote host.)
```

## 复现

```bash
go test ./internal/proxy/ -run TestPumpPreparedTunnelReader_ClientReadResetAfterIngressDoesNotFail -count=1
```

稳定失败（100%）。

## 已确认的判断

在干净基线上复现（`git stash -u` 后同样失败），与 platform-policy 分支的任何改动无关。

## 疑似原因

- `isClientReadResetError` / `isBenignTunnelCopyError` 的错误串匹配是针对 Unix (`ECONNRESET` / `connection reset by peer`) 写的。
- Windows 上错误为 `wsarecv: An existing connection was forcibly closed by the remote host`，字符串不匹配，于是被当成真实的 upstream 失败。

## 建议修复方向（未实施）

1. 在 `isClientReadResetError` 中增加 Windows 判定：`errors.Is(err, syscall.ECONNRESET)` 优先于字符串匹配；或
2. 用 `net.Error` 接口 + `syscall.Errno` 判定，而非错误文本。

## 影响面

仅测试；运行时代码在 Linux 容器中不受影响。
