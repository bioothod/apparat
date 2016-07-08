package aggregator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bioothod/apparat/services/auth"
	"github.com/bioothod/apparat/services/index"
	"github.com/bioothod/apparat/services/common"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strings"
)


type Indexer struct {
	Forwarder

	IndexUrl		string
}

func (idx *Indexer) Forward(c *gin.Context) {
	filename := strings.Trim(c.Request.URL.Path[len("/upload/"):], "/")
	if len(filename) == 0 {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("invalid url: %s, must contain '/upload/filename'", c.Request.URL.String()),
		})
		return
	}

	resp, err := idx.Send(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		idx.Flush(c, resp)
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not read response: %v", err),
		})
		return
	}

	type io_reply struct {
		Operation		string		`json:"operation"`
		Reply			[]common.Reply	`json:"reply"`
	}
	var iore io_reply

	err = json.Unmarshal(data, &iore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not unpack JSON response: '%s', error: %v", string(data), err),
		})
		return
	}

	r := iore.Reply[0]

	tag := r.Timestamp.Format("2006-01-02")
	tags := []string{tag, "all"}

	if len(r.Media.Tracks) != 0 {
		for _, track := range r.Media.Tracks {
			if strings.HasPrefix(track.MimeType, "audio/") {
				tags = append(tags, "audio")
			}
			if strings.HasPrefix(track.MimeType, "video/") {
				tags = append(tags, "video")
			}
		}
	} else {
		ctype := r.ContentType
		if strings.HasPrefix(ctype, "audio/") {
			tags = append(tags, "audio")
		}
		if strings.HasPrefix(ctype, "video/") {
			tags = append(tags, "video")
		}
		if strings.HasPrefix(ctype, "image/") {
			tags = append(tags, "image")
		}
	}

	ireq := &index.IndexRequest {
		Files: []index.Request {
			index.Request {
				File: common.Reply {
					Key:		r.Key,
					Bucket:		r.Bucket,
					Name:		r.Name,
					Timestamp:	r.Timestamp,
					Size:		r.Size,
				},
				Tags: tags,
			},
		},
	}

	index_data, err := json.Marshal(&ireq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not pack JSON index request, error: %v", err),
		})
		return
	}

	breader := bytes.NewReader(index_data)

	client := &http.Client{}
	index_req, err := http.NewRequest("POST", idx.IndexUrl, breader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not create index request to url: %s, error: %v", idx.IndexUrl, err),
		})
		return
	}
	cookie, err := c.Request.Cookie(auth.CookieName)
	if err == nil {
		index_req.AddCookie(cookie)
	}

	index_resp, err := client.Do(index_req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not send index request: '%s', error: %v", string(index_data), err),
		})
		return
	}
	defer index_resp.Body.Close()

	if index_resp.StatusCode != http.StatusOK {
		c.JSON(index_resp.StatusCode, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not send index request: '%s', status: %d", string(index_data), index_resp.StatusCode),
		})
		return
	}

	for k, v := range resp.Header {
		for _, hv := range v {
			c.Writer.Header().Add(k, hv)
		}
	}
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(data)
}
