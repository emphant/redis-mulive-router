package models

type Zone struct {
	Id   int    `json:"id,omitempty"`
	Addr string	`json:"addr"`
	Prefix string	`json:"prefix"`
}
func (z *Zone) Encode() []byte {
	return jsonEncode(z)
}