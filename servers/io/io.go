package main

import (
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/bioothod/apparat/services/io"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var ioCtl *io.IOCtl

type modifier_func func(string) string

func key_modifier(username string) modifier_func {
	return func(key string) string {
		return username + "." + key
	}
}

func upload_handler(c *gin.Context) {
	username := c.MustGet("username").(string)
	key := c.Param("key")

	reply, err := ioCtl.Upload(c.Request, key, key_modifier(username))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "io",
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H {
		"operation": "io",
		"reply": reply,
	})
}

type sslice []string
func (sl *sslice) String() string {
	return fmt.Sprintf("%s", *sl)
}
func (sl *sslice) Set(value string) error {
	*sl = append(*sl, value)
	return nil
}

func main() {
	var bnames sslice
	flag.Var(&bnames, "bucket", "list of bucket names to work with")

	addr := flag.String("addr", "", "address to listen auth server at")
	mgroups := flag.String("metadata-groups", "", "colon-separated list of metadata groups, format: 1:2:3")
	auth := flag.String("auth", "", "authentication check service (full-featured URL like http://auth.example.com:1234/check)")
	transcode := flag.String("transcode", "", "Nullx transcoding service URL (example: http://nullx.example.com:1234/transcode)")
	logfile := flag.String("log-file", "/dev/stdout", "Elliptics log file")
	loglevel := flag.String("log-level", "error", "Elliptics log level (debug, notice, info, error)")
	var remotes sslice
	flag.Var(&remotes, "remote", "list of remote elliptics nodes, format: addr:port:family")

	flag.Parse()
	if *addr == "" {
		log.Fatalf("You must provide address where auth server will listen for incoming connections")
	}
	if *auth == "" {
		log.Fatalf("You must provide authentication service URL")
	}
	if len(bnames) == 0 {
		log.Fatalf("You must provide list of bucket names")
	}
	if *mgroups == "" {
		log.Fatalf("You must provide metadata groups")
	}
	if *transcode == "" {
		log.Fatalf("You must provide Nullx transcoding service URL")
	}
	if len(remotes) == 0 {
		log.Fatalf("You must provide one or more remote elliptics nodes")
	}

	mg := make([]uint32, 0)
	for _, s := range strings.Split(*mgroups, ":") {
		group, err := strconv.Atoi(s)
		if err != nil {
			log.Fatalf("Invalid metadata groups %s: %v", *mgroups, err)
		}

		mg = append(mg, uint32(group))
	}
	if len(mg) == 0 {
		log.Fatalf("Invalid metadata groups %s", *mgroups)
	}

	var err error
	ioCtl, err = io.NewIOCtl(*logfile, *loglevel, remotes, mg, bnames, *transcode)
	if err != nil {
		log.Fatalf("Could not create new IO controller: %v", err)
	}
	defer ioCtl.Close()

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ClearGorillaContext())
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		err := ioCtl.Ping()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H {
				"message": err,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H {
			"message": "Ok",
		})
	})

	authorized := r.Group("/", middleware.AuthRequired(*auth))
	authorized.POST("/upload/:key", upload_handler)

	http.ListenAndServe(*addr, r)
}
