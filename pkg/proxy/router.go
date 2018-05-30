package proxy

import (
	"sync"
	"strconv"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/models"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"strings"
	"runtime"
)

const ZoneSpr  = "::"
var (
	ErrZoneIsNotConfig = errors.New("the key requested zone is not exist in current config")
	ErrRespIsRequired = errors.New("resp is required")
	ErrZoneIsNotMatch = errors.New("the key to write zone is not match curr")
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
	defer func() {//defer 防止出现panic,并打印栈
		if err := recover(); err != nil {
			var buf [8192]byte
			n := runtime.Stack(buf[:],false)
			log.Errorf("PANIC_ERROR %v %v",err,string(buf[:n]))
		}
	}()
	//TODO DEbug 信息按照req分组
	log.Printf("%#v",r)
	for _,v := range r.Multi {
		log.Printf("%#v",string(v.Value))
	}
	getKey := r.getKey()
	zoneInfo,exist,ok:=router.getZoneInfo(getKey)
	z := router.zones[router.currentZonePrefix]
	log.Println(z)
	log.Println(router.currentZonePrefix)
	switch r.OpStr {
		case "GET":
			router.mu.RLock()
			defer router.mu.RUnlock()
			// get zone from prefix
			z.Forward(r) //数据库字段，此部分在这需要强制阻塞住执行获取结果
			//TODO timeout 异常
			r.Batch.Wait() // 与上步操作合并，并在req中增加值execed
			val:=string(r.Resp.Value)
			log.Debugf("ENTER GET  key is %v value is %v",getKey,val)
			if !exist || zoneInfo==router.currentZonePrefix {//如果key中不存在区域信息/或者区域信息为当前，则当前即为解
				log.Debug("GET key NOT has zone info  OR  the zone info is local ")
				return nil
			}
			if val=="" && ok {//本地查询结果为空&&key中包含区域信息
				log.Debug("GET key has zone info AND find null value at local")
				//如果含有区域信息，使用指定区域再执行一次
				realZone := router.zones[zoneInfo]
				realZone.Forward(r)
				//TODO wait此种做法失败可能
				r.Batch.Wait()
				//若有返回信息，并new一个request写入本地区域,ttl信息
				// TODO 可以加个强制从其他
				other_val := string(r.Resp.Value)
				log.Debugf("SWITCHED to the '%v' zone get '%v'",zoneInfo,other_val)
				ttlr := TTLRequest(getKey)
				realZone.Forward(ttlr)
				ttlr.Batch.Wait()
				ttl,err:=strconv.Atoi(string(ttlr.Resp.Value))
				log.Debugf("GETTD ttl is  '%v' '%v' ",ttl,err)
				//ttl > 0 则继续
				if ttl > 0{
					log.Debug("START ttl setReq to local")
					setr := SetRequest(getKey,other_val,ttl)
					z.Forward(setr)
					setr.Batch.Wait()
				}else {
					log.Debug("START no ttl setReq to local")
					setr := SetRequest(getKey,other_val,-1)
					z.Forward(setr)
					setr.Batch.Wait()
				}
				log.Debugf("FINISH set key %v from zone %v to curr zone %v ",getKey,zoneInfo,router.currentZonePrefix)
				return nil
			}else if !ok {//key中包含的区域信息未配置
				log.Errorf("GET key has zone info BUT not configed at this proxy!!! the key is %v ",getKey)
				return ErrZoneIsNotConfig
			}//else value!="" 为正常(含有区域信息，也在本地取到了值)
			log.Info("GET key has zone info AND find  value at local success")
			return nil
		case "SET":
			//根据key类型设置是同步执行还是异步执行
			if exist{ //存在区域信息
				log.Info("SET key has zone info ,start to set at local")
				if  router.currentZonePrefix!=zoneInfo {
					log.Error("SET key  has zone info BUT not local error")
					return ErrZoneIsNotMatch
				}
				realZone := router.zones[zoneInfo]
				realZone.Forward(r)//失败了就失败了
				for k := range router.zones{
					if k== zoneInfo{
						continue
					}else {
						oR := r
						otherZone := router.zones[k]
						otherZone.ForwardAsync(oR)
					}
				}
				log.Info("FINISH SET key to local sync and to others async")
			}else {//同步执行的时候的一致性，一个事务
				log.Info("SET key NOT has zone info ,start set to all zones")
				transaction := &Transaction{}
				for k,v := range router.zones{
					var trancmpt *SetRedisTrx
					if k==router.currentZonePrefix {//如果为当前区域则直接使用当前req
						zone := v
						trancmpt = &SetRedisTrx{Zone:zone,Req:r}
					}else {
						zone := v
						req := r.MakeSubRequest(1)
						trancmpt = &SetRedisTrx{Zone:zone,Req:&req[0]}
					}
					transaction.Prepare(trancmpt)
				}
				err:=transaction.Exec()
				if err!=nil{
					log.Errorf("SET key to all zones transaction exec error %v start rollback",err)
				    transaction.Rollback()
				    return err
				}else{
				   err = transaction.Commit()
				   if err!=nil {
				   	   log.Errorf("SET key to all zones transaction commit error %v start rollback",err)
					   transaction.Rollback()
					   return err
				   }
				}
				log.Info("FINISH SET key NOT has zone info ,start set to all zones")
			}
			return nil
	default:
		router.mu.RLock()
		defer router.mu.RUnlock()
		// get zone from prefix
		return z.Forward(r) //数据库字段，此部分在这需要强制阻塞住执行获取结果
	}
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
