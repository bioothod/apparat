package nullx

import (
	"encoding/json"
	"time"
)

type Info struct {
	ID			string			`json:"id"`
	Checksum		string			`json:"csum"`
	Filename		string			`json:"filename"`
	Group			uint32			`json:"group"`
	Backend			int			`json:"backend"`
	Size			uint64			`json:"size"`
	Offset			uint64			`json:"offset-within-data-file"`
	Mtime			time.Time
	Server			string			`json:"server"`
}

func (info *Info) UnmarshalJSON(data []byte) (err error) {
	type Mtime struct {
		Tsec		int64		`json:"tsec"`
		Tnsec		int64		`json:"tnsec"`
	}

	type Alias Info
	tmp := &struct {
		Mtime Mtime		`json:"mtime"`
		*Alias
	} {
		Alias: (*Alias)(info),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	info.Mtime = time.Unix(tmp.Mtime.Tsec, tmp.Mtime.Tnsec)
	return nil
}

type Audio struct {
	SampleRate		uint32			`json:"sample_rate"`
	Channels		uint32			`json:"channels"`
	BitsPerSample		uint8			`json:"bits_per_sample"`
}

type Video struct {
	Width			uint32			`json:"width"`
	Height			uint32			`json:"height"`
}

type Track struct {
	Codec			string			`json:"codec"`
	MimeType		string			`json:"mime_type"`
	Number			uint32			`json:"number"`
	Timescale		uint32			`json:"timescale"`
	Duration		uint64			`json:"duration"`
	Bandwidth		uint32			`json:"bandwidth"`
	MediaTimescale		uint32			`json:"media_timescale"`
	MediaDuration		uint64			`json:"media_duration"`
	Audio			Audio			`json:"audio"`
	Video			Video			`json:"video"`
}

type Media struct {
	Tracks			[]Track			`json:"tracks"`
}

type Reply struct {
	Bucket			string			`json:"bucket"`
	Key			string			`json:"key"`
	ID			string			`json:"id"`
	Size			uint64			`json:"size"`

	MetaSize		uint64			`json:"meta_size"`
	MetaBucket		string			`json:"meta_bucket"`
	MetaKey			string			`json:"meta_key"`
	MetaID			string			`json:"meta_id"`

	Timestamp		time.Time
}

func (r *Reply) UnmarshalJSON(data []byte) (err error) {
	type Mtime struct {
		Tsec		int64		`json:"tsec"`
		Tnsec		int64		`json:"tnsec"`
	}

	type Alias Reply
	tmp := &struct {
		Mtime Mtime		`json:"timestamp"`
		*Alias
	} {
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	r.Timestamp = time.Unix(tmp.Mtime.Tsec, tmp.Mtime.Tnsec)
	return nil
}

