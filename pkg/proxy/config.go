// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/emphant/redis-mulive-router/pkg/utils/bytesize"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/utils/timesize"
)

const DefaultConfig = `
##################################################
#                                                #
#           RedisMulive-Router                   #
#                                                #
##################################################

# Set Product Name/Auth.
product_name = "redismulive-router"
product_auth = ""

# Set auth for client session
#   1. product_auth is used for auth validation among codis-dashboard,
#      codis-proxy and codis-server.
#   2. session_auth is different from product_auth, it requires clients
#      to issue AUTH <PASSWORD> before processing any other commands.
session_auth = ""

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:21080"

# Set bind address for proxy, proto_type can be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
proto_type = "tcp4"
proxy_addr = "0.0.0.0:29000"


# Set datacenter of proxy.
proxy_datacenter = ""

curr_zone_prefix = "A"
curr_zone_addr = "172.16.80.2:6379"
zone_info_key = "RMR_ZONE_INFO"
zone_spr_4key = "::"
sentinel_mode = true
sentinel_switch_period = "5s"
sentinel_switch_timeout = "3s"

# Set max number of alive sessions.
proxy_max_clients = 1000

# Set max offheap memory size. (0 to disable)
proxy_max_offheap_size = "1024mb"

# Set heap placeholder to reduce GC frequency.
proxy_heap_placeholder = "256mb"

# Proxy will ping backend redis (and clear 'MASTERDOWN' state) in a predefined interval. (0 to disable)
backend_ping_period = "5s"

# Set backend recv buffer size & timeout.
backend_recv_bufsize = "128kb"
backend_recv_timeout = "30s"

# Set backend send buffer & timeout.
backend_send_bufsize = "128kb"
backend_send_timeout = "30s"

# Set backend pipeline buffer size.
backend_max_pipeline = 20480

# Set backend never read replica groups, default is false
backend_primary_only = false

# Set backend parallel connections per server
backend_primary_parallel = 1
backend_replica_parallel = 1

# Set backend tcp keepalive period. (0 to disable)
backend_keepalive_period = "75s"

# Set number of databases of backend.
backend_number_databases = 16

# If there is no request from client for a long time, the connection will be closed. (0 to disable)
# Set session recv buffer size & timeout.
session_recv_bufsize = "128kb"
session_recv_timeout = "30m"

# Set session send buffer size & timeout.
session_send_bufsize = "64kb"
session_send_timeout = "30s"

# Make sure this is higher than the max number of requests for each pipeline request, or your client may be blocked.
# Set session pipeline buffer size.
session_max_pipeline = 10000

# Set session tcp keepalive period. (0 to disable)
session_keepalive_period = "75s"

# Set session to be sensitive to failures. Default is false, instead of closing socket, proxy will send an error response to client.
session_break_on_failure = false


`

type Config struct {
	ProtoType string `toml:"proto_type" json:"proto_type"`
	ProxyAddr string `toml:"proxy_addr" json:"proxy_addr"`
	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	HostProxy string `toml:"-" json:"-"`
	HostAdmin string `toml:"-" json:"-"`


	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`
	SessionAuth string `toml:"session_auth" json:"-"`

	ProxyDataCenter      string         `toml:"proxy_datacenter" json:"proxy_datacenter"`
	ProxyMaxClients      int            `toml:"proxy_max_clients" json:"proxy_max_clients"`
	ProxyMaxOffheapBytes bytesize.Int64 `toml:"proxy_max_offheap_size" json:"proxy_max_offheap_size"`
	ProxyHeapPlaceholder bytesize.Int64 `toml:"proxy_heap_placeholder" json:"proxy_heap_placeholder"`

	CurrZonePrefix string `toml:"curr_zone_prefix" json:"curr_zone_prefix"`
	CurrZoneAddr   string `toml:"curr_zone_addr" json:"curr_zone_addr"`
	ZoneInfoKey  string `toml:"zone_info_key" json:"zone_info_key"`
	ZoneSpr4key  string `toml:"zone_spr_4key" json:"zone_spr_4key"`
	SentinelMode     bool              `toml:"sentinel_mode" json:"sentinel_mode"`
	SentinelSwitchPeriod      timesize.Duration `toml:"sentinel_switch_period" json:"sentinel_switch_period"`
	SentinelSwitchTimeout      timesize.Duration `toml:"sentinel_switch_timeout" json:"sentinel_switch_timeout"`

	BackendPingPeriod      timesize.Duration `toml:"backend_ping_period" json:"backend_ping_period"`
	BackendRecvBufsize     bytesize.Int64    `toml:"backend_recv_bufsize" json:"backend_recv_bufsize"`
	BackendRecvTimeout     timesize.Duration `toml:"backend_recv_timeout" json:"backend_recv_timeout"`
	BackendSendBufsize     bytesize.Int64    `toml:"backend_send_bufsize" json:"backend_send_bufsize"`
	BackendSendTimeout     timesize.Duration `toml:"backend_send_timeout" json:"backend_send_timeout"`
	BackendMaxPipeline     int               `toml:"backend_max_pipeline" json:"backend_max_pipeline"`
	BackendPrimaryOnly     bool              `toml:"backend_primary_only" json:"backend_primary_only"`
	BackendPrimaryParallel int               `toml:"backend_primary_parallel" json:"backend_primary_parallel"`
	BackendReplicaParallel int               `toml:"backend_replica_parallel" json:"backend_replica_parallel"`
	BackendKeepAlivePeriod timesize.Duration `toml:"backend_keepalive_period" json:"backend_keepalive_period"`
	BackendNumberDatabases int32             `toml:"backend_number_databases" json:"backend_number_databases"`

	SessionRecvBufsize     bytesize.Int64    `toml:"session_recv_bufsize" json:"session_recv_bufsize"`
	SessionRecvTimeout     timesize.Duration `toml:"session_recv_timeout" json:"session_recv_timeout"`
	SessionSendBufsize     bytesize.Int64    `toml:"session_send_bufsize" json:"session_send_bufsize"`
	SessionSendTimeout     timesize.Duration `toml:"session_send_timeout" json:"session_send_timeout"`
	SessionMaxPipeline     int               `toml:"session_max_pipeline" json:"session_max_pipeline"`
	SessionKeepAlivePeriod timesize.Duration `toml:"session_keepalive_period" json:"session_keepalive_period"`
	SessionBreakOnFailure  bool              `toml:"session_break_on_failure" json:"session_break_on_failure"`

}

func NewDefaultConfig() *Config {
	c := &Config{}
	if _, err := toml.Decode(DefaultConfig, c); err != nil {
		log.PanicErrorf(err, "decode toml failed")
	}
	if err := c.Validate(); err != nil {
		log.PanicErrorf(err, "validate config failed")
	}
	return c
}

func (c *Config) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return errors.Trace(err)
	}
	return c.Validate()
}

func (c *Config) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}

func (c *Config) Validate() error {
	if c.ProtoType == "" {
		return errors.New("invalid proto_type")
	}
	if c.ProxyAddr == "" {
		return errors.New("invalid proxy_addr")
	}
	if c.AdminAddr == "" {
		return errors.New("invalid admin_addr")
	}

	if c.ProductName == "" {
		return errors.New("invalid product_name")
	}
	if c.ProxyMaxClients < 0 {
		return errors.New("invalid proxy_max_clients")
	}

	const MaxInt = bytesize.Int64(^uint(0) >> 1)

	if d := c.ProxyMaxOffheapBytes; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_max_offheap_size")
	}
	if d := c.ProxyHeapPlaceholder; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_heap_placeholder")
	}
	if c.SentinelSwitchPeriod < 0 {
		return errors.New("invalid sentinel_switch_period")
	}
	if c.SentinelSwitchTimeout < 0 {
		return errors.New("invalid sentinel_switch_period")
	}

	if c.BackendPingPeriod < 0 {
		return errors.New("invalid backend_ping_period")
	}

	if d := c.BackendRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_recv_bufsize")
	}
	if c.BackendRecvTimeout < 0 {
		return errors.New("invalid backend_recv_timeout")
	}
	if d := c.BackendSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_send_bufsize")
	}
	if c.BackendSendTimeout < 0 {
		return errors.New("invalid backend_send_timeout")
	}
	if c.BackendMaxPipeline < 0 {
		return errors.New("invalid backend_max_pipeline")
	}
	if c.BackendPrimaryParallel < 0 {
		return errors.New("invalid backend_primary_parallel")
	}
	if c.BackendReplicaParallel < 0 {
		return errors.New("invalid backend_replica_parallel")
	}
	if c.BackendKeepAlivePeriod < 0 {
		return errors.New("invalid backend_keepalive_period")
	}
	if c.BackendNumberDatabases < 1 {
		return errors.New("invalid backend_number_databases")
	}

	if d := c.SessionRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_recv_bufsize")
	}
	if c.SessionRecvTimeout < 0 {
		return errors.New("invalid session_recv_timeout")
	}
	if d := c.SessionSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_send_bufsize")
	}
	if c.SessionSendTimeout < 0 {
		return errors.New("invalid session_send_timeout")
	}
	if c.SessionMaxPipeline < 0 {
		return errors.New("invalid session_max_pipeline")
	}
	if c.SessionKeepAlivePeriod < 0 {
		return errors.New("invalid session_keepalive_period")
	}

	return nil
}
