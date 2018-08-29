package cache

import (
	"log"
	"io"
	"runtime"
	"bytes"
	"sync/atomic"
)

type ICache interface {

	Reset()

	Bytes() []byte

	ReadFrom(r io.Reader) (n int64, err error)

	Write(p []byte) (n int, err error)

	Get() *bytes.Buffer

	Incement()

	Decrement()
}

type IBuffer interface {
	New() ICache

	Wrap(data []byte) ICache
}

type chached struct {
	bytes.Buffer
	count int32
	cache *buffer
}

func (b *chached) Get() *bytes.Buffer {
	return &b.Buffer
}

func (b *chached) Incement() {
	atomic.AddInt32(&b.count, 1)
}

func (b *chached) Decrement() {
	if atomic.AddInt32(&b.count, -1) == 0 {
		b.cache.push(b)
	}
}

type sDirect struct {
	buf    *bytes.Buffer
	count int32
	cache  *buffer
}

func (b *sDirect) Reset() {
	b.buf.Reset()
}

func (b *sDirect) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *sDirect) ReadFrom(r io.Reader) (n int64, err error) {
	return b.buf.ReadFrom(r)
}

func (b *sDirect) Write(p []byte) (n int, err error) {
	return b.buf.Write(p)
}

func (b *sDirect) Get() *bytes.Buffer {
	return b.buf
}

func (b *sDirect) Incement() {
	atomic.AddInt32(&b.count, 1)
}

func (b *sDirect) Decrement() {
	if atomic.AddInt32(&b.count, -1) == 0 {
		b.cache.push(b)
	}
}

type buffer struct {
	buffers []chan ICache
	size int
	num int32
	read int32
	write int32
}

func NewCache(count int, size int) IBuffer {

	result := &buffer{size: size}

	result.num = int32(runtime.NumCPU())

	result.buffers = make([]chan ICache, result.num, result.num)

	for i := int32(0); i < result.num; i++ {

		result.buffers[i] = make(chan ICache, count/runtime.NumCPU())
	}

	result.read = 0

	result.write = result.num / 2

	return result
}

func (cache *buffer) push(buffer ICache) {

	if buffer, ok := buffer.(*sDirect); ok {

		buffer.Reset()

		return
	}

	buffer.Reset()

	_ = atomic.AddInt32(&cache.write, 1) % cache.num
}

func (cache *buffer) New() ICache {

	var buffer ICache

	pos := atomic.AddInt32(&cache.read, 1) % cache.num

	select {
	case buffer = <-cache.buffers[pos]:

		buffer.Incement()

		break
	default:

		buffer = &chached{count: 1, cache: cache}

		buffer.Get().Grow(cache.size)

		break
	}

	return buffer
}

func (cache *buffer) Wrap(data []byte) ICache {

	return &sDirect{count: 1, cache: cache, buf: bytes.NewBuffer(data)}
}

func ReadAll(dest ICache, r io.Reader) error {

	var err error

	defer func() {
		e := recover()

		if e == nil {
			return
		}

		log.Println(e)

	}()

	_, err = dest.ReadFrom(r)

	if err != nil {
		log.Println(err)
	}

	return err
}