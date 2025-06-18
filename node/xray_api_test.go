package node

import (
	"testing"
)

func TestQueryTraffic(t *testing.T) {
	var (
		xrayCtl *XrayController
		cfg     = &BaseConfig{
			APIAddress: "127.0.0.1",
			APIPort:    8080,
		}
	)
	xrayCtl = new(XrayController)
	err := xrayCtl.Init(cfg)
	defer xrayCtl.CmdConn.Close()
	if err != nil {
		t.Errorf("Failed %s", err)
	}
	ptn := "" //"user>>>gtzafw@gmail.com>>>traffic>>>downlink"
	trafficData, err := queryTraffic(xrayCtl.SsClient, ptn, false)
	if err != nil {
		t.Errorf("Failed %s", err)
	}
	t.Logf("Traffic data for pattern '%s': %d bytes", ptn, trafficData)

}

func TestAddUser(t *testing.T) {
	var (
		xrayCtl *XrayController
		cfg     = &BaseConfig{
			APIAddress: "127.0.0.1",
			APIPort:    8080,
		}
	)
	xrayCtl = new(XrayController)
	err := xrayCtl.Init(cfg)
	defer xrayCtl.CmdConn.Close()
	if err != nil {
		t.Errorf("Failed %s", err)
	}

	user := &UserInfo{
		Uuid:  "123e4567-e89b-12d3-a456-426614174000",
		Level: 0,
		InTag: "test",
		Email: "TestAddUser",
	}

	err = addVlessUser(xrayCtl.HsClient, user)
	if err != nil {
		t.Errorf("Failed to add user: %s", err)
	} else {
		t.Logf("User %s added successfully", user.Email)
	}
}
