
package packetizer

import (
	"fmt"
	"net"
	"strings"
	"reflect"
	"sync"
	"h0tb0x/transfer"
)

type Packet interface {
	PacketType() uint8
}

type PacketUpper interface {
	sync.Locker
	OnConnect(uint32)
	OnDisconnect(uint32)
	PullPacket(uint32) Packet
}

type Packetizer struct {
	id       uint32
	conn     net.Conn
	upper    PacketUpper
	shutdown bool
	subtypes map[uint8] reflect.Type 
	methods  map[uint8] reflect.Value
}

func MakePacketizer(id uint32, conn net.Conn, upper PacketUpper) {
	p := &Packetizer {
		id       : id,
		conn     : conn,
		upper    : upper,
		shutdown : false,
		subtypes : make(map[uint8] reflect.Type),
		methods  : make(map[uint8] reflect.Value),
	}
	t := reflect.TypeOf(upper)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if !strings.HasPrefix(m.Name, "OnRecv") {
			continue
		}
		f := m.Type
		if f.NumIn() != 3 || f.NumOut() != 0 {
			continue
		}
		if f.In(1) != reflect.TypeOf(id) {
			continue
		}
		var pip *Packet
		ptype := f.In(2)
		if !ptype.Implements(reflect.TypeOf(pip).Elem()) {
			continue
		}
		pt := reflect.Zero(ptype).Interface().(Packet)
		fmt.Printf("Type: %d -> %s\n", pt.PacketType(), ptype)
		p.subtypes[pt.PacketType()] = ptype
		p.methods[pt.PacketType()] = m.Func
	}
	upper.Lock()
	upper.OnConnect(id)
	upper.Unlock()
	go p.readLoop()
	go p.writeLoop()
}

func (this *Packetizer) readLoop() {
	this.upper.Lock()
	for !this.shutdown {
		p := this.upper.PullPacket(this.id)
		if (p == nil) {
			break
		}
		this.upper.Unlock()
		err := transfer.Encode(this.conn, p.PacketType())
		if err == nil {
			err = transfer.Encode(this.conn, p)
		}
		this.upper.Lock()
		if (err != nil) {
			break
		}
	}
	if !this.shutdown {
		this.conn.Close()
		this.shutdown = true
		this.upper.OnDisconnect(this.id)
	}
	this.upper.Unlock()
}

func (this *Packetizer) writeLoop() {
	this.upper.Lock()
	for !this.shutdown {
		this.upper.Unlock()
		var t uint8
		var pt reflect.Value
		fmt.Printf("Doing read\n")
		err := transfer.Decode(this.conn, &t)
		if err == nil {
			ptype, ok := this.subtypes[t]
			if !ok {
				err = fmt.Errorf("Unknown packet type: %d", t)
			} else {
				pt = reflect.New(ptype.Elem())
				fmt.Printf("Doing packet read\n")
				err = transfer.Decode(this.conn, pt.Interface())
			}
		}
		this.upper.Lock()
		if err != nil {
			fmt.Printf("Err: %s\n", err)
			break
		}
		this.methods[t].Call([]reflect.Value{ reflect.ValueOf(this.upper), reflect.ValueOf(this.id), pt})
	}
	if !this.shutdown {
		this.conn.Close()
		this.shutdown = true
		this.upper.OnDisconnect(this.id)
	}
	this.upper.Unlock()
}


