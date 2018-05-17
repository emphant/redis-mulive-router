package proxy

import (
	"sync"
	"time"
	"os"
	"os/exec"
	"strings"
	"net"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/utils"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"github.com/emphant/redis-mulive-router/pkg/models"
	"github.com/emphant/redis-mulive-router/pkg/utils/math2"
)

type Proxy struct {
	mu sync.Mutex
	online bool
	closed bool
	config *Config
	exit struct {
		C chan struct{}
	}
	router *Router
	model *models.Proxy

	lproxy net.Listener
	ladmin net.Listener
}

func (s *Proxy) serveProxy() {//启动代理tcp服务
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] proxy start service on %s", s, s.lproxy.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) (err error) {
		defer func() {
			eh <- err
		}()
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				return err
			}
			NewSession(c, s.config).Start(s.router)
		}
	}(s.lproxy)

	if d := s.config.BackendPingPeriod.Duration(); d != 0 {
		go s.keepAlive(d)
	}

	select {
	case <-s.exit.C:
		log.Warnf("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) serveAdmin() {//启动web管理http服务

}

func (s *Proxy) FillZones() {//填充代理支持的数据中心以及对应的数据节点关系


}


func (s *Proxy) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Proxy) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

var ErrClosedProxy = errors.New("use of closed proxy")

func (s *Proxy) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	if s.online {
		return nil
	}
	s.online = true
	return nil
}

func (s *Proxy) Close() error{
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.ladmin != nil {
		s.ladmin.Close()
	}
	if s.lproxy != nil {
		s.lproxy.Close()
	}
	if s.router != nil {
		s.router.Close()
	}

	return nil

}

func (s *Proxy)  setup(config *Config) error {
	proto := config.ProtoType
	if l, err := net.Listen(proto, config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostProxy)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.ProxyAddr = x
	}

	proto = "tcp"
	if l, err := net.Listen(proto, config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}
	return nil
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay = &DelayExp2{
		Min: 10, Max: 500,
		Unit: time.Millisecond,
	}
	for {
		c, err := l.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				delay.Sleep()
				continue
			}
		}
		return c, err
	}
}

func (s *Proxy) keepAlive(d time.Duration) {
	var ticker = time.NewTicker(math2.MaxDuration(d, time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			s.router.KeepAlive()
		}
	}
}

func New(config *Config) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	if err := models.ValidateProduct(config.ProductName); err != nil {
		return nil, errors.Trace(err)
	}
	s := &Proxy{}

	s.config = config
	s.exit.C = make(chan struct{})
	s.router = NewRouter(config)

	s.model = &models.Proxy{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.DataCenter = config.ProxyDataCenter
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}
	s.model.Hostname = utils.Hostname

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	go s.serveAdmin()
	go s.serveProxy()

	return s, nil
}