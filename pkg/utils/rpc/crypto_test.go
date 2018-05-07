package rpc

import (
	"testing"
	"fmt"
)

func TestNewXAuth(t *testing.T) {
	//token := NewToken(
	//	"codis-demo",
	//	"0.0.0.0:19000",
	//	"0.0.0.0:11080",
	//)
	info := NewXAuth( "codis-demo","","6ca864cbfec4cacc13b100e8e3c9edfe")
	fmt.Println(info)
}
