package proxy


// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {

	config *Config
	zones map[string]Zone
	pool *SharedBackendConnPool
}


func (router *Router) FillZone() {//完成zone的初始化与填充

}

func (router *Router) dispatch(r *Request) {//依照req转发到相应zone

}

func NewRouter(config *Config)  *Router{
	s := &Router{config: config}
	s.pool = NewSharedBackendConnPool(config, config.BackendPrimaryParallel)
	return s
}