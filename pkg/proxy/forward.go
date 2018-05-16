package proxy


//同步处理方法
type forwardMethod interface {
	GetId() int
	Forward(z *Zone,r *Request) error
}


type forwardSync struct {//同步写入方法
	//forwardHelper
}

func (d *forwardSync) GetId() int {
	return 0
}

func (d *forwardSync) Forward(z *Zone,r *Request) error{
	z.lock.RLock()
	bc, err := d.process(z, r)
	z.lock.RUnlock()
	if err != nil {
		return err
	}
	bc.PushBack(r)
	return nil
}
func (sync *forwardSync) process(z *Zone, r *Request) (*BackendConn, error) {
	var database, seed = r.Database, r.Seed16()
	return z.backend.bc.BackendConn(database, seed, true), nil
}


//异步处理方法
type forwardASync struct {//异步写入方法
	//forwardHelper
}

func (d *forwardASync) GetId() int {
	return 1
}

func (d *forwardASync) Forward(z *Zone,r *Request) error{
	return nil
}