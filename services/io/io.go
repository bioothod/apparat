package io

import (
	"encoding/json"
	"fmt"
	"github.com/bioothod/elliptics-go/elliptics"
	"github.com/bioothod/ebucket/bindings/go"
	goio "io"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type IOCtl struct {
	node		*elliptics.Node
	bp		*ebucket.BucketProcessor
	transcoding_url	string
}

func NewIOCtl(logfile, loglevel string, remotes []string, mgroups []uint32, bnames []string, transcoding_url string) (*IOCtl, error) {
	node, err := elliptics.NewNode(logfile, loglevel)
	if err != nil {
		return nil, err
	}
	err = node.AddRemotes(remotes)
	if err != nil {
		node.Free()
		return nil, err
	}

	bp, err := ebucket.NewBucketProcessor(node, mgroups, bnames)
	if err != nil {
		node.Free()
		return nil, err
	}

	return &IOCtl {
		node:			node,
		bp:			bp,
		transcoding_url:	transcoding_url,
	}, nil
}

func (io *IOCtl) Close() {
	io.bp.Close()
	io.node.Free()
}

func (io *IOCtl) Ping() error {
	_, err := io.GetBucket(1024)
	if err != nil {
		return err
	}

	return nil
}

func (io *IOCtl) GetBucket(size uint64) (*ebucket.BucketMeta, error) {
	return io.bp.GetBucket(size)
}

func (io *IOCtl) FindBucket(name string) (*ebucket.BucketMeta, error) {
	return io.bp.FindBucket(name)
}

type Reply struct {
	Bucket		string
	Key		string
	Size		uint64

	MetaBucket	string
	MetaKey		string
	MetaSize	uint64
}

type NullxInfo struct {
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

func (info *NullxInfo) UnmarshalJSON(data []byte) (err error) {
	type Mtime struct {
		Tsec		int64		`json:"tsec"`
		Tnsec		int64		`json:"tnsec"`
	}
	type Alias NullxInfo
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

type NullxReply struct {
	Bucket			string			`json:"bucket"`
	Key			string			`json:"key"`
	ID			string			`json:"id"`
	Size			uint64			`json:"size"`
	MetaBucket		string			`json:"meta_bucket"`
	MetaKey			string			`json:"meta_key"`
	MetaID			string			`json:"meta_id"`
	MetaSize		uint64			`json:"meta_size"`
}


func (io *IOCtl) UploadMedia(oldreq *http.Request, key, meta_key string, size uint64, ctype string, reader goio.Reader) (*Reply, error) {
	meta, err := io.GetBucket(size)
	if err != nil {
		return nil, fmt.Errorf("%s: could not get bucket, key: %s, size: %d, error: %v", ctype, key, size, err)
	}
	groups := make([]string, 0, len(meta.Groups))
	for _, g := range meta.Groups {
		groups = append(groups, strconv.Itoa(int(g)))
	}

	url := fmt.Sprintf("%s/%s", io.transcoding_url, key)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not create new HTTP request, key: %s, url: %s, size: %d, error: %v",
			ctype, key, url, size, err)
	}
	if size != 0 {
		req.ContentLength = int64(size)
	}

	sgroups := strings.Join(groups, ":")

	req.Header.Set("Content-Type", ctype)

	req.Header.Set("X-Ell-Bucket", meta.Name)
	req.Header.Set("X-Ell-Key", key)
	req.Header.Set("X-Ell-Groups", sgroups)

	req.Header.Set("X-Ell-Meta-Bucket", meta.Name)
	req.Header.Set("X-Ell-Meta-Key", meta_key)
	req.Header.Set("X-Ell-Meta-Groups", sgroups)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not upload file, key: %s, url: %s, size: %d, error: %v",
			ctype, key, url, size, err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not read reply, key: %s, url: %s, size: %d, error: %v",
			ctype, key, url, size, err)
	}

	var nullx_reply NullxReply
	err = json.Unmarshal(data, &nullx_reply)
	if err != nil {
		return nil, fmt.Errorf("%s: could not decode reply, key: %s, url: %s, reply: '%s', error: %v",
			ctype, key, url, string(data), err)
	}

	reply := Reply {
		Key:		nullx_reply.Key,
		Bucket:		nullx_reply.Bucket,
		Size:		nullx_reply.Size,

		MetaKey:	nullx_reply.MetaKey,
		MetaBucket:	nullx_reply.MetaBucket,
		MetaSize:	nullx_reply.MetaSize,
	}

	return &reply, nil
}

func (io *IOCtl) UploadData(oldreq *http.Request, key string, size uint64, reader goio.Reader) (*Reply, error) {
	session, err := elliptics.NewSession(io.node)
	if err != nil {
		return nil, fmt.Errorf("could not create new session, key: %s, error: %v", key, err)
	}
	defer session.Delete()

	meta, err := io.GetBucket(size)
	if err != nil {
		return nil, fmt.Errorf("could not get bucket, key: %s, size: %d, error: %v", key, size, err)
	}
	session.SetGroups(meta.Groups)
	session.SetNamespace(meta.Name)

	if size == 0 {
		size = math.MaxUint64
	}

	writer, err := elliptics.NewWriteSeeker(session, key, 0, size, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create new writer, bucket: %s, key: %s, groups: %v, size: %d, error: %v",
			meta.Name, key, meta.Groups, size, err)
	}
	defer writer.Free()

	var copied int64
	copied, err = goio.Copy(writer, reader)
	if err != nil {
		return nil, fmt.Errorf("could not copy data, bucket: %s, key: %s, groups: %v, size: %d, copied: %d, error: %v",
			meta.Name, key, meta.Groups, size, copied, err)
	}

	reply := Reply {
		Bucket:		meta.Name,
		Key:		key,
		Size:		uint64(copied),
	}

	return &reply, nil
}

func (io *IOCtl) UploadOne(req *http.Request, key, meta_key string, size uint64, ctype string, reader goio.Reader) (*Reply, error) {
	if strings.HasPrefix(ctype, "audio/") || strings.HasPrefix(ctype, "video/") {
		return io.UploadMedia(req, key, meta_key, size, ctype, reader)
	} else {
		return io.UploadData(req, key, size, reader)
	}
}

func (io *IOCtl) Upload(req *http.Request, key string, modifier func(x string) string) ([]Reply, error) {
	replies := make([]Reply, 0)

	var size uint64
	if req.ContentLength > 0 {
		size = uint64(req.ContentLength)
	}

	meta_key := fmt.Sprintf("meta\x00%s", key)

	ctype := req.Header.Get("Content-Type")

	mr, _ := req.MultipartReader()
	if mr == nil {
		reply, err := io.UploadOne(req, modifier(key), modifier(meta_key), size, ctype, req.Body)
		if err != nil {
			return nil, err
		}

		replies = append(replies, *reply)
	} else {
		for {
			p, err := mr.NextPart()
			if err == goio.EOF {
				break
			}
			if err != nil {
				return nil, err
			}

			ct := p.Header.Get("Content-Type")
			if ct == "" {
				ct = ctype
			}
			key = p.FileName()

			reply, err := io.UploadOne(req, modifier(key), modifier(meta_key), size, ct, p)
			if err != nil {
				return nil, err
			}

			replies = append(replies, *reply)
		}
	}

	return replies, nil
}
