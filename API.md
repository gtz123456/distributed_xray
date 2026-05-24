# Distributed Xray API Documentation

本文档总结了系统中各个微服务暴露的所有 HTTP 接口及其调用规范。

## 全局鉴权规范

系统采用两种主要鉴权方式：
1. **客户端请求 (面向终端用户)**: 绝大多数 `/webservice` 接口需要在 HTTP Header 中携带 `Authorization: <JWT_TOKEN>`。
2. **服务间通信 (RPC/内部调用)**: 内部微服务调用需在 HTTP Header 中携带 `regkey: <内部密钥>` (小写)。
3. **管理员操作**: 面向管理员的特权接口需要在 HTTP Header 中携带 `regkey: <ADMIN_密钥>` (大小写需与环境变量匹配)。

---

## 1. Web Service (端口: 8004 / 8003)

Web Service 是整个平台对外的总网关，负责账号、鉴权、流量调度等。

### 用户管理
- `POST /signup`
  - **描述**: 注册新用户。
  - **Body**: `{"email": "...", "password": "..."}`
  - **鉴权**: 无

- `POST /login`
  - **描述**: 用户登录并获取令牌。
  - **Body**: `{"email": "...", "password": "..."}`
  - **鉴权**: 无
  - **返回**: `{"token": "<JWT>"}`

- `POST /logout`
  - **描述**: 注销登录（将 JWT 放入 Redis 黑名单）。
  - **鉴权**: 需要 `Authorization`
  - **返回**: `200 OK`

- `GET /user`
  - **描述**: 获取当前用户的订阅信息和流量剩余。
  - **鉴权**: 需要 `Authorization`
  - **返回**: `{"email": "...", "plan": "...", "traffic_used": 123, "traffic_limit": 456, ...}`

- `GET /verify`
  - **描述**: 验证注册邮箱。
  - **Query**: `?token=...`
  - **鉴权**: 无

### 节点连接与心跳
- `GET /realitykey`
  - **描述**: 获取当前 Xray 集群的 Reality 公钥。
  - **鉴权**: 需要 `Authorization`

- `POST /subscribe`
  - **描述**: 客户端发起连接请求，Web 会为其在某个 Node 上开辟一条专属端口。
  - **鉴权**: 需要 `Authorization`
  - **返回**: `{"port": "12345", "node_ip": "...", "uuid": "...", "pubkey": "..."}`

- `POST /heartbeat`
  - **描述**: 客户端保持连接存活的心跳，若断开心跳则后台定时器会释放其 Node 端口。
  - **鉴权**: 需要 `Authorization`

### 管理员操作 (AdminAuth)
- `POST /admin/users` (获取所有用户列表)
- `POST /admin/user` (手动创建用户)
- `DELETE /admin/user/:uuid` (删除用户)
- `PUT /admin/user/:uuid/traffic` (修改用户流量)
- `POST /admin/generatevoucher` (生成充值兑换码)

---

## 2. Node Service (端口: 8002)

Node Service 运行在边缘节点，直接与 Xray Core (gRPC) 通信并管控本地 `iptables` 与端口。**仅限内部调用**。

- `POST /connect`
  - **描述**: 开辟一个新的代理端口并建立隧道。
  - **鉴权**: 服务端点校验。
  - **Body**: `{"uuid": "...", "clientip": "...", "rate_limit": "...", "burst": "..."}`
  - **返回**: `{"port": "..."}`

- `POST /disconnect`
  - **描述**: 关闭代理端口，销毁对应连接协程。
  - **Body**: `["uuid-1", "uuid-2"]`

- `POST /limit`
  - **描述**: 动态修改用户的流量速率上限。

- `GET /info`
  - **描述**: 获取节点的 CPU 和内存占用状态。

---

## 3. Registry Service (端口: 8000)

Registry Service 提供服务发现和健康检查。**仅限内部调用**。

- `GET /`
  - **描述**: 查询某个在线微服务的所有实例 URL。
  - **Query**: `?serviceName=<ServiceType>`
  - **返回**: `[{"ServiceURL": "...", ...}]`

- `POST /`
  - **描述**: 注册一个新的服务节点。
  - **Header**: `regkey: <ServiceKey>`
  - **Body**: Registration 结构体。

- `DELETE /`
  - **描述**: 下线移除服务节点。

- `POST /heartbeat/basic`
  - **描述**: 微服务心跳端点，防止被垃圾回收。
  - **Body**: `ServiceID` (文本流)

---

## 4. Payment Service (端口: 8006)

负责处理与 TronGrid / TRX 加密货币的挂单和回调。

- `POST /api/payment/order/create`
  - **描述**: 创建一条新的付款账单记录。
  - **Body**: `{"amount": 10, "currency": "USD", "callback_url": "...", "method": "TRX"}`
  
- `GET /api/payment/order/status`
  - **描述**: 查询某条账单的状态。
  - **Query**: `?order_id=...`

- `GET /api/payment/qrcode`
  - **描述**: 获取支付二维码图片（含小数点偏移金额）。
  - **Query**: `?order_id=...`

---

## 5. Log & Shell Service

- `POST /log` (Log Service - 8001)
  - 接收来自各服务组件的文本日志。

- `POST /exec` (Shell Service - 8005)
  - 供管理员向服务器下发安全验证后的 Shell 命令。
