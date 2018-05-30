package models

import (
	"encoding/json"
	"fmt"
)

type Zone struct {
	Id   int    `json:"id,omitempty"`
	Addr string	`json:"addr"`
	Prefix string	`json:"prefix"`
}
func (z *Zone) Encode() []byte {
	return jsonEncode(z)
}

func (z *Zone) Dncode(info []byte)  error {
	return jsonDecode(z,info)
}

func ListZone(b []byte) ([]*Zone, error) {
	var zs []*Zone
	fmt.Println(b)
	err := json.Unmarshal(b,&zs)
	fmt.Println(err)
	return zs,err
}