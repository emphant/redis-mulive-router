package proxy

import (
	"net"
	"time"
	"sync"
	"github.com/emphant/redis-mulive-router/pkg/proxy/redis"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/utils/sync2/atomic2"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"fmt"
)

//代理会话管理、处理
type Session struct {
	Conn *redis.Conn

	CreateUnix int64
	LastOpUnix int64

	config *Config

	Ops int64

	database int32

	stats struct {//内部状态统计
		opmap map[string]*opStats
		total atomic2.Int64
		fails atomic2.Int64
		flush struct {
			n    uint
			nano int64
		}
	}

	start sync.Once
	exit sync.Once
	quit bool
	broken atomic2.Bool
}

var (
	ErrRouterNotOnline          = errors.New("router is not online")
	ErrTooManySessions          = errors.New("too many sessions")
	ErrTooManyPipelinedRequests = errors.New("too many pipelined requests")
)

func (s *Session) CloseWithError(err error) error {
	s.exit.Do(func() {
		if err != nil {
			log.Infof("session [%p] closed: %s, error: %s", s, s, err)
		} else {
			log.Infof("session [%p] closed: %s, quit", s, s)
		}
	})
	s.broken.Set(true)
	return s.Conn.Close()
}

func (s *Session) CloseReaderWithError(err error) error {
	s.exit.Do(func() {
		if err != nil {
			log.Infof("session [%p] closed: %s, error: %s", s, s, err)
		} else {
			log.Infof("session [%p] closed: %s, quit", s, s)
		}
	})
	return s.Conn.CloseReader()
}

func (s *Session) Start(d *Router) {
	s.start.Do(func() {
		if int(incrSessions()) > s.config.ProxyMaxClients {
			go func() {
				s.Conn.Encode(redis.NewErrorf("ERR max number of clients reached"), true)
				s.CloseWithError(ErrTooManySessions)
				s.incrOpFails(nil, nil)
				s.flushOpStats(true)
			}()
			decrSessions()
			return
		}

		if !d.isOnline() {
			go func() {
				s.Conn.Encode(redis.NewErrorf("ERR router is not online"), true)
				s.CloseWithError(ErrRouterNotOnline)
				s.incrOpFails(nil, nil)
				s.flushOpStats(true)
			}()
			decrSessions()
			return
		}

		tasks := NewRequestChanBuffer(1024)

		go func() {
			s.loopWriter(tasks)
			decrSessions()
		}()

		go func() {
			s.loopReader(tasks, d)
			tasks.Close()
		}()
	})
}

func (s *Session) loopReader(tasks *RequestChan, d *Router) (err error) {//从端口读入转码成request
	defer func() {
		s.CloseReaderWithError(err)
	}()

	var (
		breakOnFailure = s.config.SessionBreakOnFailure
		maxPipelineLen = s.config.SessionMaxPipeline
	)
	for !s.quit {
		multi, err := s.Conn.DecodeMultiBulk()
		if err != nil {
			return err
		}
		s.incrOpTotal()

		if tasks.Buffered() > maxPipelineLen {
			return s.incrOpFails(nil, ErrTooManyPipelinedRequests)
		}

		start := time.Now()
		s.LastOpUnix = start.Unix()
		s.Ops++

		r := &Request{}
		r.Multi = multi
		r.Batch = &sync.WaitGroup{}
		r.Database = s.database
		r.UnixNano = start.UnixNano()

		if err := s.handleRequest(r, d); err != nil {
			r.Resp = redis.NewErrorf("ERR handle request, %s", err)
			tasks.PushBack(r)
			if breakOnFailure {
				return err
			}
		} else {
			tasks.PushBack(r)
		}
	}
	return nil
}

func (s *Session) loopWriter(tasks *RequestChan) (err error) {//处理完成后转码成resp返回
	defer func() {
		s.CloseWithError(err)
		tasks.PopFrontAllVoid(func(r *Request) {
			s.incrOpFails(r, nil)
		})
		s.flushOpStats(true)
	}()

	var (
		breakOnFailure = s.config.SessionBreakOnFailure
		maxPipelineLen = s.config.SessionMaxPipeline
	)

	p := s.Conn.FlushEncoder()
	p.MaxInterval = time.Millisecond
	p.MaxBuffered = maxPipelineLen / 2

	return tasks.PopFrontAll(func(r *Request) error {
		resp, err := s.handleResponse(r)
		if err != nil {
			resp = redis.NewErrorf("ERR handle response, %s", err)
			if breakOnFailure {
				s.Conn.Encode(resp, true)
				return s.incrOpFails(r, err)
			}
		}
		if err := p.Encode(resp); err != nil {
			return s.incrOpFails(r, err)
		}
		fflush := tasks.IsEmpty()
		if err := p.Flush(fflush); err != nil {
			return s.incrOpFails(r, err)
		} else {
			s.incrOpStats(r, resp.Type)
		}
		if fflush {
			s.flushOpStats(false)
		}
		return nil
	})
}


func (s *Session) handleRequest(r *Request, d *Router) error {//视情况看是否需要路由转发
	opstr, flag, err := getOpInfo(r.Multi)
	if err != nil {
		return err
	}
	r.OpStr = opstr
	r.OpFlag = flag
	r.Broken = &s.broken

	if flag.IsNotAllowed() {
		return fmt.Errorf("command '%s' is not allowed", opstr)
	}
	return d.dispatch(r)
}

func (s *Session) handleResponse(r *Request) (*redis.Resp, error) {
	r.Batch.Wait()
	if r.Coalesce != nil {
		if err := r.Coalesce(); err != nil {
			return nil, err
		}
	}
	if err := r.Err; err != nil {
		return nil, err
	} else if r.Resp == nil {
		return nil, ErrRespIsRequired
	}
	return r.Resp, nil
}

func (s *Session) incrOpTotal() {
	s.stats.total.Incr()
}

func (s *Session) getOpStats(opstr string) *opStats {
	e := s.stats.opmap[opstr]
	if e == nil {
		e = &opStats{opstr: opstr}
		s.stats.opmap[opstr] = e
	}
	return e
}

func (s *Session) incrOpStats(r *Request, t redis.RespType) {
	e := s.getOpStats(r.OpStr)
	e.calls.Incr()
	e.nsecs.Add(time.Now().UnixNano() - r.UnixNano)
	switch t {
	case redis.TypeError:
		e.redis.errors.Incr()
	}
}

func (s *Session) incrOpFails(r *Request, err error) error {
	if r != nil {
		e := s.getOpStats(r.OpStr)
		e.fails.Incr()
	} else {
		s.stats.fails.Incr()
	}
	return err
}

func (s *Session) flushOpStats(force bool) {
	var nano = time.Now().UnixNano()
	if !force {
		const period = int64(time.Millisecond) * 100
		if d := nano - s.stats.flush.nano; d < period {
			return
		}
	}
	s.stats.flush.nano = nano

	incrOpTotal(s.stats.total.Swap(0))
	incrOpFails(s.stats.fails.Swap(0))
	for _, e := range s.stats.opmap {
		if e.calls.Int64() != 0 || e.fails.Int64() != 0 {
			incrOpStats(e)
		}
	}
	s.stats.flush.n++

	if len(s.stats.opmap) <= 32 {
		return
	}
	if (s.stats.flush.n % 16384) == 0 {
		s.stats.opmap = make(map[string]*opStats, 32)
	}
}

func NewSession(sock net.Conn, config *Config) *Session {
	c := redis.NewConn(sock,
		config.SessionRecvBufsize.AsInt(),
		config.SessionSendBufsize.AsInt(),
	)
	c.ReaderTimeout = config.SessionRecvTimeout.Duration()
	c.WriterTimeout = config.SessionSendTimeout.Duration()
	c.SetKeepAlivePeriod(config.SessionKeepAlivePeriod.Duration())

	s := &Session{
		Conn: c, config: config,
		CreateUnix: time.Now().Unix(),
	}
	s.stats.opmap = make(map[string]*opStats, 16)
	log.Infof("session [%p] create: %s", s, s)
	return s
}