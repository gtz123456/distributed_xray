#!/bin/bash
set -e

# Change to the script's directory
cd "$(dirname "$0")"

echo "=========================================="
echo " FreewayVPN Distributed Backend Installer "
echo "=========================================="

# 1. Ask for parameters
read -p "Enter public IP or Domain for this server (e.g. 170.9.29.245): " WEB_HOST
if [ -z "$WEB_HOST" ]; then
    echo "Public IP/Domain is required!"
    exit 1
fi

read -p "Do you want to run a local MySQL/Redis automatically? (y/n) [y]: " RUN_DB
RUN_DB=${RUN_DB:-y}

if [ "$RUN_DB" = "y" ]; then
    read -p "Enter a secure root password for MySQL [vpn123456]: " MYSQL_PASS
    MYSQL_PASS=${MYSQL_PASS:-vpn123456}
    DB_DSN="root:${MYSQL_PASS}@tcp(127.0.0.1:3306)/vpn?charset=utf8mb4&parseTime=True&loc=Local"
    REDIS_ADDR="127.0.0.1:6379"
else
    read -p "Enter MySQL DSN (e.g. root:password@tcp(127.0.0.1:3306)/vpn?...): " DB_DSN
    read -p "Enter Redis Address (e.g. 127.0.0.1:6379): " REDIS_ADDR
fi

echo ""
echo "Xray Reality Keys..."
echo "Using default keys that match the frontend configuration."
PRIKEY="KBLB6hgV4FcA3XNPuLePDdMPz8Wqh1AHazMIOQcGRh0"
PUBKEY="hK84iC1-jZZsWe3aK2E5iJKxkHZDzNoVqcsFd8NVLzQ"

# Auto-generate security keys
SECRET=$(openssl rand -hex 16 2>/dev/null || date +%s | md5sum | head -c 32)
REGKEY=$(openssl rand -hex 8 2>/dev/null || date +%s | md5sum | head -c 16)
REGKEY_LOWER=$(openssl rand -hex 8 2>/dev/null || date +%s | md5sum | head -c 16)

# 2. Generate .env file
echo "Generating .env file..."
cat > .env <<EOF
Registry_Port=8000
Log_Port=8001
Node_Port=8002
Web_Port=8003
GIN_PORT=8004
Shell_Port=8005
Payment_Port=8006

Registry_IP=127.0.0.1
Web_Host=${WEB_HOST}:8004

REDIS_ADDR=${REDIS_ADDR}
REDIS_PASSWORD=

DB=${DB_DSN}

SECRET=${SECRET}
REGKEY=${REGKEY}
regkey=${REGKEY_LOWER}

REALITY_PRIKEY=${PRIKEY}
REALITY_PUBKEY=${PUBKEY}

TRAFFIC_LIMIT_GB=10
CYCLE_RESET_DAY=1
EOF

# 4. Start DBs if requested
if [ "$RUN_DB" = "y" ]; then
    echo "Starting Redis..."
    docker rm -f vpn-redis 2>/dev/null || true
    docker run -d --name vpn-redis -p 6379:6379 --restart always redis:alpine

    echo "Starting MySQL..."
    docker rm -f vpn-mysql 2>/dev/null || true
    docker run -d --name vpn-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD="${MYSQL_PASS}" -e MYSQL_DATABASE=vpn --restart always mysql:8
    
    echo "Waiting 15 seconds for MySQL to initialize..."
    sleep 15
fi

# 5. Build Docker images
echo "Building backend Docker images..."
docker build --target logservice -t distributed_xray_logservice .
docker build --target nodeservice -t distributed_xray_nodeservice .
docker build --target regservice -t distributed_xray_regservice .
docker build --target webservice -t distributed_xray_webservice .

# 6. Stop existing app containers
echo "Stopping existing app containers..."
docker rm -f vpn-regservice vpn-logservice vpn-nodeservice vpn-webservice 2>/dev/null || true

# 7. Start App Containers (Using Host Network)
echo "Starting Registry Service..."
docker run -d --name vpn-regservice --network host --restart always --env-file .env distributed_xray_regservice

echo "Starting Log Service..."
docker run -d --name vpn-logservice --network host --restart always --env-file .env distributed_xray_logservice

echo "Starting Web Service..."
docker run -d --name vpn-webservice --network host --restart always --env-file .env distributed_xray_webservice

echo "Starting Node Service..."
docker run -d --name vpn-nodeservice --network host --restart always --env-file .env --privileged distributed_xray_nodeservice

echo "=========================================="
echo " Deployment Complete! "
echo " Web API is accessible at: http://${WEB_HOST}:8004"
echo " Node VPN is accessible at: ${WEB_HOST}:443"
echo " Make sure to open ports 8004, 443, and 10000-50000 in your server firewall!"
echo "=========================================="
