package route

import (
	"fmt"
	"h0tb0x/crypto"
	"h0tb0x/transfer"
	"io"
)

// A route through the network.  Cryptographically encoded so a node can only see the entries it added.
type Route struct {
	impl []*crypto.TinyMessage
}

// Implements the h0tb0x transfer protocol
func (this *Route) Encode(stream io.Writer) error {
	return transfer.Encode(stream, this.impl)
}

// Implements the h0tb0x transfer protocol
func (this *Route) Decode(stream io.Reader) error {
	return transfer.Decode(stream, &this.impl)
}

func (this *Route) Length() int {
	return len(this.impl)
}

// Pushes a new entry on the end of the route
func (this *Route) Push(key *crypto.SymmetricKey, next uint32) {
	// Encode the message and add to end of array
	this.impl = append(this.impl, key.EncodeMessage(next))
}

// Removes the top entry, returns err if empty or not able to decode
func (this *Route) Pop(key *crypto.SymmetricKey) (out uint32, err error) {
	// Check for empty route
	if len(this.impl) == 0 {
		err = fmt.Errorf("Trying to get top element from empty route")
		return
	}
	// Decode the 'top', ie, last, message
	out, valid := key.DecodeMessage(this.impl[len(this.impl)-1])
	if !valid {
		// If it's not from me, error
		err = fmt.Errorf("Invalid entry on top, couldn't decode")
		return
	}
	// Slice to remove top entry
	this.impl = this.impl[:len(this.impl)-1]
	// Return out (already set), and no error
	return
}

// Check for entries made by me for routes that must be loop free such as advert
func (this *Route) HasSelf(key *crypto.SymmetricKey) bool {
	for _, entry := range this.impl {
		// Try to decode with my key
		_, valid := key.DecodeMessage(entry)
		if valid {
			// If that works, I made the entry, return true
			return true
		}
	}
	// Looks like no entries from me on the list
	return false
}

// Optimize a route (remove self loops).  Specically, find the oldest entry made by me, and remove that entry and all following entries.
func (this *Route) Optimize(key *crypto.SymmetricKey) {
	// Loop over all entries, starting with the 'oldest'
	for i, entry := range this.impl {
		// Try to decode
		_, valid := key.DecodeMessage(entry)
		if valid {
			// If I found an entry by me, truncate route and return
			this.impl = this.impl[:i]
			return
		}
	}
	// Leave things as they are
}
