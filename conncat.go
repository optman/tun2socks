package main

import (
	"io"
	"sync"
)

type closeWriter interface {
	CloseWrite() error
}

func ConcatStream(src, dst io.ReadWriter) {
	var wg sync.WaitGroup

	cp := func(dst, src io.ReadWriter) {
		defer wg.Done()

		io.Copy(dst, src)
		if c, ok := dst.(closeWriter); ok {
			c.CloseWrite()
		}
	}

	wg.Add(2)

	go cp(dst, src)
	go cp(src, dst)

	wg.Wait()
}
