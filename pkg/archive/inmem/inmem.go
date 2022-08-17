package inmem

import (
	"context"

	"github.com/cludden/concourse-go-sdk/pkg/archive/settings"
)

// Config defines backend specific configuration
type Config struct {
	History []string `json:"history"`
}

// Archive implements an in-mmeory archive backend, that provides no useful utility
// beyond testing archive behavior. DO NOT USE in production.
type Archive struct {
	history [][]byte
}

func New(ctx context.Context, cfg Config, s *settings.Settings) (*Archive, error) {
	history := make([][]byte, len(cfg.History))
	for i, raw := range cfg.History {
		history[i] = []byte(raw)
	}
	return &Archive{history: history}, nil
}

func (a *Archive) Close(context.Context) error {
	return nil
}

func (a *Archive) History(context.Context, []byte) ([][]byte, error) {
	return a.history, nil
}

func (a *Archive) Put(ctx context.Context, versions ...[]byte) error {
	a.history = append(a.history, versions...)
	return nil
}
