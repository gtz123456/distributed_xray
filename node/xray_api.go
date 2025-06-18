package node

import (
	"context"
	"fmt"

	loggerService "github.com/xtls/xray-core/app/log/command"
	"github.com/xtls/xray-core/app/proxyman/command"
	routingService "github.com/xtls/xray-core/app/router/command"
	statsService "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/vless"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserInfo struct {
	// For VMess & Trojan
	Uuid string
	// User's Level
	Level uint32
	// Which Inbound will add this user
	InTag string
	// User's Email, it's a unique identifier for users
	Email string
	// For ShadowSocks
	CipherType string
	// For ShadowSocks
	Password string
}

// Xray API listener address and port
type BaseConfig struct {
	APIAddress string
	APIPort    uint16
}

type XrayController struct {
	HsClient command.HandlerServiceClient
	SsClient statsService.StatsServiceClient
	LsClient loggerService.LoggerServiceClient
	RsClient routingService.RoutingServiceClient
	CmdConn  *grpc.ClientConn
}

func (xrayCtl *XrayController) Init(cfg *BaseConfig) (err error) {
	xrayCtl.CmdConn, err = grpc.NewClient(fmt.Sprintf("%s:%d", cfg.APIAddress, cfg.APIPort), grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return err
	}

	xrayCtl.HsClient = command.NewHandlerServiceClient(xrayCtl.CmdConn)
	xrayCtl.SsClient = statsService.NewStatsServiceClient(xrayCtl.CmdConn)
	xrayCtl.LsClient = loggerService.NewLoggerServiceClient(xrayCtl.CmdConn)
	//Not implement
	xrayCtl.RsClient = routingService.NewRoutingServiceClient(xrayCtl.CmdConn)

	return
}

func queryTraffic(c statsService.StatsServiceClient, ptn string, reset bool) (traffic int64, err error) {
	traffic = -1
	resp, err := c.QueryStats(context.Background(), &statsService.QueryStatsRequest{
		// example pattern: user>>>love@xray.com>>>traffic>>>uplink
		Pattern: ptn,
		// reset traffic data after query
		Reset_: reset,
	})
	if err != nil {
		return
	}
	// Get traffic data
	stat := resp.GetStat()
	fmt.Printf("Query traffic for pattern '%s': %v\n", ptn, stat)
	if len(stat) != 0 {
		traffic = stat[0].Value // unit: Bytes
	}

	return
}

func addVlessUser(client command.HandlerServiceClient, user *UserInfo) error {
	_, err := client.AlterInbound(context.Background(), &command.AlterInboundRequest{
		Tag: user.InTag,
		Operation: serial.ToTypedMessage(&command.AddUserOperation{
			User: &protocol.User{
				Level: user.Level,
				Email: user.Email,
				Account: serial.ToTypedMessage(&vless.Account{
					Id:         user.Uuid,
					Flow:       "xtls-rprx-vision",
					Encryption: "none",
				}),
			},
		}),
	})
	return err
}

func removeVlessUser(client command.HandlerServiceClient, user *UserInfo) error {
	_, err := client.AlterInbound(context.Background(), &command.AlterInboundRequest{
		Tag: user.InTag,
		Operation: serial.ToTypedMessage(&command.RemoveUserOperation{
			Email: user.Email,
		}),
	})
	return err
}
