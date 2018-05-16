package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"github.com/emphant/redis-mulive-router/pkg/models"
)

type Proxy struct {
	mu sync.Mutex
	online bool
	closed bool
	config *Config
	exit struct {
		C chan struct{}
	}
	router Router
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
	return nil,nil
}