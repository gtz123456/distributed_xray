# Security Audit Report — distributed_xray

Audit Date: 2026-05-24

---

## Summary

| Severity | Count |
|---|---|
| 🔴 Critical | 4 |
| 🟠 High | 4 |
| 🟡 Medium | 3 |
| 🔵 Low/Info | 3 |

---

## 🔴 Critical

---

### [C1] ShellService — 任意命令注入执行

**文件:** [`shell/server.go`](file:///Users/gao/Documents/vpn/distributed_xray/shell/server.go#L27)

**问题:**
```go
output, err := exec.Command(string(msg)).Output()
```
接收 HTTP Body 的原始字符串，直接作为 shell 命令执行。任何能访问该服务端口的人都可以在服务器上执行任意命令（`rm -rf /`, 下载木马, 反弹 Shell 等）。

**当前防护:** 无任何认证、无 IP 白名单。

**修复建议:**
- 短期：在 `firewalld` 或 `iptables` 上禁止该服务的端口对外开放，仅允许 localhost 访问。
- 长期：增加 `regkey` 认证，或完全移除该服务（仅用于调试）。

---

### [C2] `/traffic` 端点 — 无认证，任意修改用户流量数据

**文件:** [`cmd/webservice/main.go#L117`](file:///Users/gao/Documents/vpn/distributed_xray/cmd/webservice/main.go#L117), [`web/controllers/controllers.go#L457`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/controllers.go#L457)

**问题:**
```go
r.POST("/traffic", controllers.AddTraffic) // 无任何中间件
```
该端点接收 NodeService 的流量上报，但**完全没有认证**。任何人都可以向该接口发送伪造的 JSON 来任意增加任意用户的流量消耗，导致用户账号被封禁。攻击者也可以将数据刷为负数（`traffic` 字段为 `int`，无下界检查）来绕过流量限制。

**修复建议:** 增加 `AdminAuth` 或 `regkey` 中间件。

---

### [C3] Payment Callback — SSRF 风险（服务端请求伪造）

**文件:** [`web/controllers/payment.go#L67`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/payment.go#L67)

**问题:**
```go
callbackURL := fmt.Sprintf("http://%s:%s/payment/callback", publicIP, os.Getenv("GIN_PORT"))
```
在 `/payment` 接口中，`callbackURL` 虽然由服务器本身生成，但 PaymentService 的 `task.go` 在收到 callback 时会直接请求 `order.Callback` 中存储的 URL：

```go
// payment/order/task.go#L95
callbackUrl := fmt.Sprintf("%s?order_id=%s", order.Callback, order.ID)
req, err := http.NewRequest("POST", callbackUrl, nil)
```

若攻击者能控制 `CreateOrder` 时传入的 `callback` URL（通过修改请求或中间人攻击），PaymentService 就会向任意内网地址发起携带 `regkey` 的请求（SSRF + key 泄露）。

**修复建议:** 在 `CreateOrder` 中验证 `callback` URL 的域名/IP 是否在白名单内。

---

### [C4] NodeService — 连接发起端 IP 认证仅依赖 WebService 注册 IP，不安全

**文件:** [`node/server.go#L129`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go#L129)

**问题:**
```go
for _, prov := range providers {
    if prov.PublicIP == srcIP {
        allowed = true
        break
    }
}
```
NodeService 通过比对请求的 IP 是否与注册表中的 WebService IP 一致来鉴权。由于 NodeService 本身没有 TLS，且 IP 可以被伪造（同一局域网内的 ARP 欺骗或负载均衡场景），攻击者可以绕过此检查。更关键的是，这是双重鉴权（还有 `regkey` 头），但如果 `regkey` 被泄露，只需知道 WebService IP 即可伪造请求。

**修复建议:** 确保 `regkey` 足够随机和复杂；考虑使用 mTLS 进行服务间认证。

---

## 🟠 High

---

### [H1] DB 连接字符串打印到控制台

**文件:** [`web/db/connect.go#L17`](file:///Users/gao/Documents/vpn/distributed_xray/web/db/connect.go#L17), [`#L33`](file:///Users/gao/Documents/vpn/distributed_xray/web/db/connect.go#L33)

**问题:**
```go
fmt.Println("Initial DSN:", baseDSN)
fmt.Println("Connecting to:", dsnWithDB)
```
数据库用户名、密码、主机地址（包含在 `DB` 环境变量的 DSN 中）会在启动时被明文打印到标准输出。这些输出会被 LogService 收集，落入日志文件。

**修复建议:** 删除或替换为不包含密码的脱敏输出，如只打印数据库 host 和库名。

---

### [H2] JWT 令牌不支持主动吊销（Logout 功能缺失）

**文件:** [`web/controllers/controllers.go#L152`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/controllers.go#L152)

**问题:**
代码中虽然维护了 `expireMap`，但 `RequireAuth` 中间件验证 JWT 时完全没有查此 Map。同时没有 `/logout` 端点。一旦 JWT 被泄露，在其 30 天有效期内无法强制吊销，攻击者可一直使用该 token。

**修复建议:** 在 `RequireAuth` 中检查 `expireMap`，并实现 `/logout` 端点将 token 从 Map 中删除（或使用 Redis 黑名单）。

---

### [H3] `PlanMonitor` 中 TrafficLimit = -1 的无限制账号会被错误封禁

**文件:** [`web/controllers/PlanMonitor.go#L42`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/PlanMonitor.go#L42)

**问题:**
```go
db.DB.Model(&db.User{}).Where("plan_end < ? OR traffic_used >= traffic_limit", now).Find(&users)
```
当 `traffic_limit = -1`（无限流量）时，`traffic_used >= -1` **永远为真**（`traffic_used` 是 int，永远 ≥ 0 > -1），所有无限流量用户每 10 秒都会被触发一次断开连接。

**修复建议:**
```go
.Where("plan_end < ? OR (traffic_limit != -1 AND traffic_used >= traffic_limit)", now)
```

---

### [H4] 所有 HTTP 通信均为明文（无 TLS）

**问题:**
所有服务间通信（WebService → NodeService、RegService、LogService、PaymentService）均使用 `http://`，`regkey` 等认证 token 在网络中明文传输，在同一网络的攻击者可通过嗅探获取。

**修复建议:** 服务间使用 HTTPS 或 mTLS，或通过 VPN/内网通道隔离。

---

## 🟡 Medium

---

### [M1] `admin/setplan` 接口使用了错误的参数读取方式（功能性 Bug + 逻辑绕过）

**文件:** [`web/controllers/admin.go#L11`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/admin.go#L11)

**问题:**
```go
uuid := c.Param("uuid")   // 路由是 /admin/setplan，没有 :uuid 参数
plan := c.Param("plan")   // 同上，永远为空字符串
```
`uuid` 和 `plan` 永远是空字符串。函数之后检查 `user.UUID == ""`：
```go
if user.UUID == "" {
    c.JSON(404, gin.H{"error": "User not found"})
}
// 没有 return！继续执行
```
注意这里**没有 `return`**，代码会继续执行后续的 plan 分支判断，最终可能执行 `db.DB.Save(&user)` 保存一个空用户对象。

**修复建议:** 改用 `c.Query("uuid")` 和 `c.Query("plan")` 或请求体 JSON，并在 404 后加 `return`。

---

### [M2] `RequireAuth` 中间件 — JWT 解析错误被静默忽略

**文件:** [`web/middleware/middleware.go#L23`](file:///Users/gao/Documents/vpn/distributed_xray/web/middleware/middleware.go#L23)

**问题:**
```go
token, _ := jwt.Parse(tokenString, ...)  // 错误被忽略
```
JWT 解析错误被 `_` 丢弃，若解析失败则 `token` 为 `nil`，后面的 `token.Claims.(jwt.MapClaims)` 会导致 panic。虽然 Gin 的 recovery 中间件会捕获 panic，但会返回 500 而非 401，并不安全。

**修复建议:**
```go
token, err := jwt.Parse(tokenString, ...)
if err != nil {
    c.AbortWithStatus(http.StatusUnauthorized)
    return
}
```

---

### [M3] 用户余额可以通过并发请求被超额消费（Race Condition）

**文件:** [`web/controllers/controllers.go#L526`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/controllers.go#L526)

**问题:**
```go
if userinfo.Balance < amount { ... }  // 检查余额
// ...
userinfo.Balance -= amount
db.DB.Save(&userinfo)                 // 更新余额
```
两步操作之间没有数据库级别的锁，用户快速并发发送多个 `/subscribe` 请求时，可能让余额检查同时通过，导致余额被超额扣减（变为负数）。

**修复建议:** 使用数据库事务和 `SELECT ... FOR UPDATE` 行锁，或使用 `db.DB.Model(&userinfo).Where("balance >= ?", amount).Update("balance", gorm.Expr("balance - ?", amount))` 原子操作。

---

## 🔵 Low / Info

---

### [L1] DSN / Reality Key 等敏感信息暴露在 XRAY_PATH 等环境变量拼接的日志中

**文件:** [`utils/xray.go#L23`](file:///Users/gao/Documents/vpn/distributed_xray/utils/xray.go#L23)

日志中打印了 `XRAY_PATH` 路径，可能暴露服务器文件结构信息。建议日志降级或过滤路径中的敏感部分。

---

### [L2] 全局 Rate Limiter 只有 15次/分钟，且基于内存（重启失效）

**文件:** [`cmd/webservice/main.go#L102`](file:///Users/gao/Documents/vpn/distributed_xray/cmd/webservice/main.go#L102)

每次服务重启，限流计数器清零，攻击者可以通过触发服务重启来绕过限流。建议迁移到 Redis 持久化存储。

---

### [L3] IP 获取依赖第三方服务（`ipify.org`），存在可用性风险

**文件:** [`utils/net.go#L23`](file:///Users/gao/Documents/vpn/distributed_xray/utils/net.go#L23)

```go
resp, err := http.Get("https://api.ipify.org?format=text")
```
服务启动时依赖外部 API 获取公网 IP，如果 `ipify.org` 不可用，所有服务将无法启动。建议增加多个备用 IP 查询服务（fallback），或允许通过环境变量手动指定 `PUBLIC_IP`。

---

## 优先修复建议

| 优先级 | 漏洞 | 工作量 |
|---|---|---|
| P0 | [C1] ShellService 无认证 RCE | 小（加认证或防火墙限制） |
| P0 | [C2] `/traffic` 无认证 | 小（加 middleware） |
| P1 | [H3] `-1` TrafficLimit 触发误封禁 | 小（修改 SQL 条件） |
| P1 | [M1] `setplan` 无 return + 错误参数读取 | 小（加 return + 改参数读取） |
| P1 | [M2] JWT 解析错误 panic 风险 | 小（处理 err） |
| P2 | [H1] DB DSN 明文日志 | 小（删除 Println） |
| P2 | [H2] JWT 无法吊销 | 中（实现 logout + 黑名单） |
| P2 | [M3] 余额 Race Condition | 中（加事务锁） |
| P3 | [C3] SSRF callback | 中（URL 白名单验证） |
| P3 | [H4] 明文 HTTP 通信 | 大（引入 TLS） |
