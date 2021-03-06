package proxy

import "github.com/emphant/redis-mulive-router/pkg/utils/errors"

var (
	ErrSlotIsNotReady = errors.New("slot is not ready, may be offline")
	ErrBackendConnReset = errors.New("backend conn reset")
	ErrRequestIsBroken  = errors.New("request is broken")
)