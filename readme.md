# distributed vpn project


### build docker image
docker build -t logservice --target=logservice .
docker build -t nodeservice --target=nodeservice .
docker build -t regservice --target=regservice .
docker build -t webservice --target=webservice .

### run docker image
docker run regservice
docker run --name logservice -e "Registry_IP=172.17.0.2" logservice
docker run nodeservice
docker run --rm -e "DB=root:password@tcp(127.0.0.1:3306)/vpn?charset=utf8mb4&parseTime=True&loc=Local" webservice

### 
