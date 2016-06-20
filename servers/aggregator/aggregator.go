package main

import (
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
)

type Forwarder struct {
	addr		string
}

func (f *Forwarder) forward(c *gin.Context) {
	method := c.Request.Method

	url := c.Request.URL
	url.Host = f.addr
	url.Scheme = "http"

	req, err := http.NewRequest(method, url.String(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not create new request: method: %s, url: %s, error: %v", method, url.String(), err),
		})
		return
	}

	req.Header = c.Request.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "forward",
			"error": fmt.Sprintf("could not perform operation: method: %s, url: %s, error: %v", method, url.String(), err),
		})
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, hv := range v {
			c.Writer.Header().Add(k, hv)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func main() {
	addr := flag.String("addr", "", "address to listen auth server at")
	auth_addr := flag.String("auth-addr", "", "address where auth server lives")
	index_addr := flag.String("index-addr", "", "address where index server lives")
	static := flag.String("static", "", "directory for static content")

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


	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

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

	if *static == "" {
		log.Printf("[WARN] no static content directory provided, static files handling will be disabled")
	} else {
		r.Static("/", *static)
	}


	http.ListenAndServe(*addr, r)
}
