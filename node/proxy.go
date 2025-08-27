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

var defaultlimit = limit{Rate: 10 * 1000 * 1000 / 8, Burst: 16 * 1024} // for free plan

type ConnStats struct {
	Uploaded   int
	Downloaded int
}

type StatsStore struct {
	sync.Map
}

func limitReader(r io.Reader, lim *rate.Limiter, cnt *int) io.Reader {
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
				*cnt += n
			}
			if err != nil {
				return
			}
		}
	}()
	return pr
}

func handleConnection(conn net.Conn, dst string, upLim, downLim *rate.Limiter, statsStore *StatsStore) {
	defer conn.Close()
	targetConn, err := net.Dial("tcp", dst)
	if err != nil {
		return
	}
	defer targetConn.Close()

	_, portStr, _ := net.SplitHostPort(conn.LocalAddr().String())
	port, _ := strconv.Atoi(portStr)

	val, _ := statsStore.LoadOrStore(port, &ConnStats{})
	stats := val.(*ConnStats)

	go io.Copy(conn, limitReader(targetConn, downLim, &stats.Downloaded))

	io.Copy(targetConn, limitReader(conn, upLim, &stats.Uploaded))
}

func NewProxy(ctx context.Context, port int, sourceIP string, rateLimit int, burst int, statsStore *StatsStore) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	upLim := rate.NewLimiter(rate.Limit(rateLimit), burst)
	downLim := rate.NewLimiter(rate.Limit(rateLimit), burst)

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
		remoteAddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil || remoteAddr != sourceIP {
			conn.Close()
			continue
		}
		go handleConnection(conn, "localhost:443", upLim, downLim, statsStore)
	}
}
