package redis

import (
	"net"
	"strconv"
	"time"

	"golang.org/x/net/context"


	redigo "github.com/garyburd/redigo/redis"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
)

type Sentinel struct {
	context.Context
	Cancel context.CancelFunc

	Product, Auth string

	LogFunc func(format string, args ...interface{})
	ErrFunc func(err error, format string, args ...interface{})
}

func NewSentinel(product, auth string) *Sentinel {
	s := &Sentinel{Product: product, Auth: auth}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	return s
}

func (s *Sentinel) IsCanceled() bool {
	select {
	case <-s.Context.Done():
		return true
	default:
		return false
	}
}

func (s *Sentinel) printf(format string, args ...interface{}) {
	if s.LogFunc != nil {
		s.LogFunc(format, args...)
	}
}

func (s *Sentinel) errorf(err error, format string, args ...interface{}) {
	if s.ErrFunc != nil {
		s.ErrFunc(err, format, args...)
	}
}

func (s *Sentinel) do(sentinel string, timeout time.Duration,
	fn func(client *Client) error) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(c)
}

func (s *Sentinel) dispatch(ctx context.Context, sentinel string, timeout time.Duration,
	fn func(client *Client) error) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() {
		exit <- fn(c)
	}()

	select {
	case <-ctx.Done():
		return errors.Trace(ctx.Err())
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) mastersCommand(client *Client) ([]map[string]string, error) {
	defer func() {
		if !client.isRecyclable() {
			client.Close()
		}
	}()
	values, err := redigo.Values(client.Do("SENTINEL", "masters"))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var masters []map[string]string
	for i := range values {
		p, err := redigo.StringMap(values[i], nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		masters=append(masters,p)
	}
	return masters, nil
}

func (s *Sentinel) mastersDispatch(ctx context.Context, sentinel string, timeout time.Duration) (map[string]*SentinelMaster, error) {
	var masters = make(map[string]*SentinelMaster)
	var err = s.dispatch(ctx, sentinel, timeout, func(c *Client) error {
		p, err := s.mastersCommand(c)
		if err != nil {
			return err
		}
		for _, master := range p {
			epoch, err := strconv.ParseInt(master["config-epoch"], 10, 64)
			if err != nil {
				s.printf("sentinel-[%s] masters parse %s failed, config-epoch = '%s', %s",
					sentinel, master["name"], master["config-epoch"], err)
				continue
			}
			var ip, port = master["ip"], master["port"]
			if ip == "" || port == "" {
				s.printf("sentinel-[%s] masters parse %s failed, ip:port = '%s:%s'",
					sentinel, master["name"], ip, port)
				continue
			}
			masters[master["name"]] = &SentinelMaster{
				Addr: net.JoinHostPort(ip, port),
				Info: master, Epoch: epoch,
			}
		}
		return nil
	})
	if err != nil {
		switch errors.Cause(err) {
		case context.Canceled:
			return nil, nil
		default:
			return nil, err
		}
	}
	return masters, nil
}

type SentinelMaster struct {
	Addr  string
	Info  map[string]string
	Epoch int64
}

func (s *Sentinel) Masters(sentinels []string, timeout time.Duration) (map[string]string, error) {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan map[string]*SentinelMaster, len(sentinels))

	var majority = 1 + len(sentinels)/2

	for i := range sentinels {
		go func(sentinel string) {
			masters, err := s.mastersDispatch(cntx, sentinel, timeout)
			if err != nil {
				s.errorf(err, "sentinel-[%s] masters failed", sentinel)
			}
			results <- masters
		}(sentinels[i])
	}

	masters := make(map[string]string)

	var voted int
	for alive := len(sentinels); ; alive-- {
		if alive == 0 {
			switch {
			case cntx.Err() != context.DeadlineExceeded && cntx.Err() != nil:
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			case voted != len(sentinels):
				s.printf("sentinel masters voted = (%d/%d) masters = %d (%v)", voted, len(sentinels), len(masters), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		}
		select {
		case <-cntx.Done():
			switch {
			case cntx.Err() != context.DeadlineExceeded:
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			default:
				s.printf("sentinel masters voted = (%d/%d) masters = %d (%v)", voted, len(sentinels), len(masters), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		case m := <-results:
			if m == nil {
				continue
			}
			for key, master := range m {
				masters[key]=master.Addr
			}
			voted += 1
		}
	}
}