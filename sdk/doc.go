// Package sdk provides a Go client library for the awn TUI automation daemon.
//
// Connect to the daemon, create terminal sessions, capture screenshots,
// send input, detect UI elements, and more — all with typed methods.
//
//	c, err := sdk.Connect()
//	if err != nil { log.Fatal(err) }
//	defer c.Disconnect()
//
//	s, err := c.Create(ctx, "bash")
//	screen, err := c.Exec(ctx, s.ID, "ls -la", sdk.WaitStable())
//	fmt.Println(screen.Lines)
//	c.Close(ctx, s.ID)
package sdk
