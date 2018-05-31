package proxy

import (
	"sync"
	"time"
	"os"
	"os/exec"
	"strings"
	"net"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/utils"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"github.com/emphant/redis-mulive-router/pkg/models"
	"github.com/emphant/redis-mulive-router/pkg/utils/math2"
	"net/http"
	"runtime"
	"github.com/emphant/redis-mulive-router/pkg/utils/unsafe2"
	"math"
)

type StatsFlags uint32

func (s StatsFlags) HasBit(m StatsFlags) bool {
	return (s & m) != 0
}

const (
	StatsCmds = StatsFlags(1 << iota)
	StatsZones
	StatsRuntime

	StatsFull = StatsFlags(math.MaxUint32)
)

type Proxy struct {
	mu sync.Mutex
	online bool
	closed bool
	config *Config
	exit struct {
		C chan struct{}
	}
	router *Router
	model *models.Proxy

	lproxy net.Listener
	ladmin net.Listener
}

func (s *Proxy) serveProxy() {//启动代理tcp服务
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] proxy start service on %s", s, s.lproxy.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) (err error) {
		defer func() {
			eh <- err
		}()
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				return err
			}
			NewSession(c, s.config).Start(s.router)
		}
	}(s.lproxy)

	if d := s.config.BackendPingPeriod.Duration(); d != 0 {
		go s.keepAlive(d)
	}

	select {
	case <-s.exit.C:
		log.Warnf("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) serveAdmin() {//启动web管理http服务
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] admin start service on %s", s, s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Warnf("[%p] admin shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] admin exit on error", s)
	}
}

func (s *Proxy) FillZones() {//填充代理支持的数据中心以及对应的数据节点关系


}


func (s *Proxy) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Proxy) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

var ErrClosedProxy = errors.New("use of closed proxy")

func (s *Proxy) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	if s.online {
		return nil
	}
	s.online = true
	s.router.Start()
	return nil
}

func (s *Proxy) Close() error{
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.ladmin != nil {
		s.ladmin.Close()
	}
	if s.lproxy != nil {
		s.lproxy.Close()
	}
	if s.router != nil {
		s.router.Close()
	}

	return nil

}

func (s *Proxy)  setup(config *Config) error {
	proto := config.ProtoType
	if l, err := net.Listen(proto, config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostProxy)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.ProxyAddr = x
	}

	proto = "tcp"
	if l, err := net.Listen(proto, config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}
	return nil
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay = &DelayExp2{
		Min: 10, Max: 500,
		Unit: time.Millisecond,
	}
	for {
		c, err := l.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				delay.Sleep()
				continue
			}
		}
		return c, err
	}
}

func (s *Proxy) keepAlive(d time.Duration) {
	var ticker = time.NewTicker(math2.MaxDuration(d, time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			s.router.KeepAlive()
		}
	}
}

func (s *Proxy) Model() *models.Proxy {
	return s.model
}

func (s *Proxy) Config() *Config {
	return s.config
}

func (s *Proxy) Zones() []*models.Zone {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.router.GetZones()
}


func New(config *Config) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	if err := models.ValidateProduct(config.ProductName); err != nil {
		return nil, errors.Trace(err)
	}
	s := &Proxy{}

	s.config = config
	s.exit.C = make(chan struct{})
	s.router = NewRouter(config)

	s.model = &models.Proxy{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.DataCenter = config.ProxyDataCenter
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}
	s.model.Hostname = utils.Hostname

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	go s.serveAdmin()
	go s.serveProxy()

	return s, nil
}

type Overview struct {
	Version string         `json:"version"`
	Compile string         `json:"compile"`
	Config  *Config        `json:"config,omitempty"`
	Model   *models.Proxy  `json:"model,omitempty"`
	Stats   *Stats         `json:"stats,omitempty"`
	Zones   []*models.Zone         `json:"zones,omitempty"`
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Sentinels struct {
		Servers  []string          `json:"servers,omitempty"`
		Masters  map[string]string `json:"masters,omitempty"`
		Switched bool              `json:"switched,omitempty"`
	} `json:"sentinels"`

	Ops struct {
		Total int64 `json:"total"`
		Fails int64 `json:"fails"`
		Redis struct {
			Errors int64 `json:"errors"`
		} `json:"redis"`
		QPS int64      `json:"qps"`
		Cmd []*OpStats `json:"cmd,omitempty"`
	} `json:"ops"`

	Sessions struct {
		Total int64 `json:"total"`
		Alive int64 `json:"alive"`
	} `json:"sessions"`

	Rusage struct {
		Now string       `json:"now"`
		CPU float64      `json:"cpu"`
		Mem int64        `json:"mem"`
		Raw *utils.Usage `json:"raw,omitempty"`
	} `json:"rusage"`

	Backend struct {
		PrimaryOnly bool `json:"primary_only"`
	} `json:"backend"`

	Runtime *RuntimeStats `json:"runtime,omitempty"`
}

type RuntimeStats struct {
	General struct {
		Alloc   uint64 `json:"alloc"`
		Sys     uint64 `json:"sys"`
		Lookups uint64 `json:"lookups"`
		Mallocs uint64 `json:"mallocs"`
		Frees   uint64 `json:"frees"`
	} `json:"general"`

	Heap struct {
		Alloc   uint64 `json:"alloc"`
		Sys     uint64 `json:"sys"`
		Idle    uint64 `json:"idle"`
		Inuse   uint64 `json:"inuse"`
		Objects uint64 `json:"objects"`
	} `json:"heap"`

	GC struct {
		Num          uint32  `json:"num"`
		CPUFraction  float64 `json:"cpu_fraction"`
		TotalPauseMs uint64  `json:"total_pausems"`
	} `json:"gc"`

	NumProcs      int   `json:"num_procs"`
	NumGoroutines int   `json:"num_goroutines"`
	NumCgoCall    int64 `json:"num_cgo_call"`
	MemOffheap    int64 `json:"mem_offheap"`
}

func (s *Proxy) Overview(flags StatsFlags) *Overview {
	o := &Overview{
		Version: utils.Version,
		Compile: utils.Compile,
		Config:  s.Config(),
		Model:   s.Model(),
		Stats:   s.Stats(flags),
	}

	if flags.HasBit(StatsZones) {
		o.Zones = s.Zones()
	}
	return o
}

func (s *Proxy) Stats(flags StatsFlags) *Stats {
	stats := &Stats{}
	stats.Online = s.IsOnline()
	stats.Closed = s.IsClosed()



	stats.Ops.Total = OpTotal()
	stats.Ops.Fails = OpFails()
	stats.Ops.Redis.Errors = OpRedisErrors()
	stats.Ops.QPS = OpQPS()

	if flags.HasBit(StatsCmds) {
		stats.Ops.Cmd = GetOpStatsAll()
	}

	stats.Sessions.Total = SessionsTotal()
	stats.Sessions.Alive = SessionsAlive()

	if u := GetSysUsage(); u != nil {
		stats.Rusage.Now = u.Now.String()
		stats.Rusage.CPU = u.CPU
		stats.Rusage.Mem = u.MemTotal()
		stats.Rusage.Raw = u.Usage
	}

	stats.Backend.PrimaryOnly = s.Config().BackendPrimaryOnly

	if flags.HasBit(StatsRuntime) {
		var r runtime.MemStats
		runtime.ReadMemStats(&r)

		stats.Runtime = &RuntimeStats{}
		stats.Runtime.General.Alloc = r.Alloc
		stats.Runtime.General.Sys = r.Sys
		stats.Runtime.General.Lookups = r.Lookups
		stats.Runtime.General.Mallocs = r.Mallocs
		stats.Runtime.General.Frees = r.Frees
		stats.Runtime.Heap.Alloc = r.HeapAlloc
		stats.Runtime.Heap.Sys = r.HeapSys
		stats.Runtime.Heap.Idle = r.HeapIdle
		stats.Runtime.Heap.Inuse = r.HeapInuse
		stats.Runtime.Heap.Objects = r.HeapObjects
		stats.Runtime.GC.Num = r.NumGC
		stats.Runtime.GC.CPUFraction = r.GCCPUFraction
		stats.Runtime.GC.TotalPauseMs = r.PauseTotalNs / uint64(time.Millisecond)
		stats.Runtime.NumProcs = runtime.GOMAXPROCS(0)
		stats.Runtime.NumGoroutines = runtime.NumGoroutine()
		stats.Runtime.NumCgoCall = runtime.NumCgoCall()
		stats.Runtime.MemOffheap = unsafe2.OffheapBytes()
	}
	return stats
}