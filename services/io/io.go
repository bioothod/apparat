package io

import (
	"encoding/json"
	"fmt"
	"github.com/bioothod/apparat/services/common"
	"github.com/bioothod/apparat/services/nullx"
	"github.com/bioothod/elliptics-go/elliptics"
	"github.com/bioothod/ebucket/bindings/go"
	"github.com/golang/glog"
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
	transcoding_host	string
}

func NewIOCtl(logfile, loglevel string, remotes []string, mgroups []uint32, bnames []string, transcoding_host string) (*IOCtl, error) {
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
		transcoding_host:	transcoding_host,
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

type uploader struct {
	ctl		*IOCtl

	req		*http.Request
	key_orig	string
	key		string
	meta_key	string
	size		uint64
	ctype		string
	reader		goio.Reader
}

func (u *uploader) UploadMedia() (*common.Reply, error) {
	meta, err := u.ctl.GetBucket(u.size)
	if err != nil {
		return nil, fmt.Errorf("%s: could not get bucket, key: %s, size: %d, error: %v", u.ctype, u.key_orig, u.size, err)
	}
	groups := make([]string, 0, len(meta.Groups))
	for _, g := range meta.Groups {
		groups = append(groups, strconv.Itoa(int(g)))
	}

	url := fmt.Sprintf("http://%s/transcode/%s", u.ctl.transcoding_host, u.key)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, u.reader)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not create new HTTP request, key: %s -> %s, url: %s, size: %d, error: %v",
			u.ctype, u.key_orig, u.key, url, u.size, err)
	}
	if u.size != 0 {
		req.ContentLength = int64(u.size)
	}

	sgroups := strings.Join(groups, ":")

	req.Header.Set("Content-Type", u.ctype)

	req.Header.Set("X-Ell-Bucket", meta.Name)
	req.Header.Set("X-Ell-Key", u.key)
	req.Header.Set("X-Ell-Groups", sgroups)

	req.Header.Set("X-Ell-Meta-Bucket", meta.Name)
	req.Header.Set("X-Ell-Meta-Key", u.meta_key)
	req.Header.Set("X-Ell-Meta-Groups", sgroups)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not upload file, key: %s -> %s, url: %s, size: %d, error: %v",
			u.ctype, u.key_orig, u.key, url, u.size, err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: transcoding: could not read reply, key: %s -> %s, url: %s, size: %d, error: %v",
			u.ctype, u.key_orig, u.key, url, u.size, err)
	}

	var nullx_reply nullx.Reply
	err = json.Unmarshal(data, &nullx_reply)
	if err != nil {
		return nil, fmt.Errorf("%s: could not decode reply, key: %s -> %s, url: %s, reply: '%s', error: %v",
			u.ctype, u.key_orig, u.key, url, string(data), err)
	}

	reply := common.Reply {
		Key:		nullx_reply.Key,
		Bucket:		nullx_reply.Bucket,
		Size:		nullx_reply.Size,

		MetaKey:	nullx_reply.MetaKey,
		MetaBucket:	nullx_reply.MetaBucket,
		MetaSize:	nullx_reply.MetaSize,

		ContentType:	u.ctype,

		Timestamp:	nullx_reply.Timestamp,

		Media:		nullx_reply.Media,
	}

	return &reply, nil
}

func (u *uploader) UploadData() (*common.Reply, error) {
	session, err := elliptics.NewSession(u.ctl.node)
	if err != nil {
		return nil, fmt.Errorf("could not create new session, key: %s -> %s, error: %v", u.key_orig, u.key, err)
	}
	defer session.Delete()

	meta, err := u.ctl.GetBucket(u.size)
	if err != nil {
		return nil, fmt.Errorf("could not get bucket, key: %s -> %s, size: %d, error: %v", u.key_orig, u.key, u.size, err)
	}
	session.SetGroups(meta.Groups)
	session.SetNamespace(meta.Name)

	if u.size == 0 {
		u.size = math.MaxUint64
	}

	timestamp := time.Now()

	writer, err := elliptics.NewWriteSeeker(session, u.key, 0, u.size, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create new writer, bucket: %s, key: %s -> %s, groups: %v, size: %d, error: %v",
			meta.Name, u.key_orig, u.key, meta.Groups, u.size, err)
	}
	defer writer.Free()

	var copied int64
	copied, err = goio.Copy(writer, u.reader)
	if err != nil {
		return nil, fmt.Errorf("could not copy data, bucket: %s, key: %s -> %s, groups: %v, size: %d, copied: %d, error: %v",
			meta.Name, u.key_orig, u.key, meta.Groups, u.size, copied, err)
	}

	reply := common.Reply {
		Bucket:		meta.Name,
		Key:		u.key,
		Size:		uint64(copied),
		Timestamp:	timestamp,
	}

	return &reply, nil
}

func (u *uploader) Do() (*common.Reply, error) {
	var reply *common.Reply
	var err error

	if strings.HasPrefix(u.ctype, "audio/") || strings.HasPrefix(u.ctype, "video/") {
		reply, err = u.UploadMedia()
	} else {
		reply, err = u.UploadData()
	}

	reply.ContentType = u.ctype
	reply.Name = u.key_orig

	return reply, err
}

func (io *IOCtl) Upload(req *http.Request, key string, modifier common.ModifierFunc) ([]common.Reply, error) {
	replies := make([]common.Reply, 0)

	var size uint64
	if req.ContentLength > 0 {
		size = uint64(req.ContentLength)
	}

	ctype := req.Header.Get("Content-Type")

	u := &uploader {
		ctl:		io,
		req:		req,
		key_orig:	key,
		key:		modifier(key),
		meta_key:	modifier(common.MetaModifier()(key)),
		size:		size,
		ctype:		ctype,
	}

	mr, _ := req.MultipartReader()
	if mr == nil {
		u.reader = req.Body

		reply, err := u.Do()
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
			if ct != "" {
				u.ctype = ct
			}
			key = p.FileName()

			u.key = modifier(key)
			u.meta_key = modifier(common.MetaModifier()(key))
			u.reader = p

			reply, err := u.Do()
			if err != nil {
				return nil, err
			}

			replies = append(replies, *reply)
		}
	}

	return replies, nil
}

func (io *IOCtl) GetKey(req *http.Request, w http.ResponseWriter, bucket, key string) (int, error) {
	session, err := elliptics.NewSession(io.node)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("could not create new session, bucket: %s, key: %s, error: %v", bucket, key, err)
	}
	defer session.Delete()

	meta, err := io.FindBucket(bucket)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("could not find bucket: %s, key: %s, error: %v", bucket, key, err)
	}
	session.SetGroups(meta.Groups)
	session.SetNamespace(meta.Name)

	reader, err := elliptics.NewReadSeeker(session, key)
	if err != nil {
		status := http.StatusServiceUnavailable
		if e, ok := err.(*elliptics.DnetError); ok {
			if e.Code == -2 {
				status = http.StatusNotFound
			}
		}
		return status, fmt.Errorf("could not create new writer, bucket: %s, key: %s, groups: %v, error: %v",
			meta.Name, key, meta.Groups, err)
	}
	defer reader.Free()

	var copied int64
	copied, err = goio.Copy(w, reader)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("could not copy data, bucket: %s, key: %s, groups: %v, copied: %d, error: %v",
				meta.Name, key, meta.Groups, copied, err)
	}

	glog.Infof("GetKey: bucket: %s, key: %s, groups: %v, copied: %d", bucket, key, meta.Groups, copied)
	return http.StatusOK, nil
}

func (io *IOCtl) Get(req *http.Request, w http.ResponseWriter, bucket, key string, modifier func(x string) string) (int, error) {
	mkey := modifier(key)
	status, err := io.GetKey(req, w, bucket, mkey)
	if err != nil {
		glog.Errorf("bucket: %s, key: %s -> %s, error: %v", bucket, key, mkey, err)
	} else {
		glog.Infof("bucket: %s, key: %s -> %s", bucket, key, mkey)
	}
	return status, err
}

func (io *IOCtl) MetaJson(oldreq *http.Request, w http.ResponseWriter, bucket, key string, modifier func(x string) string) (int, error) {
	mkey := modifier(common.MetaModifier()(key))

	meta, err := io.FindBucket(bucket)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("%s: could not find bucket: %s, key: %s -> %s, error: %v",
		bucket, key, mkey, err)
	}
	groups := make([]string, 0, len(meta.Groups))
	for _, g := range meta.Groups {
		groups = append(groups, strconv.Itoa(int(g)))
	}

	url := fmt.Sprintf("http://%s/meta_json/%s/%s", io.transcoding_host, bucket, mkey)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("MetaJson: could not create new HTTP request, key: %s -> %s, url: %s, error: %v",
				key, mkey, url, err)
	}

	sgroups := strings.Join(groups, ":")

	req.Header.Set("X-Ell-Bucket", meta.Name)
	req.Header.Set("X-Ell-Key", key)
	req.Header.Set("X-Ell-Groups", sgroups)

	resp, err := client.Do(req)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("MetaJson: could not download metadata, key: %s -> %s, url: %s, error: %v",
				key, mkey, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode,
			fmt.Errorf("MetaJson: could not download metadata, key: %s -> %s, url: %s",
				key, mkey, url)
	}

	for hk, hv := range resp.Header {
		for _, v := range(hv) {
			w.Header().Add(hk, v)
		}
	}

	_, err = goio.Copy(w, resp.Body)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("MetaJson: could copy metadata, key: %s -> %s, url: %s, error: %v",
				key, mkey, url, err)
	}

	return http.StatusOK, nil
}
