package driveshaft

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func make_pair(t *testing.T, port int) (a net.Conn, b net.Conn) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	c1 := make(chan net.Conn)
	go func() {
		c, _ := l.Accept()
		c1 <- c
	}()
	fmt.Printf("Doing dial\n")
	a, err = net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	fmt.Printf("Dial done: %v %v\n", a, err)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Doing select\n")
	select {
	case b = <-c1:
		l.Close()
	case <-time.After(1 * time.Second):
		l.Close()
		b = <-c1
	}
	if a == nil || b == nil {
		panic(fmt.Errorf("Failed to make pair"))
	}
	return
}

func makeDriveshafts(t *testing.T, port int) (a *Driveshaft, b *Driveshaft) {
	as, bs := make_pair(t, port)
	a = NewDriveshaft(as, 5)
	b = NewDriveshaft(bs, 5)
	return
}

func TestBasic(t *testing.T) {
	a, b := makeDriveshafts(t, 1234)
	c := make(chan bool)
	a.SendRequest([]byte{1, 2, 3}, func(resp []byte, err error) {
		fmt.Printf("Got Response: (%v,%v)\n", resp, err)
		c <- true
	})
	req, onResp, err := b.RecvRequest()
	fmt.Printf("Got Request: (%v,%v)\n", req, err)
	onResp([]byte{4, 5, 6})
	_ = <-c
	a.Stop()
	b.Stop()
}

func TestFlowLimit(t *testing.T) {
	a, b := makeDriveshafts(t, 1235)
	c := make(chan int)
	go func() {
		for i := 0; i < 100; i++ {
			fmt.Printf("Sending request: %d\n", i)
			err := a.SendRequest([]byte{byte(i)}, func(resp []byte, err error) {
				fmt.Printf("Got Response: (%v, %v)\n", resp, err)
			})
			if err != nil {
				c <- i
				break
			}
		}
	}()
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		req, onResp, err := b.RecvRequest()
		fmt.Printf("Got Request: (%v, %v)\n", req, err)
		onResp([]byte{byte(i)})
	}
	a.Stop()
	b.Stop()
	stopAt := <-c
	fmt.Printf("Stop at: %d\n", stopAt)
	if stopAt < 5 || stopAt > 10 {
		t.Fatal("Buffering is bad")
	}
}
