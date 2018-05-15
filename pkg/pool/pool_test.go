package pool

import (
	"testing"
	"github.com/emphant/redis-mulive-router/pkg/utils/math2"
	"net"
	"time"
	"sync"
	"strconv"
	"github.com/emphant/redis-mulive-router/pkg/pool/redis"
	"github.com/emphant/redis-mulive-router/pkg/utils/assert"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"fmt"
)

var p *SharedBackendConnPool

var addr = "172.16.1.7:6379"


func tinit()  {
	config := NewDefaultConfig()
	p = &SharedBackendConnPool{
		config: config, parallel: math2.MaxInt(1, config.BackendPrimaryParallel),
	}
	p.pool = make(map[string]*SharedBackendConn)

	p.Retain(addr)
}

func TestError(t *testing.T) {
	tinit()
	conn := p.Get(addr)
	conn.KeepAlive()

	rConn := conn.BackendConn(0,0,true)
	for  {
		if rConn.IsConnected() {
			break;
		}else {
			log.Println("wating connected")
			time.Sleep(1* time.Second)
		}
	}

	for i:=0;i<1000 ;i++ {
		log.Println("starting request %d",i)
		req := &Request{Batch: &sync.WaitGroup{}}
		req.Multi = []*redis.Resp{
			//redis.NewInt([]byte("keys *")),
			redis.NewBulkBytes([]byte("keys *")),
		}
		rConn.PushBack(req)
		req.Batch.Wait()//MARK 此种情况上一个请求处于阻塞状态，所以不会有新的请求过来，再分析实际的情况
		time.Sleep(1*time.Second)
	}




}

func TestItoa(t *testing.T) {
	fmt.Printf("%d",f())
	tinit()
	conn := p.Get(addr)
	conn.KeepAlive()

	rConn := conn.BackendConn(0,0,true)
	for  {
		if rConn.IsConnected() {
			break;
		}else {
			log.Println("wating connected")
			time.Sleep(1* time.Second)
		}
	}

	fmt.Println(rConn.IsConnected())

	//conn.Release()
	req := &Request{Batch: &sync.WaitGroup{}}
	req.Multi = []*redis.Resp{
		//redis.NewInt([]byte("keys *")),
		redis.NewBulkBytes([]byte("keys *")),
	}
	rConn.PushBack(req)
	req.Batch.Wait()
	fmt.Printf("resp is %v",string(req.Resp.Value))


	//TODO 测试sleep状态下的server断开情况
	time.Sleep(1000* time.Second)
}

func newConnPair(config *Config) (*redis.Conn, *BackendConn) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	const bufsize = 128 * 1024

	cc := make(chan *redis.Conn, 1)
	go func() {
		defer close(cc)
		c, err := l.Accept()
		assert.MustNoError(err)
		cc <- redis.NewConn(c, bufsize, bufsize)
	}()

	bc := NewBackendConn(l.Addr().String(), 0, config)
	return <-cc, bc
}

func TestBackend(t *testing.T) {
	config := NewDefaultConfig()
	config.BackendMaxPipeline = 0
	config.BackendSendTimeout.Set(time.Second)
	config.BackendRecvTimeout.Set(time.Minute)

	conn, bc := newConnPair(config)

	var array = make([]*Request, 16384)
	for i := range array {
		array[i] = &Request{Batch: &sync.WaitGroup{}}
	}

	go func() {
		defer conn.Close()
		time.Sleep(time.Millisecond * 300)
		for i, _ := range array {
			_, err := conn.Decode()
			assert.MustNoError(err)
			//log.Println("hh")
			//log.Printf("get resp from %d and resp is %v",i,resp)
			//conn.Encode(resp, true)
			resp := redis.NewString([]byte(strconv.Itoa(i)))
			assert.MustNoError(conn.Encode(resp, true))
		}
	}()

	defer bc.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go func() {
		for i := 0; i < 1000; i++ {
			<-ticker.C
		}
		log.Panicf("timeout")
	}()

	for _, r := range array {
		bc.PushBack(r)
	}
	for i, r := range array {
		r.Batch.Wait()
		assert.MustNoError(r.Err)
		log.Println(r.Resp.Value)
		assert.Must(r.Resp != nil)
		assert.Must(string(r.Resp.Value) == strconv.Itoa(i))
	}
}

func f() ( int)  {
	t := 5
	defer func() {
		t = t + 5
	}()
	return t
}