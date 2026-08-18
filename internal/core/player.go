package core

import (
	"context"
	"time"
)

type Player interface {
	Name() string
	Play(ctx context.Context, opts PlayOptions) error
	Stop() error
	Pause() error
	Resume() error
	Position() (time.Duration, error)
	Running() bool
}

type PlayerManager interface {
	Available() []Player
	Default() Player
	Play(ctx context.Context, opts PlayOptions) error
}
