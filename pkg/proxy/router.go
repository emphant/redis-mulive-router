package proxy


// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {

	method forwardMethod
}
