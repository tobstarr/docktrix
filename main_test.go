package main

import (
	"net/http"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	c := &Server{LogToBuffer: true, Cmd: "node", Args: []string{"fixtures/node.js"}}
	if err := c.Run(); err != nil {
		t.Error("error starting rackup", err)
	}
	defer c.Close()

	http.DefaultClient.Transport = &http.Transport{DisableKeepAlives: true}

	started := time.Now()
	for i := 0; i < 100; i++ {
		ok := func() bool {
			rsp, err := http.Get("http://127.0.0.1:1234")
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				return false
			}
			defer rsp.Body.Close()
			return true
		}()
		if ok {
			break
		}
	}
	logger.Printf("finished %.06f", time.Since(started).Seconds())

	for i := 0; i < 2; i++ {
		func() {
			rsp, err := http.Get("http://127.0.0.1:1234")
			if err != nil {
				t.Fatal("error doing http request", err)
			}
			defer rsp.Body.Close()

			if rsp.StatusCode != 200 {
				t.Errorf("expected status 200, got %d", rsp.StatusCode)
			}
		}()
	}

	//var ex interface{} = "request 1\nrequest 2\nrequest 3\n"
	//out := c.out.String()
	//if ex != out {
	//	t.Errorf("expected out to be %q, was %q", ex, out)
	//}
}
