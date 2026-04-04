package awn

import "go.uber.org/zap"

// DriverOption configures a Driver.
type DriverOption func(*Driver)

// WithPTY sets the PTY backend. Useful for injecting fakes in tests.
func WithPTY(p PTYStarter) DriverOption {
	return func(d *Driver) {
		d.pty = p
	}
}

// WithPersistenceDir stores session snapshots in dir and restores them on startup.
func WithPersistenceDir(dir string) DriverOption {
	return func(d *Driver) {
		d.persistenceDir = dir
	}
}

// WithLogger sets the logger for the driver and its sessions.
func WithLogger(l *zap.Logger) DriverOption {
	return func(d *Driver) {
		d.log = l
	}
}
