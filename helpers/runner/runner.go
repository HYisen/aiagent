package runner

import "context"

func Run[InputType any, OutputType any](
	ctx context.Context,
	nThreads int,
	handler func(ctx context.Context, input InputType) (OutputType, error),
	input []InputType,
) ([]OutputType, error) {
	ctx, cancel := context.WithCancel(ctx)
	in := make(chan InputType)
	out := make(chan result[OutputType])

	for range nThreads {
		go process(ctx, in, out, handler)
	}

	go feed(in, input)

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
	for {
		select {
		case <-ctx.Done():
			return
		case one := <-inputCh:
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
}

func collect[OutputType any](ch <-chan result[OutputType], cancel context.CancelFunc) ([]OutputType, error) {
	defer func() {
		cancel()
	}()
	var ret []OutputType
	for two := range ch {
		if two.Error == nil {
			ret = append(ret, two.Output)
		} else {
			return nil, two.Error
		}
	}
	return ret, nil
}
