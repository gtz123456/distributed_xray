package node

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type limit struct {
	Rate  int
	Burst int
}

var (
	limiters     = make(map[string]*rate.Limiter)
	mutex        sync.Mutex
	defaultlimit = limit{Rate: 2000, Burst: 2000} // for free plan
)

func Limiter(key string, limit int, burst int) *rate.Limiter {
	mutex.Lock()
	defer mutex.Unlock()

	limiter, ok := limiters[key]
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(limit), burst)
		limiters[key] = limiter
	} else {
		limiter.SetLimit(rate.Limit(limit))
		limiter.SetBurst(burst)
	}

	return limiter
}

func handleConnection(conn net.Conn, dst string) {
	defer conn.Close()

	targetConn, err := net.Dial("tcp", dst)
	if err != nil {
		return
	}

	defer targetConn.Close()

	limiter, ok := limiters[conn.RemoteAddr().String()]
	if !ok {
		limiter = Limiter(conn.LocalAddr().Network(), defaultlimit.Rate, defaultlimit.Burst)
	}

	go func() {
		io.Copy(conn, targetConn)
	}()

	buffer := make([]byte, 2000)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		err = limiter.WaitN(ctx, n)

		targetConn.Write(buffer[:n])

		cancel()

		if err != nil {
			return
		}

	}

}

func Rate() limit {
	// mock function
	return limit{Rate: 4000, Burst: 4000}
}

func Start(port int, dest string) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go handleConnection(conn, dest)
	}
}
