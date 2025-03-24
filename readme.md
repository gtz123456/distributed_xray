# distributed vpn project


### build docker image
docker build -t logservice --target=logservice .
docker build -t nodeservice --target=nodeservice .
docker build -t regservice --target=regservice .
docker build -t webservice --target=webservice .

docker tag logservice:latest gtzfw/distributed_xray:logservice-latest
docker tag nodeservice:latest gtzfw/distributed_xray:nodeservice-latest
docker tag regservice:latest gtzfw/distributed_xray:regservice-latest
docker tag webservice:latest gtzfw/distributed_xray:webservice-latest

docker push gtzfw/distributed_xray:logservice-latest
docker push gtzfw/distributed_xray:nodeservice-latest
docker push gtzfw/distributed_xray:regservice-latest
docker push gtzfw/distributed_xray:webservice-latest

### run docker image
docker run -itd --name mysql -p 3306:3306 -v /home/qtdev/bi/mysql/conf:/etc/mysql/conf -v /nfs/mysql/data:/var/lib/mysql -v /home/qtdev/bi/mysql/logs:/logs -e MYSQL_ROOT_PASSWORD=password mysql

docker run -itd --name regservice regservice
docker run -itd --name logservice -e "Registry_IP=172.17.0.2" logservice
docker run -itd --name nodeservice -e "Registry_IP=172.17.0.2" -p 443:443 nodeservice 

docker run --rm -e "DB=root:password@tcp(146.235.210.34:3306)/vpn?charset=utf8mb4&parseTime=True&loc=Local" -e "Registry_IP=172.17.0.2" -e "REALITY_PUBKEY=pus2DL_XaiCBK05ddIynVtkYb75EjBm0vyCoZsUi2yw" -e "REALITY_PRIKEY=mNoGzlLbIVdKM0ZJY4sVZ8IOnFhwhdpcIYWBDQ_xQiw" -p 80:8080 webservice

### set file permission and disable firewall
chmod 777 

iptables -A INPUT -p tcp --dport 443 -j ACCEPT
iptables -A OUTPUT -p tcp --sport 443 -j ACCEPT


### deploy with k8s

kubectl apply -f k8s/regservice-deployment.yaml
kubectl apply -f k8s/logservice-deployment.yaml
kubectl apply -f k8s/nodeservice-deployment.yaml
kubectl apply -f k8s/webservice-secret.yaml
kubectl apply -f k8s/webservice-deployment.yaml
kubectl apply -f k8s/mysql-pv.yaml
kubectl apply -f k8s/mysql-deployment.yaml