package models

import (
	"testing"
	"fmt"
)

func TestListZone(t *testing.T) {
	str := `[{"prefix":"A","addr":"172.16.80.2:6379"},{"prefix":"B","addr":"172.16.80.171:26379"}]`

	ret,_ := ListZone([]byte(str))
	fmt.Println(ret[0])
}