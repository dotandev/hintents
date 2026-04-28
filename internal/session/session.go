package session

import (
	"encoding/json"
	"fmt"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/p2p"
)

type Session struct {
	State   *StateStore
	DB      *Store
	P2PHost *p2p.Host
}

func NewSession() (*Session, error) {
	db, err := NewStore()
	if err != nil {
		return nil, err
	}

	return &Session{
		State: NewStateStore(),
		DB:    db,
	}, nil
}

func (s *Session) StartHosting(port int) ([]string, error) {
	h, err := p2p.NewHost(port)
	if err != nil {
		return nil, err
	}

	s.P2PHost = h

	s.State.Use(func(next Dispatcher) Dispatcher {
		return func(action Action) {
			next(action)

			b, err := json.Marshal(action)
			if err != nil {
				logger.Logger.Error("Failed to marshal action", "err", err)
				return
			}

			if err := s.P2PHost.Broadcast(b); err != nil {
				logger.Logger.Error("Failed to broadcast state update", "err", err)
			}
		}
	})

	return h.FullAddrs(), nil
}

func (s *Session) Join(peerAddr string) error {
	h, err := p2p.NewHost(0)
	if err != nil {
		return err
	}

	s.P2PHost = h

	if err := h.Connect(peerAddr); err != nil {
		return err
	}

	ch, err := h.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		for data := range ch {
			var action Action
			if err := json.Unmarshal(data, &action); err != nil {
				logger.Logger.Error("Failed to parse incoming action", "err", err)
				continue
			}
			s.State.Dispatch(action)
		}
	}()

	return nil
}

func (s *Session) Close() error {
	var err error
	if s.P2PHost != nil {
		err = s.P2PHost.Close()
	}
	if dbErr := s.DB.Close(); dbErr != nil && err == nil {
		err = dbErr
	}
	return err
}
