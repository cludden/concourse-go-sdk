package inmem

import (
	"context"
)

type Config struct {
	History []string `json:"history"`
}

type Archive struct {
	history [][]byte
}

func New(ctx context.Context, cfg Config) (*Archive, error) {
	history := make([][]byte, len(cfg.History))
	for i, raw := range cfg.History {
		history[i] = []byte(raw)
	}
	return &Archive{history: history}, nil
}

func (a *Archive) Close(context.Context) error {
	return nil
}

func (a *Archive) History(context.Context) ([][]byte, error) {
	return a.history, nil
}

func (a *Archive) Put(ctx context.Context, versions ...[]byte) error {
	a.history = append(a.history, versions...)
	return nil
}
