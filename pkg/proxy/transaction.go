package proxy

import (
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"runtime"
)

var (
	ErrTransCommit = errors.New("the trancsaction compants to commit not equal all")
	ErrTransCmptCheck = errors.New("one trancsaction compant check error")
	ErrTransCmptExec = errors.New("one trancsaction compant exec error")
)

type Transaction struct{
	id int
	rollback func()
	mode int
	trans []TrxCompants
	execed []TrxCompants
}

func (t *Transaction)  Prepare(tran TrxCompants)  {
	//检查各节点是否正常
	t.trans = append(t.trans,tran)
}

func (t *Transaction)  Exec() (err2 error){
	defer func() {//defer 防止出现panic,并打印栈
		if err := recover(); err != nil {
			var buf [8192]byte
			n := runtime.Stack(buf[:],false)
			log.Errorf("Commit PANIC_ERROR %v %v",err,string(buf[:n]))
			err2 = ErrTransCmptExec
		}
	}()
	for i := 0; i < len(t.trans); i++{
		err:=t.trans[i].Exec()
		if err != nil {
			return err
		}
		t.execed=append(t.execed,t.trans[i])
	}
	return nil
}

func (t *Transaction)  Commit()  (err2 error){
	defer func() {//defer 防止出现panic,并打印栈
		if err := recover(); err != nil {
			var buf [8192]byte
			n := runtime.Stack(buf[:],false)
			log.Errorf("Commit PANIC_ERROR %v %v",err,string(buf[:n]))
			err2 = ErrTransCmptCheck
		}
	}()
	if len(t.execed) != len(t.trans){
		return ErrTransCommit
	}
	for i := 0; i < len(t.execed); i++ {
		success := t.execed[i].Check()
		if !success {
			return ErrTransCmptCheck
		}
	}
	// 出error了
	return
}

func (t *Transaction)  Rollback()  {
	// 失败回滚
	for i := 0; i < len(t.execed); i++{
		t.execed[i].Rollback()
	}
}

type TrxCompants interface{
	Rollback()
	Exec() error
	Check() bool//在commit阶段被调用，用于检查是否与预期一致
}


type SetRedisTrx struct {
	Zone *Zone
	flag bool
	Req *Request
	oldV string
}

func (trxc *SetRedisTrx) Exec()  error{
	//find old data use for rollback
	greq := GetRequest(trxc.Req.getKey())
	err:=trxc.Zone.Forward(greq)
	if err!=nil {
		return err
	}
	greq.Batch.Wait();
	trxc.oldV=string(greq.Resp.Value)
	err = trxc.Zone.Forward(trxc.Req)
	return err
}

func (trxc *SetRedisTrx) Check() bool {
	//等待执行完成
	trxc.Req.Batch.Wait()
	if trxc.Req.Resp.Value!=nil && string(trxc.Req.Resp.Value)=="OK" {
		trxc.flag=true
	}
	//time.Sleep(10*time.Second)
	return trxc.flag
}

func (trxc *SetRedisTrx) Rollback() {
	//执行不用等待执行结果的即可
	if trxc.oldV!="" {
		setr := SetRequest(trxc.Req.getKey(),trxc.oldV,-1)//has no ttl
		trxc.Zone.Forward(setr)
		setr.Batch.Wait()
	}else {
		delr := DELRequest(trxc.Req.getKey())
		trxc.Zone.Forward(delr)
		delr.Batch.Wait()
	}
}