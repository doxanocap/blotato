// Package instagram implements outgoing Graph API operations for Unified Login.
package instagram

import (
	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/pkg/config"
	"github.com/doxanocap/blotato/internal/pkg/httpc"
)

type Client struct {
	httpClient *httpc.Client
	cfg        config.Instagram
}

// New configures the transport once, including the timeout and credentials.
func New(cfg config.Instagram, opts ...httpc.Option) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errtrace.Wrap(err)
	}

	opts = append([]httpc.Option{
		httpc.WithTimeout(cfg.RequestTimeout),
		httpc.WithHeader("Authorization", "Bearer "+cfg.Token),
	}, opts...)

	transport, err := httpc.New(cfg.Host, opts...)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return &Client{
		httpClient: transport,
		cfg:        cfg,
	}, nil
}
