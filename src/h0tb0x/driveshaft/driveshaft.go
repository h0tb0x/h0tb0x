package driveshaft

import (
	"fmt"
	"h0tb0x/transfer"
	"io"
	"log"
	"sync"
)

const (
	mt_request  = 0
	mt_response = 1
	mt_window   = 2
)

type framedMessage struct {
	Type  int    // 0 = Request, 1 = Response, 2 = Window
	Value uint64 // Request# or window delta
	Buf   []byte // Actual data buffer
}

type mesg struct {
	reqNum uint64
	data   []byte
}

// The driveshaft object
type Driveshaft struct {
	conn          io.ReadWriteCloser             // The underlying connection
	log           *log.Logger                    // The log
	shutdown      bool                           // Are we closing?
	mutex         sync.Mutex                     // A mutex that locks all access
	notifyWriter  sync.Cond                      // Notifies the writer thread to check it's state
	notifySender  sync.Cond                      // Notifies the 'SendRequest' function to check it's state
	notifyRecv    sync.Cond                      // Notifies the 'RecvRequest' function to check it's state
	notifyStop    sync.Cond                      // Notifies the 'Stop' function that it's done
	threads       sync.WaitGroup                 // Wait group for all my threads
	recvWinDelta  int                            // The current unsent increments to the recv window
	sendWin       int                            // How many requests can we send before we run out of window space
	requestQueue  []*mesg                        // The queue of unprocessed incoming requests
	responseQueue []*mesg                        // The outbound responses
	nextReqNum    uint64                         // The request number to assign the next outbound request
	outRequest    []byte                         // The single outbound request
	respFuncs     map[uint64]func([]byte, error) // The list of callbacks to send incoming responses to
}

// Construct a new driveshaft that wraps a connection
func NewDriveshaft(conn io.ReadWriteCloser, recvBuf int) *Driveshaft {
	ds := &Driveshaft{
		conn:          conn,
		recvWinDelta:  recvBuf,
		sendWin:       0,
		requestQueue:  []*mesg{},
		responseQueue: []*mesg{},
		respFuncs:     make(map[uint64]func([]byte, error)),
	}
	ds.notifyWriter.L = &ds.mutex
	ds.notifySender.L = &ds.mutex
	ds.notifyRecv.L = &ds.mutex
	ds.notifyStop.L = &ds.mutex
	go ds.masterLoop()
	return ds
}

// Stop a driveshaft object, causing any pending requests to error out
// and possible not delivering some responses.
// TODO: Make a nicer shutdown
func (this *Driveshaft) Stop() {
	this.mutex.Lock()
	this.doShutdown()
	this.notifyStop.Wait()
	this.mutex.Unlock()
}

// Send a request, when the response comes back (or network error or stop happens), call onResp
// If already shutdown, error is returned immediately instead
func (this *Driveshaft) SendRequest(req []byte, onResp func([]byte, error)) error {
	this.mutex.Lock()
	// Wait until shutdown or room to 'send'
	for !this.shutdown && this.outRequest != nil {
		this.notifySender.Wait()
	}
	if this.shutdown {
		// On shutdown, return in error
		this.mutex.Unlock()
		return fmt.Errorf("Driveshaft shutdown in send")
	}
	// Otherwise, put request in outRequest, add resp to map, and return
	this.outRequest = req
	this.respFuncs[this.nextReqNum] = onResp
	this.notifyWriter.Signal()
	this.mutex.Unlock()
	return nil
}

func (this *Driveshaft) RecvRequest() (req []byte, onResp func([]byte), err error) {
	this.mutex.Lock()
	// Wait until shutdown or something to receive
	for !this.shutdown && len(this.requestQueue) == 0 {
		this.notifyRecv.Wait()
	}
	if this.shutdown {
		// On shutdown, return in error
		this.mutex.Unlock()
		err = fmt.Errorf("Driveshaft shutdown in recv")
		return
	}
	// Pull the top request
	reqMesg := this.requestQueue[0]
	this.requestQueue = this.requestQueue[1:]
	// Make a new response closure
	onResp = func(resp []byte) {
		respMesg := &mesg{reqNum: reqMesg.reqNum, data: resp}
		this.mutex.Lock() // This is run later, need to lock
		if this.shutdown {
			return
		} // Exit early if aleady closing
		// Append message
		this.responseQueue = append(this.responseQueue, respMesg)
		// Notify writer
		this.notifyWriter.Signal()
		this.mutex.Unlock()
	}
	// Move delta forward
	this.recvWinDelta = this.recvWinDelta + 1
	// Notify writer (who might want to send new delta)
	this.notifyWriter.Signal()
	this.mutex.Unlock()
	req = reqMesg.data
	return
}

func (this *Driveshaft) masterLoop() {
	this.threads.Add(2)
	go this.readLoop()
	go this.writeLoop()
	this.threads.Wait()
	for _, f := range this.respFuncs {
		f(nil, fmt.Errorf("Driveshaft shutdown before response"))
	}
	this.notifyStop.Broadcast()
}

func (this *Driveshaft) writeLoop() {
	this.mutex.Lock()
	for !this.shutdown {
		var outMesg framedMessage
		if this.recvWinDelta > 0 { // First see if we have new window data
			// If so, make output packet, and clear diff
			outMesg.Type = mt_window
			outMesg.Value = uint64(this.recvWinDelta)
			this.recvWinDelta = 0
		} else if len(this.responseQueue) > 0 { // Next look for responses to send
			outMesg.Type = mt_response
			outMesg.Value = this.responseQueue[0].reqNum
			outMesg.Buf = this.responseQueue[0].data
			this.responseQueue = this.responseQueue[1:]
		} else if this.sendWin > 0 && this.outRequest != nil { // Next send requests
			this.sendWin = this.sendWin - 1
			outMesg.Type = mt_request
			outMesg.Value = this.nextReqNum
			this.nextReqNum = this.nextReqNum + 1
			outMesg.Buf = this.outRequest
			this.outRequest = nil
			this.notifySender.Signal()
		} else {
			// Nothing to do, wait
			this.notifyWriter.Wait()
			continue
		}
		this.mutex.Unlock()
		//fmt.Printf("writeLoop: sending message: %v, %v, %v\n", outMesg.Type, outMesg.Value, outMesg.Buf)
		err := transfer.Encode(this.conn, outMesg)
		this.mutex.Lock()
		if err != nil {
			this.doShutdown()
			break
		}
	}
	this.mutex.Unlock()
	this.threads.Done()
}

func (this *Driveshaft) readLoop() {
	this.mutex.Lock()
	for {
		var inMesg framedMessage
		this.mutex.Unlock()
		err := transfer.Decode(this.conn, &inMesg)
		this.mutex.Lock()
		if err != nil {
			this.doShutdown()
			break
		}
		//fmt.Printf("readlLoop: got message: %v, %v, %v\n", inMesg.Type, inMesg.Value, inMesg.Buf)
		switch inMesg.Type {
		case mt_window:
			this.sendWin = this.sendWin + int(inMesg.Value)
			this.notifyWriter.Signal()
		case mt_response:
			f, exists := this.respFuncs[inMesg.Value]
			if exists {
				go f(inMesg.Buf, nil) // Should I really 'go' this?
				delete(this.respFuncs, inMesg.Value)
			}
		case mt_request:
			m := &mesg{reqNum: inMesg.Value, data: inMesg.Buf}
			this.requestQueue = append(this.requestQueue, m)
			this.notifyRecv.Signal()
		}
	}
	this.mutex.Unlock()
	this.threads.Done()
}

func (this *Driveshaft) doShutdown() { // Called with lock!
	if this.shutdown {
		return
	}
	this.shutdown = true
	this.conn.Close()
	this.notifyWriter.Broadcast()
	this.notifySender.Broadcast()
	this.notifyRecv.Broadcast()
}
