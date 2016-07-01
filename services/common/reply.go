package common

import "time"

type Reply struct {
	Name		string			`json:"name"`

	Bucket		string			`json:"bucket,omitempty"`
	Key		string			`json:"key,omitempty"`
	Size		uint64			`json:"size,omitempty"`

	MetaSize	uint64			`json:"meta_size,omitempty"`
	MetaBucket	string			`json:"meta_bucket,omitempty"`
	MetaKey		string			`json:"meta_key,omitempty"`

	ContentType	string			`json:"content_type,omitempty"`

	Timestamp	time.Time		`json:"timestamp,omitempty"`
}

