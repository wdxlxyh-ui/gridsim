package protocol

import (
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

type Protocol interface {
	Name() string
	Start() error
	Stop()
	ClientConnected() bool
	ClientAddr() string
	Stats() (interrog, control, spont int64)
	Uptime() int64
	Publish(point *config.Point)
	SetStore(store *library.Store)
}
