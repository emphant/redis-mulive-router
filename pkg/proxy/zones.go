package proxy

import (
	"sync"
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
		bc *SharedBackendConn
	}
	method forwardMethod
}

func (s *Zone) forward(r *Request) error {
	return s.method.Forward(s, r)
}

//状态快照