package runner

import (
	"context"
	"log/slog"
	"sync"
)

// Run do handler on input at most nThreads.
// nThreads is internally limited by len(input),
// because you can't produce a baby in one month by getting nine women pregnant.
// Kindly don't use typed error nil in handler return,
// because (*TypedError)(nil) != nil and you will suffer.
func Run[InputType any, OutputType any](
	ctx context.Context,
	nThreads int,
	handler func(ctx context.Context, input InputType) (OutputType, error),
	input []InputType,
) ([]OutputType, error) {
	if len(input) == 0 {
		return nil, nil
	}
	nThreads = min(nThreads, len(input))

	ctx, cancel := context.WithCancel(ctx)
	in := make(chan InputType)
	out := make(chan result[OutputType])

	var wg sync.WaitGroup
	for range nThreads {
		wg.Go(func() {
			process(ctx, in, out, handler)
		})
	}
	go watch(&wg, out)

	go feed(in, input)

	// Happy Path: feed close in, process done through in close, wg notify watch to close out, collect finish.
	// Exception Path: feed don't stop, process send an error, collect catch the error and return,
	// return defer cancel ctx, all process catch it and start draining (without which feed leak),
	// Pending item from process to collect would be drained in collect's defer. (without which process leak)
	// Then happy path, except when watch close out, collect don't care as has failed fast.
	return collect(out, cancel)
}

type result[OutputType any] struct {
	Output OutputType
	Error  error
}

func process[InputType any, OutputType any](
	ctx context.Context,
	inputCh <-chan InputType,
	outputCh chan<- result[OutputType],
	handler func(ctx context.Context, input InputType) (OutputType, error),
) {
	var draining bool
	for {
		select {
		case <-ctx.Done():
			draining = true
			return
		case one, ok := <-inputCh:
			if !ok {
				return
			}
			if draining {
				slog.Warn("process drain in", "item", one)
				continue
			}
			two, err := handler(ctx, one)
			outputCh <- result[OutputType]{
				Output: two,
				Error:  err,
			}
		}
	}
}

func feed[InputType any](ch chan<- InputType, items []InputType) {
	for _, one := range items {
		ch <- one
	}
	close(ch)
}

func collect[OutputType any](ch <-chan result[OutputType], cancel context.CancelFunc) ([]OutputType, error) {
	defer func() {
		cancel()
		for two := range ch {
			slog.Warn("collect drain out", "item", two)
		}
	}()
	var ret []OutputType
	for two := range ch {
		if two.Error != nil {
			return nil, two.Error
		}
		ret = append(ret, two.Output)
	}
	return ret, nil
}

func watch[ItemType any](wg *sync.WaitGroup, closer chan ItemType) {
	wg.Wait()
	close(closer)
}
