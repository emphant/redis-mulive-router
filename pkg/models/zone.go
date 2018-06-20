package models

import (
	"encoding/json"
	"strings"
)

type Zone struct {
	Id   int    `json:"id,omitempty"`
	Addrs string	`json:"addrs"`
	Prefix string	`json:"prefix"`
	IsSentinel bool	`json:"is_sentinel"`
	MasterName string	`json:"master_name"`
	IsConnected bool	`json:"is_connected,omitempty"`
}
func (z *Zone) Encode() []byte {
	return jsonEncode(z)
}

func (z *Zone) Dncode(info []byte)  error {
	return jsonDecode(z,info)
}

func (z *Zone) GetAddrs()  []string {
	return strings.Split(z.Addrs,",")
}

func ListZone(b []byte) ([]*Zone, error) {
	var zs []*Zone
	err := json.Unmarshal(b,&zs)
	return zs,err
}