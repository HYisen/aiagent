package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func SimpleFastFunc(v int) int {
	return v * 1000
}

func EqualAsSet(input []int, fn func(v int) int, got []int) (ok bool, explain string) {
	var want []int
	for _, v := range input {
		want = append(want, fn(v))
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		return false, fmt.Sprintf("want %v, got %v", want, got)
	}
	return true, ""
}

func IntRange(head, tail int) []int {
	var ret []int
	for i := head; i <= tail; i++ {
		ret = append(ret, i)
	}
	return ret
}

func TestRun_Happy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var counter atomic.Int32
		fn := SimpleFastFunc
		handler := func(_ context.Context, id int) (int, error) {
			time.Sleep(time.Duration(id) * time.Second)
			counter.Add(1)
			return fn(id), nil
		}
		input := IntRange(1, 8)

		got, err := Run(t.Context(), 2, handler, input)

		if err != nil {
			t.Errorf("Run failed with error: %v", err)
		}
		if ok, explain := EqualAsSet(input, fn, got); !ok {
			t.Errorf("Run failed on result: %s", explain)
		}
		if counter.Load() != int32(len(input)) {
			t.Errorf("Run executed want %d got %d", len(input), counter.Load())
		}
	})
}

func TestRun_FailFast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var startedCounter atomic.Int32
		var successCounter atomic.Int32
		var abortedCounter atomic.Int32
		fn := SimpleFastFunc
		dummyError := errors.New("this is a dummy error")
		handler := func(ctx context.Context, id int) (int, error) {
			startedCounter.Add(1)
			time.Sleep(time.Duration(id) * time.Second)
			if id == 3 {
				return 0, dummyError
			}
			if err := ctx.Err(); err != nil {
				abortedCounter.Add(1)
				return 0, err
			}
			successCounter.Add(1)
			return fn(id), nil
		}
		input := IntRange(1, 12)

		_, err := Run(t.Context(), 4, handler, input)

		if err == nil || !errors.Is(err, dummyError) {
			t.Errorf("Run error want %v got %v ", dummyError, err)
		}
		if successCounter.Load() == int32(len(input)) {
			t.Errorf("Run succeed want <%d got %d", len(input), successCounter.Load())
		}
		if abortedCounter.Load() == 0 {
			t.Error("Run aborted want >0 got 0")
		}
		if startedCounter.Load() == int32(len(input)) {
			t.Errorf("Run started want <%d got %d", len(input), startedCounter.Load())
		}
	})
}
