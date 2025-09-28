package node

import (
	"testing"
)

func TestGetMediaConnectivity(t *testing.T) {
	res := getMediaConnectivity("en")
	if res == "" {
		t.Error("Expected non-empty media connectivity string")
	}
	t.Log("Media Connectivity Result:\n", res)
}

func TestGetConnectivity(t *testing.T) {
	res := GetConnectivity()
	if res == nil {
		t.Error("Expected non-nil connectivity map")
	}
	t.Log("Connectivity Result:\n", res)
}
