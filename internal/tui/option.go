package tui

import (
	"github.com/ayinke-llc/sdump/config"
)

type Option func(*model)

func WithColorscheme(colorscheme string) Option {
	return func(m *model) {
		m.colorscheme = colorscheme
	}
}

func WithConfig(cfg *config.Config) Option {
	return func(m *model) {
		m.cfg = cfg
	}
}

func WithHeight(height int) Option {
	return func(m *model) {
		m.height = height
	}
}

func WithWidth(width int) Option {
	return func(m *model) {
		m.width = width
	}
}

func WithSSHFingerPrint(fingerPrint string) Option {
	return func(m *model) {
		m.sshFingerPrint = fingerPrint
	}
}

func WithHTTPForwarding(host string, portToForwardTo int) Option {
	return func(m *model) {
		m.isHTTPForwardingEnabled = true
		m.portToForwardTo = portToForwardTo
		m.host = host
	}
}
