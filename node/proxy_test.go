package node

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// 测试限速器的创建和更新
func TestLimiter(t *testing.T) {
	key := "test_key"
	rateLimit := 5000
	burstLimit := 5000

	// 第一次创建限速器
	limiter := Limiter(key, rateLimit, burstLimit)

	if limiter.Limit() != rate.Limit(rateLimit) {
		t.Errorf("预期限速: %v, 实际限速: %v", rateLimit, limiter.Limit())
	}

	if limiter.Burst() != burstLimit {
		t.Errorf("预期突发: %v, 实际突发: %v", burstLimit, limiter.Burst())
	}

	// 更新限速器
	newRateLimit := 10000
	newBurstLimit := 10000
	limiter = Limiter(key, newRateLimit, newBurstLimit)

	if limiter.Limit() != rate.Limit(newRateLimit) {
		t.Errorf("更新后预期限速: %v, 实际限速: %v", newRateLimit, limiter.Limit())
	}

	if limiter.Burst() != newBurstLimit {
		t.Errorf("更新后预期突发: %v, 实际突发: %v", newBurstLimit, limiter.Burst())
	}
}

// 测试连接处理函数 handleConnection，验证是否限速成功
func TestDefaultHandleConnection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	listener, err := net.Listen("tcp", "localhost:80")
	if err != nil {
		t.Fatalf("无法在 localhost:80 启动监听: %v", err)
	}
	defer listener.Close()

	go handleConnection(clientConn, "localhost:80", rate.NewLimiter(rate.Limit(defaultlimit.Rate), defaultlimit.Burst), rate.NewLimiter(rate.Limit(defaultlimit.Rate), defaultlimit.Burst))

	// 模拟客户端发送数据
	dataToSend := make([]byte, 20000)
	dataToSend[10] = 1 // 避免数据为0
	start := time.Now()

	go func() {
		_, err := serverConn.Write(dataToSend)
		if err != nil {
			t.Errorf("模拟客户端发送数据时出错: %v", err)
		}
	}()

	// 从listener获取数据，计算平均速度
	receivedData := make([]byte, 0)
	buffer := make([]byte, 10000)

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("接受连接时出错: %v", err)
	}
	defer conn.Close()
	for {
		n, err := conn.Read(buffer)

		if err != nil {
			break
		}
		receivedData = append(receivedData, buffer[:n]...)

		if len(receivedData) >= 20000 {
			conn.Close()
			break
		}
	}

	elapsed := time.Since(start)
	expectedDuration := time.Duration(len(dataToSend)-defaultlimit.Burst) * time.Second / time.Duration(defaultlimit.Rate)
	if elapsed < expectedDuration {
		t.Errorf("限速未生效: %v", elapsed)
	}

	if !bytes.Equal(dataToSend, receivedData) {
		t.Errorf("接收到的数据与发送的数据不一致,源数据: %v, 接收到的数据: %v", dataToSend, receivedData)
	}

	t.Logf("接收到的数据长度: %d, 发送的数据长度: %d, 耗时: %v", len(receivedData), len(dataToSend), elapsed)
}

// 测试端口监听和连接接受
func TestStart(t *testing.T) {
	go func() {
		err := NewProxy(context.Background(), 8080)
		if err != nil {
			t.Errorf("监听时出错: %v", err)
		}
	}()

	// 模拟客户端连接
	time.Sleep(time.Second) // 等待服务启动
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("连接服务时出错: %v", err)
	}
	defer conn.Close()

	// 模拟发送数据
	message := "test message"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("发送数据时出错: %v", err)
	}
}
