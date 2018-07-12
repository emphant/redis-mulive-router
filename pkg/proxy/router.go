package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/models"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"strings"
	"runtime"
	"strconv"
	"github.com/emphant/redis-mulive-router/pkg/utils/redis"
)

var (
	ErrZoneIsNotConfig = errors.New("the key requested zone is not exist in current config")
	ErrRespIsRequired = errors.New("resp is required")
	ErrZoneIsNotMatch = errors.New("the key to write zone is not match curr")
	ErrClosedRouter  = errors.New("use of closed router")
	ErrGetconn  = errors.New("poll get addr error,not exist")
	ErrGetMaster  = errors.New("get master form sentinel error")
	ErrGetMasterZoro  = errors.New("get no master from sentinel")
)

// 对请求任务进行分派，选择到对应的数据中心读/写数据
type Router struct {
	mu sync.RWMutex

	config *Config
	zones map[string]*Zone //应该保存更多的信息
	pool *SharedBackendConnPool


	currentZonePrefix string

	online  bool
	closed  bool
	zoneSpr string
	monitor *redis.Sentinel
}

func (s *Router) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.online = true
}

func (s *Router) GetZones() []*models.Zone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	zoneModels := make([]*models.Zone, len(s.zones))
	index := 0
	for i,_ := range s.zones {
		zoneModels[index] = s.zones[i].snapshot()
		index++
	}
	return zoneModels
}

func (router *Router) FillZone(pzones []*models.Zone) error {//完成zone的初始化与填充
	router.mu.Lock()
	defer router.mu.Unlock()
	if router.closed {
		return ErrClosedRouter
	}

	for _,zone := range pzones {
		if zone.Prefix==router.currentZonePrefix && router.zones[router.currentZonePrefix]!=nil{
			continue
		}
		if zone.IsSentinel {
			log.Info(zone)
			log.Println(zone.GetAddrs())
			masters,err := router.monitor.Masters(zone.GetAddrs(),router.config.SentinelSwitchTimeout.Duration())
			if err!=nil {
				log.Error(err)
				return ErrGetMaster
			}
			log.Println(masters)
			if len(masters)==0 {
				log.Error(ErrGetMasterZoro)
				return ErrGetMasterZoro
			}
			for name,addr :=range masters {
				if name==zone.MasterName {
					conn:=router.pool.Retain(addr)
					rZone := NewZone(zone.Id,conn,zone.Prefix,zone.IsSentinel,zone.GetAddrs(),zone.MasterName)
					router.zones[zone.Prefix]=rZone
				}else {
					router.pool.Retain(addr)
				}
			}
		}else {
			conn:=router.pool.Retain(zone.Addrs)
			if nil!=conn {
				rZone := NewZone(zone.Id,conn,zone.Prefix,zone.IsSentinel,zone.GetAddrs(),zone.MasterName)
				router.zones[zone.Prefix]=rZone
			}else {
				log.Errorf("pool get %v addr conn error",zone.GetAddrs())
				return ErrGetconn
			}
		}
	}
	return nil
}

func (router *Router) Close() {//关闭
	router.mu.Lock()
	defer router.mu.Unlock()
	if router.closed {
		return
	}
	for _,zone := range router.zones {
		zone.backend.bc.Release()
		zone.backend.bc = nil
		zone.backend.id = 0
	}
	router.zones=nil
	router.closed = true
}

func (router *Router) isOnline() bool {
	return router.online && !router.closed
}

func (router *Router) KeepAlive() error{//保持连接池在线
	router.mu.RLock()
	defer router.mu.RUnlock()
	if router.closed {
		return ErrClosedRouter
	}
	router.pool.KeepAlive()
	return nil
}



func (router *Router) SentinelSwitch() error{//哨兵工作模式下持续选择master
	router.mu.RLock()
	defer router.mu.RUnlock()
	if router.closed {
		return ErrClosedRouter
	}
	for _,zone := range router.zones {
		if zone.isSentinelMode{
			masters,err := router.monitor.Masters(zone.addrs,router.config.SentinelSwitchTimeout.Duration())
			//log.Info("ENTER ing SentinelSwitch %s \n",masters)
			masterAddr,ok := masters[zone.masterName]
			if err!=nil || !ok {
				log.Errorf("[SentinelSwitch] get Masters %s and get by name not found ",err,ok)
				return err
			}
			oldAddr := zone.backend.bc.addr
			if oldAddr != masterAddr{
				conn := router.pool.Retain(masterAddr)
				zone.ChangeConn(conn)
			}
		}
	}
	return nil
}

func (router *Router) dispatch(r *Request) error{//依照req转发到相应zone
	defer func() {//defer 防止出现panic,并打印栈
		if err := recover(); err != nil {
			var buf [8192]byte
			n := runtime.Stack(buf[:],false)
			log.Errorf("PANIC_ERROR %v %v",err,string(buf[:n]))
		}
	}()
	getKey := r.getKey()
	zoneInfo,exist,ok:=router.getZoneInfo(getKey)
	z := router.zones[router.currentZonePrefix]
	switch r.OpStr {
		case "GET":
			router.mu.RLock()
			defer router.mu.RUnlock()
			// get zone from prefix
			z.Forward(r) //数据库字段，此部分在这需要强制阻塞住执行获取结果
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
				r.Batch.Wait()
				//若有返回信息，并new一个request写入本地区域,ttl信息
				other_val := string(r.Resp.Value)
				if other_val!=""{
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
				}
				return nil
			}else if !ok {//key中包含的区域信息未配置
				log.Errorf("GET key has zone info BUT not configed at this proxy!!! the key is %v ",getKey)
				return ErrZoneIsNotConfig
			}//else value!="" 为正常(含有区域信息，也在本地取到了值)
			//log.Info("GET key has zone info AND find  value at local success")
			return nil
		case "SET":
			if exist{ //存在区域信息
				//log.Info("SET key has zone info ,start to set at local")
				if  router.currentZonePrefix!=zoneInfo {
					log.Error("SET key  has zone info BUT not local error")
					return ErrZoneIsNotMatch
				}
				realZone := router.zones[zoneInfo]
				realZone.Forward(r)//失败了就失败了
				//r.Batch.Wait()
				for k := range router.zones{
					if k== zoneInfo{
						continue
					}else {
						//oR := r
						oR := &r.CPRequest(1)[0]
						otherZone := router.zones[k]
						otherZone.ForwardAsync(oR)
					}
				}
				//log.Info("FINISH SET key to local sync and to others async")
			}else {//同步执行的时候的一致性，一个事务
				//log.Info("SET key NOT has zone info ,start set to all zones")
				transaction := &Transaction{}
				for k,v := range router.zones{
					var trancmpt *SetRedisTrx
					if k==router.currentZonePrefix {//如果为当前区域则直接使用当前req
						zone := v
						trancmpt = &SetRedisTrx{Zone:zone,flag:false,Req:r}
					}else {
						zone := v
						req := r.CPRequest(1)
						trancmpt = &SetRedisTrx{Zone:zone,flag:false,Req:&req[0]}
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
				//log.Info("FINISH SET key NOT has zone info ,start set to all zones")
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
	keyArray := strings.Split(key,router.zoneSpr)
	if len(keyArray) ==1 {
		return "",false,false
	}
	zoneInfo :=keyArray[0]
	_,ok := router.zones[zoneInfo]
	if ok {
		return zoneInfo,true,true
	}else {
		return "",true,false
	}

}

func loadZoneInfo(z *Zone,key string) ( []*models.Zone, error) {
	r := GetRequest(key)
	z.Forward(r)
	r.Batch.Wait()

	if r.Value!=nil{
		log.Println(string(r.Value))
		ret,err := models.ListZone(r.Value)
		if err==nil {
			return ret,nil
		}
		return nil,err
	}else {
		return nil,nil
	}

}

func NewRouter(config *Config)  *Router{
	s := &Router{config: config}

	s.currentZonePrefix=config.CurrZonePrefix
	s.zoneSpr = config.ZoneSpr4key

	if config.SentinelMode {
		monitor := redis.NewSentinel(config.ProductName, config.ProductAuth)
		s.monitor=monitor

		//go
		//go
	}

	s.pool = NewSharedBackendConnPool(config, config.BackendPrimaryParallel)
	s.zones = make(map[string]*Zone)

	log.Println("called")
	zonemA := models.Zone{1,config.CurrZoneAddr,config.CurrZonePrefix,config.SentinelMode,"mymaster",false}
	s.FillZone([]*models.Zone{&zonemA})


	realZone := s.zones[config.CurrZonePrefix]
	zones,err := loadZoneInfo(realZone,config.ZoneInfoKey)

	log.Println(len(zones))
	if err==nil {
		s.FillZone(zones)
	}else {
		log.Errorf("new router while get zones error %v",err)
	}
	return s
}
