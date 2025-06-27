package node

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"

	"golang.org/x/time/rate"
)

type limit struct { // unit: bytes per second
	Rate  int
	Burst int
}

var (
	limiters     = make(map[string]*rate.Limiter)
	mutex        sync.Mutex
	defaultlimit = limit{Rate: 10 * 1000 * 1000 / 8, Burst: 4 * 1024} // for free plan
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

func limitReader(r io.Reader, lim *rate.Limiter) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		buf := make([]byte, 4*1024) // 4KB buffer
		for {
			n, err := r.Read(buf)
			if n > 0 {
				// wait for tokens; blocking on global background context
				if err2 := lim.WaitN(context.Background(), n); err2 != nil {
					return
				}
				if _, err2 := pw.Write(buf[:n]); err2 != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return pr
}

func handleConnection(conn net.Conn, dst string, upLim, downLim *rate.Limiter) {
	defer conn.Close()
	targetConn, err := net.Dial("tcp", dst)
	if err != nil {
		return
	}
	defer targetConn.Close()

	// wrap and copy
	go io.Copy(conn, limitReader(targetConn, downLim))
	io.Copy(targetConn, limitReader(conn, upLim))
}

func NewProxy(ctx context.Context, port int) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	upLim := rate.NewLimiter(rate.Limit(defaultlimit.Rate), defaultlimit.Burst)
	downLim := rate.NewLimiter(rate.Limit(defaultlimit.Rate), defaultlimit.Burst)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // graceful shutdown
			default:
				return err
			}
		}
		go handleConnection(conn, "localhost:443", upLim, downLim)
	}
}
