package util

import (
	"context"
	"sync"
)

// Collector is a function type that represents a task that collects results of type T and returns an error if the task fails
type Collector[T any] func() ([]T, error)

// CollectParallel runs the given tasks in parallel and collects their results.
// If any task returns an error, it returns immediately with that error.
func CollectParallel[T any](ctx context.Context, tasks ...Collector[T]) ([]T, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	var wg sync.WaitGroup
	// mutex to make collecting atomic
	var mu sync.Mutex
	results := make([]T, 0)

	// collect the first error in task routines
	errChan := make(chan error, 1)

	// task routines
	for _, task := range tasks {
		wg.Go(func() {
			// if context is done, routine won't be executed
			select {
			case <-ctx.Done():
				return
			default:
			}

			res, err := task()
			if err != nil {
				// default case exists, won't be blocked here
				select {
				// if error can be sent, sent it
				// only the first error in routines will be sent to errChan
				case errChan <- err:
				default:
				}
				return
			}
			// no error, append results
			_ = WithMutex(&mu, func() error {
				results = append(results, res...)
				return nil
			})
		})
	}

	// assistant routine for waiting and closing channel
	// avoid main routine to be blocked and unabled to wait for the errChan
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// main routine blocked here for the errChan to be sent an error or to be closed
	if err := <-errChan; err != nil {
		// once an error occurs in task routines, return current results
		// WARN: other routines will continue until done, maybe not the best way
		return results, err
	}
	return results, nil
}

func WithMutex(mutex *sync.Mutex, fn func() error) error {
	mutex.Lock()
	defer mutex.Unlock()
	return fn()
}
