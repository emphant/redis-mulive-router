// Copyright 2018 emphant. All Rights Reserved.
// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"sync"
	"unsafe"

	"github.com/emphant/redis-mulive-router/pkg/proxy/redis"
	"github.com/emphant/redis-mulive-router/pkg/utils/sync2/atomic2"
	"strconv"
)

type Request struct {
	Multi []*redis.Resp
	Batch *sync.WaitGroup
	Group *sync.WaitGroup

	Broken *atomic2.Bool

	OpStr string
	OpFlag

	Database int32
	UnixNano int64

	*redis.Resp
	Err error

	Coalesce func() error
}

func (r *Request) getKey() string{
	if len(r.Multi)>=2 {
		return string(r.Multi[1].Value)
	}
	return ""
}

func (r *Request) IsBroken() bool {
	return r.Broken != nil && r.Broken.IsTrue()
}

func (r *Request) MakeSubRequest(n int) []Request {
	var sub = make([]Request, n)
	for i := range sub {
		x := &sub[i]
		x.Batch = r.Batch
		x.OpStr = r.OpStr
		x.OpFlag = r.OpFlag
		x.Broken = r.Broken
		x.Database = r.Database
		x.UnixNano = r.UnixNano
		x.Multi = make([]*redis.Resp,len(r.Multi))
		for index,v := range r.Multi{
			x.Multi[index]=v
		}
	}
	return sub
}


func (r *Request) CPRequest(n int) []Request {
	var sub = make([]Request, n)
	for i := range sub {
		x := &sub[i]
		x.Batch = &sync.WaitGroup{}
		x.OpStr = r.OpStr
		x.OpFlag = r.OpFlag
		x.Database = r.Database
		x.UnixNano = r.UnixNano
		x.Multi = make([]*redis.Resp,len(r.Multi))
		for index,v := range r.Multi{
			x.Multi[index]=v
		}
	}
	return sub
}

const GOLDEN_RATIO_PRIME_32 = 0x9e370001

func (r *Request) Seed16() uint {
	h32 := uint32(r.UnixNano) + uint32(uintptr(unsafe.Pointer(r)))
	h32 *= GOLDEN_RATIO_PRIME_32
	return uint(h32 >> 16)
}

type RequestChan struct {
	lock sync.Mutex
	cond *sync.Cond

	data []*Request
	buff []*Request

	waits  int
	closed bool
}

const DefaultRequestChanBuffer = 128

func NewRequestChan() *RequestChan {
	return NewRequestChanBuffer(0)
}

func NewRequestChanBuffer(n int) *RequestChan {
	if n <= 0 {
		n = DefaultRequestChanBuffer
	}
	var ch = &RequestChan{
		buff: make([]*Request, n),
	}
	ch.cond = sync.NewCond(&ch.lock)
	return ch
}

func (c *RequestChan) Close() {
	c.lock.Lock()
	if !c.closed {
		c.closed = true
		c.cond.Broadcast()
	}
	c.lock.Unlock()
}

func (c *RequestChan) Buffered() int {
	c.lock.Lock()
	n := len(c.data)
	c.lock.Unlock()
	return n
}

func (c *RequestChan) PushBack(r *Request) int {
	c.lock.Lock()
	n := c.lockedPushBack(r)
	c.lock.Unlock()
	return n
}

func (c *RequestChan) PopFront() (*Request, bool) {
	c.lock.Lock()
	r, ok := c.lockedPopFront()
	c.lock.Unlock()
	return r, ok
}

func (c *RequestChan) lockedPushBack(r *Request) int {
	if c.closed {
		panic("send on closed chan")
	}
	if c.waits != 0 {
		c.cond.Signal()
	}
	c.data = append(c.data, r)
	return len(c.data)
}

func (c *RequestChan) lockedPopFront() (*Request, bool) {
	for len(c.data) == 0 {
		if c.closed {
			return nil, false
		}
		c.data = c.buff[:0]
		c.waits++
		c.cond.Wait()
		c.waits--
	}
	var r = c.data[0]
	c.data, c.data[0] = c.data[1:], nil
	return r, true
}

func (c *RequestChan) IsEmpty() bool {
	return c.Buffered() == 0
}

func (c *RequestChan) PopFrontAll(onRequest func(r *Request) error) error {
	for {
		r, ok := c.PopFront()
		if ok {
			if err := onRequest(r); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
}

func (c *RequestChan) PopFrontAllVoid(onRequest func(r *Request)) {
	c.PopFrontAll(func(r *Request) error {
		onRequest(r)
		return nil
	})
}

func GetRequest(key string) *Request {
	req := &Request{Batch: &sync.WaitGroup{}}
	req.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("get")),
		redis.NewBulkBytes([]byte(key)),
	}
	return req
}

func TTLRequest(key string)  *Request{
	req := &Request{Batch: &sync.WaitGroup{}}
	req.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("ttl")),
		redis.NewBulkBytes([]byte(key)),
	}
	return req
}

func SetRequest(key string,value string,expire int) *Request {
	req := &Request{Batch: &sync.WaitGroup{}}
	if expire<0{
		req.Multi = []*redis.Resp{
			redis.NewBulkBytes([]byte("set")),
			redis.NewBulkBytes([]byte(key)),
			redis.NewBulkBytes([]byte(value)),
		}
	}else {
		req.Multi = []*redis.Resp{
			redis.NewBulkBytes([]byte("set")),
			redis.NewBulkBytes([]byte(key)),
			redis.NewBulkBytes([]byte(value)),
			redis.NewBulkBytes([]byte("EX")),
			redis.NewBulkBytes([]byte(strconv.Itoa(expire))),
		}
	}

	return req
}

func DELRequest(key string)  *Request{
	req := &Request{Batch: &sync.WaitGroup{}}
	req.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("del")),
		redis.NewBulkBytes([]byte(key)),
	}
	return req
}
