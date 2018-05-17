package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
)

// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {
	mu sync.RWMutex

	config *Config
	zones map[string]Zone
	pool *SharedBackendConnPool

	online bool
	closed bool
}

func (s *Router) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.online = true
}

func (router *Router) FillZone() {//完成zone的初始化与填充

}

func (router *Router) Close() {//关闭

}

func (router *Router) isOnline() bool {
	return router.online && !router.closed
}

func (router *Router) KeepAlive() {//保持连接池在线

}

func (router *Router) dispatch(r *Request) error{//依照req转发到相应zone
	log.Printf("%#v",r)
	for _,v := range r.Multi {
		log.Printf("%#v",string(v.Value))
	}



	// 多写是否有rollback的概念

	// 读需要根据key的格式去判断
	return nil
}

func NewRouter(config *Config)  *Router{
	s := &Router{config: config}
	s.pool = NewSharedBackendConnPool(config, config.BackendPrimaryParallel)
	return s
}