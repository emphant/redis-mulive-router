package proxy

import "github.com/emphant/redis-mulive-router/pkg/utils/errors"

//rollback 方法


//多机执行问题


//check all 执行
var (
	ErrTransCommit = errors.New("the trancsaction compants to commit not equal all")
	ErrTransCmptCheck = errors.New("one trancsaction compant check error")
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


func (t *Transaction)  Exec()  error{
	defer func() {

	}()
	//TODO 并发/依次执行
	for _,transaction := range t.trans{
		transaction.Exec()
		t.execed=append(t.execed,transaction)
	}
	return nil
}

func (t *Transaction)  Commit()  error{
	if len(t.execed) != len(t.trans){
		return ErrTransCommit
	}
        for _,cmpt := range t.execed {
             success := cmpt.Check()
             if !success {
                 return ErrTransCmptCheck
             }
        }
	// 出error了
	return nil
}

func (t *Transaction)  Rollback()  {
	// 失败回滚
	for _,transaction := range t.execed{
		transaction.Rollback()
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
}

func (trxc *SetRedisTrx) Exec()  error{
	err := trxc.Zone.Forward(trxc.Req)
	return err
}

func (trxc *SetRedisTrx) Check() bool {
	//等待执行完成
	trxc.Req.Batch.Wait()
	if trxc.Req.Value!=nil && string(trxc.Req.Value)=="OK" {
            trxc.flag=true
	}
	return trxc.flag
}

func (trxc *SetRedisTrx) Rollback() {
	//执行不用等待执行结果的即可
	delr := DELRequest(trxc.Req.getKey())
	trxc.Zone.Forward(delr)
	delr.Batch.Wait()
}