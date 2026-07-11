package main

// Config for write buffer length
type BufferConfig struct{
	Size int // bytes
 }

// DefaultBuffer set to 16KB (16 * 1024 bytes)
// standard for optimzed sequential writes
func DefaultBuffer() BufferConfig {
	return BufferConfig{
		Size: 16 * 1024,
	}
}