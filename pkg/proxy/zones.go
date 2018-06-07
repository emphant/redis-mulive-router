package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/models"
)

// 管理数据中心
type Zone struct {
	id   int
	lock struct {
		hold bool
		sync.RWMutex
	}
	backend struct {
		id int
		lock sync.Mutex
		bc *SharedBackendConn
	}
	isSentinelMode bool
	addrs []string
	masterName string //
	prefix string //区域标识前缀
	//method forwardMethod // 应该所有的都需要支持
}

func (sync *Zone) process(z *Zone, r *Request) (*BackendConn, error) {
	var database, seed = r.Database, r.Seed16()
	return z.backend.bc.BackendConn(database, seed, true), nil
}

func (z *Zone) Forward(r *Request) error {
	z.lock.RLock()
	bc, err := z.process(z, r)
	z.lock.RUnlock()
	if err != nil {
		return err
	}
	bc.PushBack(r)
	return nil
}

func (z *Zone) ChangeConn(conn *SharedBackendConn)  {
	z.backend.lock.Lock()
	defer z.backend.lock.Unlock()
	z.backend.bc.Release()
	z.backend.bc = conn
}



func (z *Zone) ForwardAsync(r *Request) error {
	z.lock.RLock()
	bc, err := z.process(z, r)
	z.lock.RUnlock()
	if err != nil {
		return err
	}
	bc.PushBack(r)
	return nil
}

func (z *Zone) snapshot() *models.Zone {
	var m = &models.Zone{
		Prefix:z.prefix,
		Addrs: z.backend.bc.Addr(),
	}
	return m
}

func NewZone(id int, conn *SharedBackendConn,prefix string,isSentinel bool,addrs []string,masterName string) *Zone {
	z := &Zone{}
	z.id=id
	z.lock.hold=false
	//z.lock.RWMutex=sync.RWMutex
	z.backend.id=id
	z.backend.bc=conn
	z.prefix=prefix
	z.isSentinelMode=isSentinel
	z.addrs=addrs
	z.masterName=masterName
	return z
}

//状态快照