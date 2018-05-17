package proxy

import "fmt"

// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {

	config *Config
	zones map[string]Zone
	pool *SharedBackendConnPool

	online bool
	closed bool
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
	fmt.Printf("%#v",r)
	return nil
}

func NewRouter(config *Config)  *Router{
	s := &Router{config: config}
	s.pool = NewSharedBackendConnPool(config, config.BackendPrimaryParallel)
	return s
}