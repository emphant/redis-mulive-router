package proxy

import (
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/pool"
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
		bc *pool.SharedBackendConn
	}
}


//状态快照