# 设计缺陷分析报告 — distributed_xray

---

## 一、支付系统设计缺陷

---

### [P1] 支付确认依赖金额唯一性（单钱包 + 金额微调方案的根本性缺陷）

**文件:** [`payment/order/order.go`](file:///Users/gao/Documents/vpn/distributed_xray/payment/order/order.go), [`payment/order/interval_set.go`](file:///Users/gao/Documents/vpn/distributed_xray/payment/order/interval_set.go)

**问题:**
整个支付系统使用一个固定的钱包地址 (`defaultWalletAddress`)，通过给每笔订单分配一个略微不同的金额（例如原价 +1 sun, +2 sun...）来区分不同用户的付款。

```go
const defaultWalletAddress = "TQehEHqevPkudydohYrjJxDwdBkAgFUebw"
// 匹配逻辑：amount → order ID
orderID, ok := ActualAmountToID[amount]
```

这个设计有以下问题：

1. **并发容量有限**：如果有大量并发待付款订单，金额微调的间隔会越来越大，对用户展示的实际支付金额会与原价相差甚远。
2. **内存状态易失**：`orderMap` 和 `ActualAmountToID` 全部存在内存中，服务重启会丢失（虽有 `RestoreStateFromDB` 但逻辑不完整——从 DB 恢复时重新计算了新的 `actualAmount`，不一定与数据库中记录的一致）。
3. **金额匹配被动轮询**：每 5 秒查询一次 TronGrid API 轮询交易，轮询时间窗口内如果有用户发送了多笔相同金额的交易则无法区分。
4. **不支持退款**：没有任何退款机制的设计。

**更好的方案:** 为每个订单生成一个独立的链上地址（HD 钱包派生），或使用 USDT/TRC20 加 memo 字段区分订单。

---

### [P2] 支付成功后立即充值，没有链上确认数验证

**文件:** [`payment/order/task.go#L87`](file:///Users/gao/Documents/vpn/distributed_xray/payment/order/task.go#L87)

**问题:**
```go
if order.Status == "pending" {
    order.Status = "paid"
    // ...立即更新 DB，立即 callback
}
```
TronGrid API 返回的交易可能是刚刚打包进区块、但还未获得足够确认数的交易。在区块链网络出现分叉时，这笔交易可能被回滚（双花攻击）。

**修复建议:** 在更新状态前检查交易的 `blockNumber`，要求至少等待 19-20 个区块确认（Tron 通常 19 个确认后才算不可逆）。

---

### [P3] 汇率缓存未加锁（并发 Race Condition）

**文件:** [`payment/order/currency_conversion.go#L18`](file:///Users/gao/Documents/vpn/distributed_xray/payment/order/currency_conversion.go#L18)

**问题:**
```go
var ratesCache map[string]float64  // 全局变量
var lastFetchTime int64 = 0         // 全局变量

func Convert(...) {
    if time.Now().Unix()-lastFetchTime > cacheDuration {
        _, err := FetchAllRates()  // 更新 ratesCache
    }
    rA := ratesCache[from]  // 并发读
}
```
`ratesCache` 和 `lastFetchTime` 没有任何并发保护，多个 goroutine 并发调用 `Convert` 时会产生数据竞争（race condition），可能导致 panic 或读到中间状态的汇率数据。

**修复建议:** 加 `sync.RWMutex` 或改用 `sync/atomic`。

---

## 二、状态管理缺陷

---

### [S1] 用户连接状态完全存在 WebService 内存中，无持久化

**文件:** [`web/controllers/controllers.go#L49`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/controllers.go#L49)

**问题:**
```go
var userConnectionMap = make(map[string][]UserConnection)
```
所有用户的当前连接信息（连接的节点 IP、端口、最后心跳时间）只存在 WebService 的内存 Map 中。

后果：
- **WebService 重启/崩溃** → 所有用户连接记录丢失，NodeService 上的 Xray 用户和 TCP 代理不会被清理，直到心跳超时（30秒）后才会由心跳 monitor 清理——但 monitor 也因重启而丢失了连接记录，所以实际上**NodeService 上的 Xray 用户泄漏了，永远不会被清理**。
- **WebService 无法水平扩展**：多实例部署时各实例的 `userConnectionMap` 相互独立，状态不一致。

**修复建议:** 使用 Redis 持久化连接状态，并在 WebService 启动时从 NodeService 同步当前连接。

---

### [S2] NodeService 重启后 Xray 中的用户不会被自动恢复

**文件:** [`node/server.go`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go)

**问题:**
NodeService 的 `connections` map 也是纯内存状态：
```go
var connections = make(map[string]int) // uuid: port
```
NodeService 重启后，内存中的连接记录清空，但 Xray 进程（如果没有一起重启）中的用户仍然存在，导致状态不一致——NodeService 认为该用户没有连接，但 Xray 仍然允许该用户的流量通过，且流量统计无法关联到任何代理端口。

---

### [S3] XrayController 每次连接请求都重新创建，不复用

**文件:** [`node/server.go#L175`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go#L175)

**问题:**
```go
func (sh *nodeHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
    xrayCtl = new(XrayController)         // 每次都 new
    err = xrayCtl.Init(cfg)               // 每次都建立 gRPC 连接
    // ...
    defer xrayCtl.CmdConn.Close()         // 每次都关闭
}
```
每个 `/connect` 请求都会创建一个新的 gRPC 连接到 Xray API，并在请求结束时关闭。这产生了大量不必要的连接建立/关闭开销，并且会覆盖全局的 `xrayCtl` 变量（非线程安全）。

**修复建议:** 在服务启动时初始化一个单例 `xrayCtl`，请求处理函数中复用。

---

## 三、架构设计缺陷

---

### [A1] WebService 与 NodeService 之间缺少流量统计核对机制

**问题:**
- WebService 依赖 NodeService 上报流量（`POST /traffic`）来更新用户的 `traffic_used`。
- NodeService 的代理层（[`proxy.go`](file:///Users/gao/Documents/vpn/distributed_xray/node/proxy.go)）统计的是 TCP 层的字节数，而 Xray 内部的流量统计（gRPC stats API）则是另一套数据。
- 两套统计相互独立，无法核对，存在流量少报/多报的风险。

**修复建议:** 统一使用一套流量统计来源（建议使用 Xray 原生的 Stats API），取消 Node 层的重复统计。

---

### [A2] 服务发现完全依赖内存，Registry 成为单点故障

**文件:** [`registry/server.go`](file:///Users/gao/Documents/vpn/distributed_xray/registry/server.go)

**问题:**
Registry 的所有注册信息（`registrationsMap`）存在内存中，没有任何持久化。Registry 进程一旦崩溃重启，所有服务必须重新注册，在重新注册期间整个系统无法工作。同时 Registry 是所有其他服务的强依赖，是一个单点故障。

**修复建议:** 考虑使用成熟的服务发现方案（如 Consul, etcd）替代自制 Registry，或至少为 Registry 添加状态持久化。

---

### [A3] 代理层 IP 白名单设计无法支持 NAT 场景

**文件:** [`node/proxy.go#L97`](file:///Users/gao/Documents/vpn/distributed_xray/node/proxy.go#L97)

**问题:**
```go
remoteAddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
if err != nil || remoteAddr != sourceIP {
    conn.Close()
    continue
}
```
代理端口只接受来自特定 `sourceIP` 的连接，这是针对客户端 IP 的白名单。但这要求：
1. 客户端必须有固定公网 IP（移动用户、4G 用户 IP 经常变化）。
2. 不支持客户端通过代理或 NAT 访问（IP 在经过 NAT 后会变成网关 IP）。

这个设计过于严格，现实中大多数 VPN 用户的 IP 是动态的。

---

### [A4] 单一节点代理到固定 `localhost:443`，无法灵活路由

**文件:** [`node/proxy.go#L102`](file:///Users/gao/Documents/vpn/distributed_xray/node/proxy.go#L102)

**问题:**
```go
go handleConnection(conn, "localhost:443", upLim, downLim, statsStore)
```
所有用户的代理连接都硬编码转发到 `localhost:443`（Xray 监听的端口）。如果将来需要支持多个 Xray 实例、不同的入站协议端口，或者端口可配置化，这里需要重构。

---

## 四、数据模型缺陷

---

### [D1] `TrafficUsed` 使用 `int` 类型，存在溢出风险

**文件:** [`web/db/models.go#L21`](file:///Users/gao/Documents/vpn/distributed_xray/web/db/models.go#L21)

**问题:**
```go
TrafficUsed  int // in Bytes
TrafficLimit int // in Bytes, -1 means unlimited
```
Go 的 `int` 在 64 位系统上是 64 位，理论上可以存储约 9.2 EB，够用。但 `TrafficLimit = -1` 的魔法值设计不优雅，并与 `PlanMonitor` 中的 `traffic_used >= traffic_limit` 查询逻辑冲突（见安全报告 H3）。

同样，`Balance` 和 `Amount` 都用 `int`（单位：分），在大金额场景下有精度问题（应用 `int64` 更安全）。

**修复建议:** 
- 使用 `int64` 统一所有金额和流量字段。
- 用专门的 `IsUnlimited bool` 字段替代 `-1` 魔法值。

---

### [D2] `Payment` 模型缺少 `UserUUID` 字段，只有 `UserID`

**文件:** [`web/db/models.go#L55`](file:///Users/gao/Documents/vpn/distributed_xray/web/db/models.go#L55)

**问题:**
```go
type Payment struct {
    UserID   uint   // 数据库自增 ID
    // 没有 UUID
}
```
`Payment` 关联的是 `UserID`（数据库自增 ID），而系统的其他地方大量使用 `UUID` 标识用户。这造成了不一致性：查询用户支付记录时需要先用 UUID 查 User 表拿到 ID，再去查 Payment。

---

### [D3] `Voucher` 没有使用次数限制字段

**文件:** [`web/db/models.go#L33`](file:///Users/gao/Documents/vpn/distributed_xray/web/db/models.go#L33)

**问题:**
Voucher 的设计只支持"一次性使用"（`IsUsed bool`），无法创建可以被多个用户使用的优惠码（如促销码）。同时没有 `MaxUses int` 和 `UsedCount int` 字段，功能局限。

---

## 五、可靠性缺陷

---

### [R1] `PlanMonitor` 中 DB 查询错误时直接 `return`，停止监控

**文件:** [`web/controllers/PlanMonitor.go#L23`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/PlanMonitor.go#L23)

**问题:**
```go
err := db.DB.Model(&db.User{}).Where(...).Find(&users).Error
if err != nil {
    return  // 整个 goroutine 退出！
}
```
如果数据库发生临时错误（网络抖动、连接超时），整个 `PlanMonitor` goroutine 会永久退出，之后不再检测任何用户的计划到期或流量超限。服务需要重启才能恢复此功能。

同样的问题存在于 `Save` 操作的错误处理中（第 35 行）。

**修复建议:** 将 `return` 改为 `continue`（继续下一个循环周期），并记录错误日志。

---

### [R2] `task.go` 中 JSON 解析失败时调用 `panic`

**文件:** [`payment/order/task.go#L57`](file:///Users/gao/Documents/vpn/distributed_xray/payment/order/task.go#L57)

**问题:**
```go
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
    resp.Body.Close()
    panic(err)  // 整个服务 panic
}
```
TronGrid API 如果返回了格式异常的响应（比如临时错误页面），会导致整个 PaymentService 崩溃。

**修复建议:** 将 `panic(err)` 替换为 `log.Println(err); return`。

---

### [R3] 获取公网 IP 失败时服务无法启动，缺少重试机制

**文件:** [`utils/net.go#L22`](file:///Users/gao/Documents/vpn/distributed_xray/utils/net.go#L22)

**问题:**
```go
func GetPublicIP() (string, error) {
    resp, err := http.Get("https://api.ipify.org?format=text")
```
所有服务启动时都会调用此函数。若 `ipify.org` 临时不可用，服务直接 `Fatal` 退出。没有重试机制，也没有环境变量覆盖手动指定 IP 的选项。

**修复建议:** 增加重试逻辑和多个备用 IP 查询服务，并支持通过 `PUBLIC_IP` 环境变量手动指定。

---

## 六、代码质量缺陷

---

### [Q1] 大量 `TODO` 未实现的关键功能

代码中存在大量未实现的关键功能，会在生产环境中导致问题：

| 位置 | 内容 | 影响 |
|---|---|---|
| [`utils/xray.go#L38`](file:///Users/gao/Documents/vpn/distributed_xray/utils/xray.go#L38) | `ConfigXray` 是空函数 | Reality 私钥无法动态配置 |
| [`node/server.go#L188`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go#L188) | `InTag: "test"` 硬编码 | 用户只能添加到 `"test"` inbound，无法灵活配置 |
| [`web/controllers/payment.go#L116`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/payment.go#L116) | 只支持 TRX，其他支付方式未实现 | |
| [`registry/heartbeat/server.go#L74`](file:///Users/gao/Documents/vpn/distributed_xray/registry/heartbeat/server.go#L74) | `ServerInfoHeartbeatHandler` 注释掉 | 无法监控节点负载 |
| [`web/controllers/controllers.go#L515`](file:///Users/gao/Documents/vpn/distributed_xray/web/controllers/controllers.go#L515) | 套餐价格 `300` 硬编码 | 无法动态配置价格 |

---

### [Q2] 全局 `xrayCtl` 变量存在并发写入问题

**文件:** [`node/server.go#L36`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go#L36), [`#L175`](file:///Users/gao/Documents/vpn/distributed_xray/node/server.go#L175)

**问题:**
```go
var xrayCtl *XrayController  // 包级全局变量

func (sh *nodeHandler) handleConnect(...) {
    xrayCtl = new(XrayController)  // 并发写全局变量，无锁保护
}
```
多个并发请求同时执行 `handleConnect` 时会相互覆盖 `xrayCtl`，可能导致 gRPC 连接在被另一个请求关闭后仍被使用。

---

### [Q3] 速率限制只覆盖部分端点，`/heartbeat` 没有限制

**文件:** [`cmd/webservice/main.go#L116`](file:///Users/gao/Documents/vpn/distributed_xray/cmd/webservice/main.go#L116)

**问题:**
```go
r.POST("/heartbeat", middleware.RequireAuth, controllers.HeartbeatFromClient)
// 没有 globalLimiter.Middleware()
```
`/heartbeat` 端点虽然需要 JWT 认证，但没有速率限制。恶意用户或者客户端 Bug 可能每秒发送大量心跳请求，导致 WebService 的 `userConnectionMapMutex` 锁竞争加剧，影响其他用户的连接速度。

---

## 总结优先级

| 优先级 | 问题 | 影响范围 |
|---|---|---|
| 🔴 P0 | [R1] PlanMonitor DB 错误直接退出 | 计费系统停止工作 |
| 🔴 P0 | [R2] Payment panic | 支付服务崩溃 |
| 🔴 P0 | [S1] 连接状态无持久化 | 重启后 Xray 用户泄漏 |
| 🟠 P1 | [S3] XrayController 并发覆盖 | 节点连接不稳定 |
| 🟠 P1 | [P3] 汇率缓存无锁 | 潜在 panic |
| 🟠 P1 | [P2] 支付无区块确认数验证 | 双花攻击风险 |
| 🟡 P2 | [A2] Registry 单点故障 | 系统可用性 |
| 🟡 P2 | [D1] int 魔法值 `-1` | 代码逻辑错误 |
| 🟡 P2 | [Q1] TODO 未实现功能 | 功能缺失 |
| 🔵 P3 | [P1] 单钱包支付方案 | 可扩展性受限 |
