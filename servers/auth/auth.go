package main

import (
	"flag"
	"fmt"
	"github.com/bioothod/apparat/middleware"
	"github.com/bioothod/apparat/services/auth"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

var authCtl *auth.AuthCtl

type Mailbox struct {
	Username		string		`form:"username" json:"username" binding:"required"`
	Password		string		`form:"password" json:"password" binding:"required"`
	Realname		string		`form:"realname" json:"realname"`
	Email			string		`form:"email" json:"email"`
}

func FromRequest(c *gin.Context) (*auth.Mailbox, error) {
	var mbox Mailbox

	err := c.Bind(&mbox)
	if err != nil {
		return nil, fmt.Errorf("inalid form data: %v", err)
	}

	ambox := &auth.Mailbox {
		Username:		mbox.Username,
		Password:		mbox.Password,
		Realname:		mbox.Realname,
		Email:			mbox.Email,
	}

	return ambox, nil
}

func user_signup(c *gin.Context) {
	mbox, err := FromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "signup",
			"error": fmt.Sprintf("could not parse mailbox: %v", err),
		})
		return
	}

	err = authCtl.NewUser(mbox)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "signup",
			"error": fmt.Sprintf("could not create new user: %v", err),
		})
		return
	}

	err = auth.SetAuthCookie(c.Request, c.Writer, auth.NewAuthCookie(mbox.Username))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H {
			"operation": "signup",
			"error": fmt.Sprintf("could not set cookie: %v", err),
		})
		return
	}

	mbox.Password = ""
	c.JSON(http.StatusOK, gin.H {
		"operation": "signup",
		"mailbox": mbox,
	})
}

func user_login(c *gin.Context) {
	mbox, err := FromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "login",
			"error": fmt.Sprintf("could not parse mailbox: %v", err),
		})
		return
	}

	err = authCtl.GetUser(mbox)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "login",
			"error": fmt.Sprintf("could not check user: %s, error: %v", mbox.Username, err),
		})
		return
	}

	err = auth.SetAuthCookie(c.Request, c.Writer, auth.NewAuthCookie(mbox.Username))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H {
			"operation": "login",
			"error": fmt.Sprintf("could not set cookie: %v", err),
		})
		return
	}

	mbox.Password = ""
	c.JSON(http.StatusOK, gin.H {
		"operation": "login",
		"mailbox": mbox,
	})
}

func user_update(c *gin.Context) {
	mbox, err := FromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "update",
			"error": fmt.Sprintf("could not parse mailbox: %v", err),
		})
		return
	}

	err = authCtl.UpdateUser(mbox)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H {
			"operation": "update",
			"error": fmt.Sprintf("could not update user: %s, error: %v", mbox.Username, err),
		})
		return
	}

	mbox.Password = ""
	c.JSON(http.StatusOK, gin.H {
		"operation": "update",
		"mailbox": mbox,
	})
}

func check_cookie(c *gin.Context) {
	ac, err := auth.CheckAuthCookie(c.Request)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H {
			"operation": "check",
			"error": fmt.Sprintf("cookie check has failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H {
		"operation": "check",
		"auth": ac,
	})
}

func main() {
	addr := flag.String("addr", "", "address to listen auth server at")
	dbparams := flag.String("db", "", "mysql database parameters:\n" +
		"	user@unix(/path/to/socket)/dbname?charset=utf8\n" +
		"	user:password@tcp(localhost:5555)/dbname?charset=utf8\n" +
		"	user:password@/dbname\n" +
		"	user:password@tcp([de:ad:be:ef::ca:fe]:80)/dbname")
	cookie_auth := flag.String("cookie-auth", "", "key to authenticate cookies")
	cookie_encrypt := flag.String("cookie-encrypt", "", "key to encrypt cookies")
	cookie_path := flag.String("cookie-path", "/", "cookie path")

	flag.Parse()
	if *addr == "" {
		log.Fatalf("You must provide address where auth server will listen for incoming connections")
	}
	if *dbparams == "" {
		log.Fatalf("You must provide mysql auth database parameters")
	}
	if *cookie_auth == "" {
		log.Fatalf("you must provide auth key")
	}

	var err error
	authCtl, err = auth.NewAuthCtl("mysql", *dbparams)
	if err != nil {
		log.Fatalf("could not connect to MySQL database '%s': %v", *dbparams, err)
	}
	defer authCtl.Close()

	var cookie_keys [][]byte
	cookie_keys = make([][]byte, 0)
	cookie_keys = append(cookie_keys, []byte(*cookie_auth))

	if *cookie_encrypt != "" {
		cookie_keys = append(cookie_keys, []byte(*cookie_encrypt))
	}

	auth.InitCookieStore(cookie_keys, *cookie_path)

	r := gin.New()
	r.Use(middleware.XTrace())
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ClearGorillaContext())
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		err := authCtl.Ping()
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

	r.POST("/login", user_login)
	r.POST("/signup", user_signup)
	r.POST("/check", check_cookie)

	authorized := r.Group("/", middleware.AuthRequired(*addr))
	authorized.POST("/update", user_update)

	http.ListenAndServe(*addr, r)
}
