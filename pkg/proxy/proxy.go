package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
)

type Proxy struct {
	mu sync.Mutex
	online bool
	closed bool
}

func (s *Proxy) serveProxy() {//启动代理tcp服务

}

func (s *Proxy) serveAdmin() {//启动web管理http服务

}

func (s *Proxy) FillZones() {//填充代理支持的数据中心以及对应的数据节点关系

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