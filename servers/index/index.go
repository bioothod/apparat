package main

import (
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/bioothod/apparat/services/common"
	"github.com/bioothod/apparat/services/index"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

var idxCtl *index.IndexCtl

func index_tags(c *gin.Context) {
	username := c.MustGet("username").(string)
	idx, err := index.NewIndexer(username, idxCtl)
	if err != nil {
		estr := fmt.Sprintf("could not create new indexer for user '%s', error: %v", username, err)
		common.NewErrorString(c, "index", estr)
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "index",
			"error": estr,
		})
		return
	}

	var ireq index.IndexRequest
	err = c.BindJSON(&ireq)
	if err != nil {
		estr := fmt.Sprintf("could not parse json request from user '%s', error: %v", username, err)
		common.NewErrorString(c, "index", estr)
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "index",
			"error": estr,
		})
		return
	}

	err = idx.Index(&ireq)
	if err != nil {
		estr := fmt.Sprintf("could not index tags from user '%s', error: %v", username, err)
		common.NewErrorString(c, "index", estr)
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "index",
			"error": estr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H {
		"operation": "index",
	})
}

func list_meta_tags(c *gin.Context) {
	username := c.MustGet("username").(string)
	idx, err := index.NewIndexer(username, idxCtl)
	if err != nil {
		estr := fmt.Sprintf("could not create new indexer for user '%s', error: %v", username, err)
		common.NewErrorString(c, "list_meta", estr)
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "list_meta",
			"error": estr,
		})
		return
	}

	reply, err := idx.ListMeta()
	if err != nil {
		estr := fmt.Sprintf("could not list meta tags from user '%s', error: %v", username, err)
		common.NewErrorString(c, "list_meta", estr)
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "list_meta",
			"error": estr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H {
		"operation": "list_meta",
		"reply": reply,
	})
}

func list_tags(c *gin.Context) {
	username := c.MustGet("username").(string)
	idx, err := index.NewIndexer(username, idxCtl)
	if err != nil {
		estr := fmt.Sprintf("could not create new indexer for user '%s', error: %v", username, err)
		common.NewErrorString(c, "list", estr)
		c.JSON(http.StatusServiceUnavailable, gin.H {
			"operation": "list",
			"error": estr,
		})
		return
	}

	var obj index.ListRequest
	err = c.BindJSON(&obj)
	if err != nil {
		estr := fmt.Sprintf("could not parse json request from user '%s', error: %v", username, err)
		common.NewErrorString(c, "list", estr)
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "list",
			"error": estr,
		})
		return
	}

	reply, err := idx.List(&obj)
	if err != nil {
		estr := fmt.Sprintf("could not list tags from user '%s', error: %v", username, err)
		common.NewErrorString(c, "list", estr)
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "list",
			"error": estr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H {
		"operation": "list",
		"reply": reply,
	})
}

func main() {
	addr := flag.String("addr", "", "address to listen auth server at")
	dbparams := flag.String("db", "", "mysql database parameters:\n" +
		"	user@unix(/path/to/socket)/dbname?charset=utf8\n" +
		"	user:password@tcp(localhost:5555)/dbname?charset=utf8\n" +
		"	user:password@/dbname\n" +
		"	user:password@tcp([de:ad:be:ef::ca:fe]:80)/dbname")
	auth := flag.String("auth", "", "authentication check service (full-featured URL like http://auth.example.com:1234/check)")


	flag.Parse()
	if *addr == "" {
		log.Fatalf("You must provide address where auth server will listen for incoming connections")
	}
	if *dbparams == "" {
		log.Fatalf("You must provide mysql auth database parameters")
	}
	if *auth == "" {
		log.Fatalf("You must provide authentication service URL")
	}

	var err error
	idxCtl, err = index.NewIndexCtl("mysql", *dbparams)
	if err != nil {
		log.Fatalf("could not connect to MySQL database '%s': %v", *dbparams, err)
	}
	defer idxCtl.Close()

	r := gin.New()
	r.Use(middleware.XTrace())
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ClearGorillaContext())
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		err := idxCtl.Ping()
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
	authorized.POST("/index", index_tags)
	authorized.POST("/list", list_tags)
	authorized.POST("/list_meta", list_meta_tags)

	http.ListenAndServe(*addr, r)
}
