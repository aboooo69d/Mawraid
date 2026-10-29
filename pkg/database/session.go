package database

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/najm-x/nova/config"
)

func NewSessionStore(cfg *config.SessionConfg) (sessions.Store, error) {
	store, err := redis.NewStore(cfg.Size,
		cfg.Network,
		cfg.Address,
		cfg.Username,
		cfg.Password,
		cfg.KeyPairs)
	if err != nil {
		return nil, err
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   false,
		SameSite: 3,
	})

	return store, nil

}
