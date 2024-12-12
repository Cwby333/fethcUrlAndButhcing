package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func main() {
	ctx, _ := context.WithTimeout(context.Background(), time.Second * 10)
	urls := make(chan string)
	codeChan := make(chan int)
	g, gctx := errgroup.WithContext(ctx)
	builder := &strings.Builder{}
	reset(builder)

	go func ()  {
		for i := range 30 {
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

		g.Wait()
		close(codeChan)
	}()

	for code := range codeChan {
		fmt.Println(code)
	}
}

func reset(b *strings.Builder) {
	b.Reset()
	b.WriteString("https://jsonplaceholder.typicode.com/posts")
}