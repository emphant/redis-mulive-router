package pool

import "github.com/emphant/redis-mulive-router/pkg/utils/math2"

type sharedBackendConnPool struct {
	config   *Config
	//not know
	parallel int

	pool map[string]*sharedBackendConn
}

func newSharedBackendConnPool(config *Config, parallel int) *sharedBackendConnPool {
	p := &sharedBackendConnPool{
		config: config, parallel: math2.MaxInt(1, parallel),
	}
	p.pool = make(map[string]*sharedBackendConn)
	return p
}

func (p *sharedBackendConnPool) KeepAlive() {
	for _, bc := range p.pool {
		bc.KeepAlive()
	}
}

func (p *sharedBackendConnPool) Get(addr string) *sharedBackendConn {
	return p.pool[addr]
}

func (p *sharedBackendConnPool) Retain(addr string) *sharedBackendConn {
	if bc := p.pool[addr]; bc != nil {
		return bc.Retain()
	} else {
		bc = newSharedBackendConn(addr, p)
		p.pool[addr] = bc
		return bc
	}
}
