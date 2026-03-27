package awn

// DriverOption configures a Driver.
type DriverOption func(*Driver)

// WithPTY sets the PTY backend. Useful for injecting fakes in tests.
func WithPTY(p PTYStarter) DriverOption {
	return func(d *Driver) {
		d.pty = p
	}
}
