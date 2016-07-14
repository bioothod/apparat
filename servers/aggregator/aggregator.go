package main

import (
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/bioothod/apparat/services/common"
	"github.com/bioothod/apparat/services/aggregator"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"time"
)

func static_index_handler(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := os.Open(root + "/index.html")
		if err != nil {
			common.NewError(c, "static", err)

			c.Status(http.StatusBadRequest)
			return
		}
		defer file.Close()

		var t time.Time
		http.ServeContent(c.Writer, c.Request, "index.html", t, file)
	}
}


func main() {
	addr := flag.String("addr", "", "address to listen auth server at")
	auth_addr := flag.String("auth-addr", "", "address where auth server lives")
	index_addr := flag.String("index-addr", "", "address where index server lives")
	io_addr := flag.String("io-addr", "", "address where IO server lives")
	static_dir := flag.String("static", "", "directory for static content")
	nulla_addr := flag.String("nulla-addr", "", "address where Nulla streaming server lives")

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
	if *nulla_addr == "" {
		log.Fatalf("You must provide Nulla server addr")
	}


	r := gin.New()
	r.Use(middleware.XTrace())
	r.Use(middleware.Logger())
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
		r.GET("/", static_index_handler(*static_dir))
		r.Use(static.Serve("/", static.LocalFile(*static_dir, false)))
	}

	auth_forwarder := &aggregator.Forwarder {
		Addr:	*auth_addr,
	}

	r.POST("/login", func (c *gin.Context) {
		auth_forwarder.Forward(c)
	})
	r.POST("/signup", func (c *gin.Context) {
		auth_forwarder.Forward(c)
	})
	r.POST("/update", func (c *gin.Context) {
		auth_forwarder.Forward(c)
	})

	index_forwarder := &aggregator.Forwarder {
		Addr:	*index_addr,
	}
	r.POST("/index", func (c *gin.Context) {
		index_forwarder.Forward(c)
	})
	r.POST("/list", func (c *gin.Context) {
		index_forwarder.Forward(c)
	})
	r.POST("/list_meta", func (c *gin.Context) {
		index_forwarder.Forward(c)
	})

	nulla_forwarder := &aggregator.Forwarder {
		Addr:	*nulla_addr,
	}
	r.POST("/manifest", func (c *gin.Context) {
		nulla_forwarder.Forward(c)
	})

	io_forwarder := &aggregator.Indexer {
		Forwarder: aggregator.Forwarder {
			Addr:	*io_addr,
		},
		IndexUrl: fmt.Sprintf("http://%s/index", *index_addr),
	}
	r.POST("/upload/:key", func (c *gin.Context) {
		io_forwarder.Forward(c)
	})
	r.GET("/get/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.Forward(c)
	})
	r.GET("/get_key/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.Forward(c)
	})
	r.GET("/meta_json/:bucket/:key", func (c *gin.Context) {
		io_forwarder.Forwarder.Forward(c)
	})

	http.ListenAndServe(*addr, r)
}
