# Distributed Xray VPN Project

A distributed, microservice-based VPN platform utilizing **Xray-Core** (VLESS protocol with Reality security). The project incorporates user signup and login, automated bandwidth traffic calculation, subscription plan enforcement, rate limiting, and Tron (TRX) cryptocurrency payment integration.

---

## System Architecture & Microservices

The application is structured as a collection of distributed microservices that register with a central service registry:

```
  ┌──────────────────────────────────────────────────────────┐
  │                   Registry Service                       │
  │                  (Service Discovery)                     │
  └───────────────────────────▲──────────────────────────────┘
                              │ Registers & Heartbeats
  ┌───────────────────────────┴──────────────────────────────┐
  │                      Web Service                         │
  │     (User Mgmt, Plans, Vouchers, DB & Router Gateway)    │
  └─────┬──────────────────────────────────────────────┬─────┘
        │                                              │
        │ gRPC Connection                              │ REST Callback
        ▼                                              ▼
  ┌──────────────────────────┐                  ┌────────────┴─────────────┐
  │     Node Service         │                  │     Payment Service      │
  │ (Xray controller, Proxy  │                  │  (TRX Payment Tracker &  │
  │  Tunnels & Bandwidth Rpt)│                  │   TronGrid Shasta API)   │
  └──────────┬───────────────┘                  └──────────────────────────┘
             │
             ▼ Local gRPC
  ┌──────────────────────────┐
  │     Xray Core Daemon     │
  └──────────────────────────┘
```

### 1. Registry Service (`regservice`)
* **Role**: Centralized service discovery and coordinator.
* **Port**: `8000` (by default)
* **Functionality**:
  * Exposes registration endpoints (`/services` and `/heartbeat/`).
  * Microservices register themselves on startup and send periodic heartbeats (every 3 seconds).
  * Automatically removes services if their heartbeats have timed out (> 20 seconds).
  * Sends patches/updates to dependent services whenever dependent registrations are added or removed.

### 2. Log Service (`logservice`)
* **Role**: Centralized log aggregator.
* **Port**: `8001` (by default)
* **Functionality**:
  * Exposes a `/log` endpoint to receive log messages via HTTP POST from other services.
  * Appends received entries to a local log file named `distributed.log`.

### 3. Node Service (`nodeservice`)
* **Role**: Controls the VPN routing engine and client connections on node servers.
* **Port**: `8002` (by default)
* **Functionality**:
  * **Xray API Controller**: Communicates with the local Xray Core daemon via gRPC (`127.0.0.1:8080`) to dynamically add and remove VLESS user accounts (`addVlessUser`/`removeVlessUser`).
  * **Dynamic Proxy Tunnels**: When a user connects, it starts a dedicated proxy listener on a random TCP port (10000–60000) that tunnels incoming TCP streams to Xray (`localhost:443`).
  * **Redis Sliding Window Rate Limiter**: Enforces upload/download speed limits on a per-user basis based on their subscription plan using a distributed Redis sliding window.
  * **IP Pinning**: Restricts traffic to the tunnel port to only allow the client's public IP.
  * **State Restoration**: NodeService persists its proxy configurations in Redis and automatically recovers running proxies upon restart without disconnecting users.
  * **Bandwidth Monitoring**: Collects traffic statistics passing through user tunnels and reports data consumption increments to the Web Service (`/traffic`) every 5 seconds.
  * **Traffic Limit Enforcement**: Periodically checks bandwidth consumption and automatically blocks all ports (except SSH port 22) using `iptables` if the traffic limit is reached.

### 4. Web Service (`webservice`)
* **Role**: Primary entry point, database gateway, and user management center.
* **Port**: `8003` (main router), `8004` (Gin server routing)
* **Functionality**:
  * **User Authentication**: Binds signup `/signup` (automatically sets `IsVerified: true`) and login `/login` routes. Provides signed JWT tokens.
  * **Server Registry**: Queries the registry server for active `NodeService` providers and serves a JSON list of available VPN nodes to clients.
  * **Connection Allocator**: Receives client `/connect` requests. Verifies that the user has a valid plan and sufficient traffic quota. Commands the Node Service to open a rate-limited tunnel and returns the connection credentials (port, UUID, reality pubkey) to the client.
  * **Traffic Aggregator**: Receives traffic report updates from nodes (`/traffic`) and updates user consumption data in the MySQL database.
  * **Billing & Voucher System**: Supports redeeming `/redeem` and generating `/admin/generatevoucher` voucher codes for balance top-ups or subscription plan extensions.

### 5. Payment Service (`paymentservice`)
* **Role**: Integrates with the Tron blockchain network to process payments.
* **Port**: `8006` (by default)
* **Functionality**:
  * Exposes `/api/payment/order/create` and `/api/payment/order/status` endpoints.
  * **TRX Sun-Offset Mapping**: Resolves payment verification overlap issues on a single wallet address by mapping each pending order to a unique, slightly offset TRX amount (measured in Sun: `1 Sun = 0.000001 TRX`).
  * **TronGrid Polling**: Checks transaction histories on the Tron Shasta Testnet via the TronGrid API. When a matching amount is detected, the status changes to `paid` and it triggers the callback webhook on the Web Service to credit user balance.

---

## Core Connection Workflow

```
Client               Web Service             Node Service          Xray Core
  │                       │                       │                    │
  ├─────── /login ───────>│                       │                    │
  │<─── Returns JWT ──────┤                       │                    │
  │                       │                       │                    │
  ├────── /connect ──────>│                       │                    │
  │                       ├────── /connect ──────>│                    │
  │                       │                       ├─ gRPC (Add User) ─>│
  │                       │                       │                    │
  │                       │                       ├─ Spawns Tunnel ────┤
  │                       │<── Returns Node Port ─┤                    │
  │<── Returns Config ────┤                       │                    │
  │                       │                       │                    │
  ├────────────── Connects to Node Port ─────────>│                    │
  │                       │                       ├─ Tunnels traffic ─>│
```

---

## Configuration

Prerequisites are controlled via environment variables. Configure them by copying `.env.template` to `.env` in the root directory:

```bash
cp .env.template .env
```

### Essential Parameters
* `Registry_Port` / `Registry_IP`: Controls where the Registry Service listens and how clients locate it.
* `DB`: The database connection base DSN (e.g. `root:password@tcp(127.0.0.1:3306)`). The system automatically creates a `vpn` database if it does not exist.
* `REDIS_ADDR` / `REDIS_PASSWORD`: Connection parameters for the Redis cluster. Used for rate limiting, connection caching, and order ID resolution.
* `SECRET`: A secret string used to sign JWT web authorization tokens.
* `REGKEY` / `regkey`: Inter-service security keys for internal RPC authorization.
* `REALITY_PRIKEY` / `REALITY_PUBKEY`: Keypair for VLESS Reality protocol.

---

## How to Build & Run on a Server

### Prerequisites
1. **Go Environment**: Version `>= 1.23.1`.
2. **Database**: MySQL server running and accessible.
3. **IP route**: If nodes run on the same server, packages sent to the server's public IP may not be routed to localhost. Run `sudo ip route add local <YOUR PUBLIC IP> dev <YOUR  eg eth>`
4. **Firewall**: Ensure the port ranges `8000-8006` and client proxy ports `10000-60000` are open in your server firewall rules.

### Option 1: Run as Binaries directly on Host

1. **Build the Microservices**:
   ```bash
   go build -o regservice ./cmd/regservice
   go build -o logservice ./cmd/logservice
   go build -o paymentservice ./cmd/paymentservice
   go build -o webservice ./cmd/webservice
   go build -o nodeservice ./cmd/nodeservice
   ```

2. **Start the Services in Sequence using tmux**:
   Using `tmux` allows you to manage all services in a single session and easily view their real-time logs.
   ```bash
   # Create a new detached tmux session named 'vpn'
   tmux new-session -d -s vpn

   # 1. Start Service Registry (Always start this first)
   tmux rename-window -t vpn:0 'regservice'
   tmux send-keys -t vpn:0 './regservice' C-m

   # 2. Start Centralized Logger
   tmux new-window -t vpn -n 'logservice'
   tmux send-keys -t vpn:logservice './logservice' C-m

   # 3. Start Payment Service
   tmux new-window -t vpn -n 'paymentservice'
   tmux send-keys -t vpn:paymentservice './paymentservice' C-m

   # 4. Start Web Service
   tmux new-window -t vpn -n 'webservice'
   tmux send-keys -t vpn:webservice './webservice' C-m

   # 5. Start Node Service (Must run as root for iptables / port allocation permissions)
   tmux new-window -t vpn -n 'nodeservice'
   tmux send-keys -t vpn:nodeservice 'sudo ./nodeservice' C-m

   # Attach to the session to monitor the services
   tmux attach -t vpn
   ```

3. **Stop the Services**:
   To stop all running services cleanly, simply kill the tmux session:
   ```bash
   tmux kill-session -t vpn
   ```

---

### Option 2: Run via Docker Containers

1. **Build Docker Images**:
   Build target-specific images from the multi-stage `Dockerfile`:
   ```bash
   docker build -t regservice --target=regservice .
   docker build -t logservice --target=logservice .
   docker build -t paymentservice --target=paymentservice .
   docker build -t webservice --target=webservice .
   docker build -t nodeservice --target=nodeservice .
   ```

2. **Run Containers**:
   To ensure all containers can resolve each other by name, first create a dedicated Docker network:
   ```bash
   docker network create vpn-net
   ```

   ```bash
   # 1. MySQL Database Container
   docker run -d \
     --name mysql-server \
     --network vpn-net \
     -p 3306:3306 \
     -v /my/own/datadir:/var/lib/mysql \
     -e MYSQL_ROOT_PASSWORD='gcB710.fR+z]' \
     mysql:latest

   # 2. Redis Cache Container
   docker run -d \
     --name redis-server \
     --network vpn-net \
     -p 6379:6379 \
     -v /my/own/redisdata:/data \
     redis:latest redis-server --appendonly yes

   # 3. Registry Discovery Container
   docker run -itd --name regservice --network vpn-net -p 8000:8000 regservice

   # 4. Log Aggregator Container
   docker run -itd --name logservice --network vpn-net -p 8001:8001 -e "Registry_IP=regservice" logservice

   # 5. Payment Service Container
   docker run -itd --name paymentservice --network vpn-net -p 8006:8006 \
     -e "DB=root:gcB710.fR+z]@tcp(mysql-server:3306)" \
     -e "REDIS_ADDR=redis-server:6379" \
     -e "Registry_IP=regservice" \
     paymentservice

   # 6. Web Gateway Container
   docker run -itd --name webservice --network vpn-net -p 8003:8003 -p 8004:8004 \
     -e "DB=root:gcB710.fR+z]@tcp(mysql-server:3306)" \
     -e "REDIS_ADDR=redis-server:6379" \
     -e "Registry_IP=regservice" \
     -e "REALITY_PUBKEY=pus2DL_XaiCBK05ddIynVtkYb75EjBm0vyCoZsUi2yw" \
     -e "REALITY_PRIKEY=mNoGzlLbIVdKM0ZJY4sVZ8IOnFhwhdpcIYWBDQ_xQiw" \
     webservice

   # 7. Node Service Container 
   # (Requires host network and network privileges for iptables & proxying. 
   #  Since it's on the host network, it uses 127.0.0.1 to access exposed ports of other containers)
   docker run -itd --name nodeservice --network=host --privileged \
     -e "Registry_IP=127.0.0.1" \
     -e "REDIS_ADDR=127.0.0.1:6379" \
     nodeservice
   ```

---

### Option 3: Deploy to Kubernetes Cluster

Apply the configuration files located in the `k8s/` directory:

```bash
# 1. Deploy Databases & Volumes
kubectl apply -f k8s/mysql-pv.yaml
kubectl apply -f k8s/mysql-deployment.yaml

# 2. Setup Secrets & Core Discovery
kubectl apply -f k8s/webservice-secret.yaml
kubectl apply -f k8s/regservice-deployment.yaml

# 3. Deploy Dependent Services
kubectl apply -f k8s/logservice-deployment.yaml
kubectl apply -f k8s/nodeservice-deployment.yaml
kubectl apply -f k8s/webservice-deployment.yaml
```

---

### Option 4: Testing the Project

The project includes comprehensive unit and integration tests covering the core architecture. It uses `sqlite` and `miniredis` to emulate database and caching environments locally, making the tests totally clean and detached from local services.

1. **Install Test Dependencies**:
   ```bash
   go get -t github.com/stretchr/testify github.com/alicebob/miniredis/v2 gorm.io/driver/sqlite
   ```

2. **Run Tests with Race Detector**:
   ```bash
   # Tests concurrent locks, Redis expirations, SQL parsing, and JWT state machines
   TEST_MODE=1 go test -v -race ./...
   ```
