FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY . .

FROM builder AS logservicebuilder
RUN CGO_ENABLED=0 go build -o /app/logservice ./cmd/logservice

FROM builder AS nodeservicebuilder
RUN CGO_ENABLED=0 go build -o /app/nodeservice ./cmd/nodeservice

FROM builder AS regservicebuilder
RUN CGO_ENABLED=0 go build -o /app/regservice ./cmd/regservice

FROM builder AS webservicebuilder
RUN CGO_ENABLED=0 go build -o /app/webservice ./cmd/webservice


FROM alpine:latest AS logservice
COPY --from=logservicebuilder /app/logservice /app/logservice
ENTRYPOINT ["/app/logservice"]

FROM alpine:latest AS nodeservice
COPY --from=nodeservicebuilder /app/nodeservice /app/nodeservice
RUN chmod 777 /app/bin/xray
RUN chmod 777 /app/bin/xray_arm
COPY ./node/bin /app/bin
ENTRYPOINT ["/app/nodeservice"]

FROM alpine:latest AS regservice
COPY --from=regservicebuilder /app/regservice /app/regservice
ENTRYPOINT ["/app/regservice"]

FROM alpine:latest AS webservice
COPY --from=webservicebuilder /app/webservice /app/webservice
ENV DB="root:134679852@tcp(host.docker.internal:3306)/vpn?charset=utf8mb4&parseTime=True&loc=Local"
ENTRYPOINT ["/app/webservice"]
