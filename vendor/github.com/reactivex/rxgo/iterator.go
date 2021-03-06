package rxgo

import (
	"context"
)

type Iterator interface {
	Next(ctx context.Context) (interface{}, error)
}

type iteratorFromChannel struct {
	ch         chan interface{}
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type iteratorFromRange struct {
	current int
	end     int // Included
}

type iteratorFromSlice struct {
	index int
	s     []interface{}
}

func (it *iteratorFromChannel) Next(ctx context.Context) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, &TimeoutError{}
	case <-it.ctx.Done():
		return nil, &CancelledIteratorError{}
	case next, ok := <-it.ch:
		if ok {
			return next, nil
		}
		return nil, &EndOfIteratorError{}
	}
}

func (it *iteratorFromRange) Next(ctx context.Context) (interface{}, error) {
	it.current++
	if it.current <= it.end {
		return it.current, nil
	}
	return nil, &EndOfIteratorError{}
}

func (it *iteratorFromSlice) Next(ctx context.Context) (interface{}, error) {
	it.index++
	if it.index < len(it.s) {
		return it.s[it.index], nil
	}
	return nil, &EndOfIteratorError{}
}

func newIteratorFromChannel(ch chan interface{}) Iterator {
	ctx, cancel := context.WithCancel(context.Background())
	return &iteratorFromChannel{
		ch:         ch,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

func newIteratorFromRange(start, end int) Iterator {
	return &iteratorFromRange{
		current: start,
		end:     end,
	}
}

func newIteratorFromSlice(s []interface{}) Iterator {
	return &iteratorFromSlice{
		index: -1,
		s:     s,
	}
}
