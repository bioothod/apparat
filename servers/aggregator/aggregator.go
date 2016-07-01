package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/bioothod/apparat/services/auth"
	"github.com/bioothod/apparat/services/index"
	"github.com/bioothod/apparat/services/common"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func static_index_handler(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := os.Open(root + "/index.html")
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		defer file.Close()

		var t time.Time
		http.ServeContent(c.Writer, c.Request, "index.html", t, file)
	}
}

type Forwarder struct {
	addr		string
}

func (f *Forwarder) send(c *gin.Context) (*http.Response, error) {
	method := c.Request.Method

	url := c.Request.URL
	url.Host = f.addr
	url.Scheme = "http"

	req, err := http.NewRequest(method, url.String(), c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("could not create new request: method: %s, url: %s, error: %v", method, url.String(), err)
	}

	req.Header = c.Request.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform operation: method: %s, url: %s, error: %v", method, url.String(), err)
	}

	return resp, nil
}

func (f *Forwarder) flush(c *gin.Context, resp *http.Response) {
	for k, v := range resp.Header {
		for _, hv := range v {
			c.Writer.Header().Add(k, hv)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (f *Forwarder) forward(c *gin.Context) {
	resp, err := f.send(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	f.flush(c, resp)
}


type Indexer struct {
	Forwarder

	index_url		string
}

func (idx *Indexer) forward(c *gin.Context) {
	filename := strings.Trim(c.Request.URL.Path[len("/upload/"):], "/")
	if len(filename) == 0 {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("invalid url: %s, must contain '/upload/filename'", c.Request.URL.String()),
		})
		return
	}

	resp, err := idx.send(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		idx.flush(c, resp)
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
	index_req, err := http.NewRequest("POST", idx.index_url, breader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not create index request to url: %s, error: %v", idx.index_url, err),
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

func main() {
	addr := flag.String("addr", "", "address to listen auth server at")
	auth_addr := flag.String("auth-addr", "", "address where auth server lives")
	index_addr := flag.String("index-addr", "", "address where index server lives")
	io_addr := flag.String("io-addr", "", "address where IO server lives")
	static_dir := flag.String("static", "", "directory for static content")

	flag.Parse()
	if *addr == "" {
		log.Fatalf("You must provide address where auth server will listen for incoming connections")
	}
	if *auth_addr == "" {
		log.Fatalf("You must provide auth server addr")
	}
	if *index_addr == "" {
		log.Fatalf("You must provide index server addr")
	}
	if *io_addr == "" {
		log.Fatalf("You must provide IO server addr")
	}


	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())


	if *static_dir == "" {
		log.Printf("[WARN] no static content directory provided, static files handling will be disabled")
	} else {
		// this is needed since otherwise ServeFile() redirects /index.html to / and there is no wildcard / handler
		// / wildcard handler can not be added, since it will clash with /get and other GET handlers
		// instead we have this static middleware which checks everything against static root and handles
		// files via http.FileServer.ServerHTTP() which ends up calling http.ServeFile() with its weird redirect
		r.GET("/index.html", static_index_handler(*static_dir))
		r.Use(static.Serve("/", static.LocalFile(*static_dir, false)))
	}

	auth_forwarder := &Forwarder {
		addr:	*auth_addr,
	}

	r.POST("/login", func (c *gin.Context) {
		auth_forwarder.forward(c)
	})
	r.POST("/signup", func (c *gin.Context) {
		auth_forwarder.forward(c)
	})
	r.POST("/update", func (c *gin.Context) {
		auth_forwarder.forward(c)
	})

	index_forwarder := &Forwarder {
		addr:	*index_addr,
	}
	r.POST("/index", func (c *gin.Context) {
		index_forwarder.forward(c)
	})
	r.POST("/list", func (c *gin.Context) {
		index_forwarder.forward(c)
	})
	r.POST("/list_meta", func (c *gin.Context) {
		index_forwarder.forward(c)
	})

	io_forwarder := &Indexer {
		Forwarder: Forwarder {
			addr:	*io_addr,
		},
		index_url: fmt.Sprintf("http://%s/index", *index_addr),
	}
	r.POST("/upload/:key", func (c *gin.Context) {
		io_forwarder.forward(c)
	})
	r.GET("/get/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.forward(c)
	})
	r.GET("/get_key/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.forward(c)
	})
	r.GET("/meta_json/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.forward(c)
	})


	http.ListenAndServe(*addr, r)
}
