// Represents all data that needs to be persisted.
package model

import (
	"github.com/coopernurse/gorp"
)

/*
Records at its core is a tuple divided into three parts:

    Topic:      determines who cares.
    RecordType: allows multiple 'namespaces' within the topic.
    Key:        the rest of the key.

Multiple authors may disagree about the value, thus we allow one value per author. The value also
has a primary part (Value), along with Priority to help disambiguate.  The Signature allows
cryptographic validation of the Author if set.
*/
type Record struct {
	Topic     string // Basis for subscriptions, defines who is interested in this record.
	Type      int    // A namespacing mechanism for keys.
	Key       string // The primary key for this record within the topic
	Value     []byte // The current value of this key
	SeqNo     int
	Priority  int    // A mechanism to disambiguate multiple records with the same key
	Author    string // The Fingerprint of the Author that generated this value
	Signature []byte // The signature from the author
}

/*
RT* constants are values used by the Record.RecordType field.
*/
const (
	RTSubscribe = iota // Used by the sync layer itself to manage subscriptions
	RTBasis            // Used by the meta-data layer to manage collections
	RTWriter           // Used by the meta-data layer to manage writer
	RTData             // Used by the meta-data layer to manage meta-data
	RTAdvert           // Used by the data layer to manage storage
)

func Init(db *gorp.DbMap) {
	db.AddTableWithName(Record{}, "Record").SetKeys(false, "topic", "type", "author", "key")
}
