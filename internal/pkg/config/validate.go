package config

import (
	"net"
	"net/url"

	"braces.dev/errtrace"
)

func (a App) Validate() error {
	if a.Mode != "fake" && a.Mode != "instagram" {
		return errtrace.New("BLOTATO_MODE must be fake or instagram")
	}
	if a.Listen == "" {
		return errtrace.New("HTTP listener is required")
	}
	return nil
}

func (c Instagram) Validate() error {
	if c.AccountID == "" || c.Token == "" || c.Host == "" || c.RequestTimeout <= 0 {
		return errtrace.New("Instagram account, credentials, API base and positive timeout are required")
	}
	base, err := url.Parse(c.Host)
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return errtrace.New("invalid Instagram API base")
	}
	ip := net.ParseIP(base.Hostname())
	loopback := base.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	if base.Scheme != "https" && !(base.Scheme == "http" && loopback) {
		return errtrace.New("Instagram API base must use HTTPS or loopback HTTP")
	}
	return nil
}
