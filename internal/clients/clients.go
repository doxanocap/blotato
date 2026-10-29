package clients

import (
	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/clients/fake"
	"github.com/doxanocap/blotato/internal/clients/instagram"
	"github.com/doxanocap/blotato/internal/pkg/config"
)

type Clients struct {
	instagram *instagram.Client
	fake      *fake.Client
}

func New(cfg config.Clients) (Clients, error) {
	if cfg.Mode == "fake" {
		sender := fake.New()

		return Clients{fake: sender}, nil
	}

	if cfg.Mode != "instagram" {
		return Clients{}, errtrace.New("unsupported client mode")
	}

	client, err := instagram.New(cfg.Instagram)
	if err != nil {
		return Clients{}, errtrace.Wrap(err)
	}

	return Clients{
		instagram: client,
	}, nil
}

func (c Clients) Instagram() *instagram.Client { return c.instagram }

func (c Clients) Fake() *fake.Client { return c.fake }
