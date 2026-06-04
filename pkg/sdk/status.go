package sdk

// Initialized returns true once the SDK has a known state. It is true
// immediately in offline mode (the client is in a known offline state).
// In online mode it becomes true after the first successful snapshot
// fetch (or bootstrap).
func (c *Client) Initialized() bool {
	if c.opts.offline {
		return true
	}
	return !c.cache.get().fetchedAt.IsZero()
}
