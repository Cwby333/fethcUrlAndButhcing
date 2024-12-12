package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func fetchURL(ctx context.Context, urlChannel <- chan string, codeChannel chan <- int) error {
	for {
		select {
		case <- ctx.Done():
			return nil
		default:
		}

		select {
		case <- ctx.Done():
			return nil
		case url, ok := <- urlChannel:
			if !ok {
				return nil
			}

			response, err := http.Get(url)

			if err != nil {
				return err
			}

			select {
			case <- ctx.Done():
				return nil
			case codeChannel <- response.StatusCode:
			}
		}
	}
}

func batch(ctx context.Context, codeChannel <- chan int, batchSize int, batchTimeout time.Duration) chan []int {
	batch := make([]int, 0, batchSize)
	out := make(chan []int)
	wg := &sync.WaitGroup{}
	ticker := time.NewTicker(batchTimeout)

	wg.Add(1)
	go func ()  {
		defer wg.Done()

		for {
			select {
			case <- ctx.Done():
				return
			default:
			}
		
			select {
			case <- ctx.Done():
				return
			case code, ok := <- codeChannel:
				if !ok {
					return
				}

				if len(batch) < batchSize {
					batch = append(batch, code)
				}else {

					select {
					case <- ctx.Done():
						return
					case out <- batch:
						batch = batch[:0]
					case <- ticker.C:
						select {
						case <- ctx.Done():
							return
						case out <- batch:
							batch = batch[:0]
						}
					}

				}
			case <- ticker.C:
				select {
				case <- ctx.Done():
					return
				case out <- batch:
					batch = batch[:0]
				}
			}
		}
	}()

	go func ()  {
		wg.Wait()

		if len(batch) != 0 {
			out <- batch
			batch = batch[:0]
		}

		close(out)
	}()

	return out
}

func fanOutButch(ctx context.Context, codeChannel <- chan int, batchSize int, batchTimeout time.Duration, countWorkers int) []chan []int {
	out := make([]chan []int, 0, countWorkers)

	for range countWorkers {
		out = append(out, batch(ctx, codeChannel, batchSize, batchTimeout))
	}

	return out
}

func fanInBatch(ctx context.Context, channelsButch []chan []int, counter *int, mu *sync.Mutex, resultChannel chan []int) {
	for {
		if *counter == len(channelsButch) {
			return
		}

		select {
		case <- ctx.Done():
			return
		default:
		}

		mu.Lock()
		idx := *counter
		*counter++
		mu.Unlock()

	loop:
		for {
			select {
			case <- ctx.Done():
				return
			default:
			}

			select {
			case <- ctx.Done():
				return
			case batch, ok := <- channelsButch[idx]:
				if !ok {
					break loop
				}

				select {
				case <- ctx.Done():
					return
				case resultChannel <- batch:
				}
			}
		}
	}
}

func main() {
	ctx, _ := context.WithTimeout(context.Background(), time.Second * 10)
	urls := make(chan string)
	codeChan := make(chan int)
	g, gctx := errgroup.WithContext(ctx)
	builder := &strings.Builder{}
	resultChannel := make(chan []int)
	counter := 0
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	reset(builder)

	go func ()  {
		for i := range 100 {
			builder.WriteString("/")
			builder.WriteString(strconv.Itoa(i))

			urls <- builder.String()

			reset(builder)
		}
	
		close(urls)	
	}()

	go func ()  {
		for range 5 {
			g.Go(func() error {
				return fetchURL(gctx, urls, codeChan)
			})
		}

		if err := g.Wait(); err != nil {
			log.Fatal(err)
		}

		close(codeChan)
	}()

	slice := fanOutButch(ctx, codeChan, 2, time.Second * 2, 5)

	for range 5 {
		wg.Add(1)
		go func ()  {
			defer wg.Done()
			
			fanInBatch(ctx, slice, &counter, mu, resultChannel)
		}()
	}

	go func ()  {
		wg.Wait()
		close(resultChannel)
	}()

	for b := range resultChannel {
		fmt.Println(b)
	}
}

func reset(b *strings.Builder) {
	b.Reset()
	b.WriteString("https://jsonplaceholder.typicode.com/posts")
}