package proxy

import (
	"sync"
	"strings"
	"strconv"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/models"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
)

const ZoneSpr  = "::"
var (
	ErrZoneIsNotConfig = errors.New("the key requested zone is not exist in current config")
	ErrRespIsRequired = errors.New("resp is required")
)

// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {
	mu sync.RWMutex

	config *Config
	zones map[string]*Zone
	pool *SharedBackendConnPool

	currentZonePrefix string

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

func (router *Router) FillZone(pzones []*models.Zone) {//完成zone的初始化与填充
	for _,zone := range pzones {
		conn:=router.pool.Get(zone.Addr)
		if nil!=conn {
			rZone := NewZone(zone.Id,conn,zone.Prefix)
			router.zones[zone.Prefix]=rZone
		}else {
			log.Errorf("pool get %v addr conn error",zone.Addr)//TODO 
		}

	}
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

	switch r.OpStr {
		case "GET":
			router.mu.RLock()
			defer router.mu.RUnlock()
			// get zone from prefix
			z := router.zones[router.currentZonePrefix]
			log.Println(z)
			log.Println(router.currentZonePrefix)

			z.Forward(r) //数据库字段，此部分在这需要强制阻塞住执行获取结果
			r.Batch.Wait() // 与上步操作合并，并在req中增加值execed
			val:=string(r.Resp.Value)

			getKey := r.getKey()
			zoneInfo,exist,ok:=router.getZoneInfo(getKey)
			if !exist {//key 为全局读取写入的信息，不存在则直接返回
				return nil
			}
			//TODO timeout 异常
			log.Printf("get from loacl is null %v , key contains zone %v , key not in curr %v ",val=="",ok,router.currentZonePrefix!=zoneInfo)
			if val=="" && ok && router.currentZonePrefix!=zoneInfo{//本地查询结果为空&&key中包含区域信息&&区域不为当前区
				//如果含有区域信息，使用指定区域再执行一次
				realZone := router.zones[zoneInfo]
				realZone.Forward(r)
				//TODO wait此种做法失败可能
				r.Batch.Wait()
				//若有返回信息，并new一个request写入本地区域,ttl信息
				// TODO 可以加个强制从其他
				other_val := string(r.Resp.Value)
				log.Printf("switch to the other zone get %v",other_val)
				ttlr := TTLRequest(getKey)
				realZone.Forward(ttlr)
				ttlr.Batch.Wait()
				ttl,err:=strconv.Atoi(string(ttlr.Resp.Value))
				log.Printf("ttl is  %v %v",ttl,err)
				//ttl > 0 则继续
				if ttl > 0{
					setr := SetRequest(getKey,other_val,ttl)
					z.Forward(setr)
					setr.Batch.Wait()
				}else {
					setr := SetRequest(getKey,other_val,-1)
					z.Forward(setr)
					setr.Batch.Wait()
				}
				log.Printf("set key %v from zone %v to curr zone %v finish",getKey,zoneInfo,router.currentZonePrefix)
				return nil
			}else {//若无则直接返回
				//当前已经获取到值
				//区域信息为当前,可能已超时或丢失
				if !ok {//key中包含的区域信息未配置
					return ErrZoneIsNotConfig
				}
				if router.currentZonePrefix==zoneInfo {
					log.Warn("key is in curr zone,but not found this key may expired or missing")
				}
			}
			return nil
		case "SET":
			//根据key类型设置是同步执行还是异步执行

			//同步执行的时候的一致性，一个事务
			return nil

	}
	// 多写是否有rollback的概念

	// 读需要根据key的格式去判断
	return nil
}

func (router *Router) getZoneInfo(key string) (string,bool,bool) {//包含的区域信息,两种情况1.key不包含区域信息 2.包含的区域信息不存在
	//keys := make([]string, 0, len(router.zones))
	//for k := range router.zones {
	//	keys = append(keys, k)
	//}
	//log.Printf("zone keys %v",keys)
	//使用split方式
	keyArray := strings.Split(key,ZoneSpr)
	if len(keyArray) ==1 {
		return "",false,false
	}
	zoneInfo :=keyArray[0]
	log.Print(key,zoneInfo)
	_,ok := router.zones[zoneInfo]
	if ok {
		return zoneInfo,true,true
	}else {
		return "",true,false
	}

}

func NewRouter(config *Config)  *Router{
	var addrA = "172.16.80.2:6379"
	var addrB = "172.16.80.171:26379"
	log.Println("@@@@@@@@@@@@@@@@@@")
	s := &Router{config: config}
	s.pool = NewSharedBackendConnPool(config, config.BackendPrimaryParallel)
	s.zones = make(map[string]*Zone)

	//TODO 修改逻辑，暂时默认填充
	s.pool.Retain(addrA)
	s.pool.Retain(addrB)


	s.currentZonePrefix="A"
	log.Println("called")
	zonemA := models.Zone{1,addrA,"A"}
	zonemB := models.Zone{2,addrB,"B"}
	s.FillZone([]*models.Zone{&zonemA,&zonemB})
	return s
}