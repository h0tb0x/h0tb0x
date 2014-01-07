
package packetizer

import (
	"testing"
	"sync"
	"net"
	"fmt"
	"time"
)

type intPacket struct {
	Value int 
}

type stringPacket struct {
	Value string
}

func (this *intPacket) PacketType() uint8 {
	return 0
}

func (this *stringPacket) PacketType() uint8 {
	return 1 
}

type testUpper struct {
	sync.Mutex 
	name  string
	queue []Packet
}

func (this *testUpper) OnConnect(id uint32) {
	fmt.Printf("%s: OnConnect %d\n", this.name, id)	
}

func (this *testUpper) OnDisconnect(id uint32) {
	fmt.Printf("%s: OnDisconnect %d\n", this.name, id)	
}

func (this *testUpper) OnRecvInt(src uint32, pkt *intPacket) {
	fmt.Printf("%s: OnRecvInt, %d -> %d\n",this.name, src, pkt.Value)
}

func (this *testUpper) OnRecvString(src uint32, pkt *stringPacket) {
	fmt.Printf("%s: OnRecvString, %d -> %s\n", this.name, src, pkt.Value)
}

func (this *testUpper) PullPacket(id uint32) Packet {
	fmt.Printf("%s: PullPacket, %d\n", this.name, id)
	var r Packet
	if len(this.queue) > 0 {
		r = this.queue[0]
		this.queue = this.queue[1:]
	}
	if (r == nil) {
		fmt.Printf("Doing some sleeping\n")
		this.Unlock()
		time.Sleep(1 * time.Second)
		this.Lock()
	}
	return r
}

func make_pair(port int) (a net.Conn, b net.Conn) {
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


func TestBasic(t *testing.T) {
	upper1 := &testUpper{ name : "A", queue : []Packet{ &intPacket{Value: 5}, &stringPacket{Value: "Hello"} } }
	upper2 := &testUpper{ name : "B"}
	c1, c2 := make_pair(5000)
	MakePacketizer(0, c1, upper1)
	MakePacketizer(0, c2, upper2)
	time.Sleep(5 * time.Second)
}

