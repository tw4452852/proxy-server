package main

import (
	"net"
	"time"
)

var readTimeout time.Duration

func setReadTimeout(c net.Conn) {
	if readTimeout != 0 {
		c.SetReadDeadline(time.Now().Add(readTimeout))
	}
}

// PipeThenClose copies data from src to dst, closes dst when done.
func PipeThenClose(src, dst net.Conn) {
	var (
		rerr, werr error
		n          int
	)
	defer func() {
		if rerr != nil {
			Debug.Printf("[pipe]: read error: %s\n", rerr)
		}
		if werr != nil {
			Debug.Printf("[pipe]: write error: %s\n", werr)
		}

		dst.Close()
	}()

	buf := make([]byte, 4096)
	for {
		setReadTimeout(src)
		n, rerr = src.Read(buf)
		// read may return EOF with n > 0
		// should always process n > 0 bytes before handling error
		if n > 0 {
			// Note: avoid overwrite err returned by Read.
			if _, werr = dst.Write(buf[0:n]); werr != nil {
				return
			}
		}
		if rerr != nil {
			// Always "use of closed network connection", but no easy way to
			// identify this specific error. So just leave the error along for now.
			// More info here: https://code.google.com/p/go/issues/detail?id=4373
			break
		}
	}
}
