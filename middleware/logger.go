package middleware

import (
	"github.com/bioothod/apparat/services/common"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"net/http"
	"time"
)

func Logger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
	path := c.Request.URL.Path

        c.Next()

	end := time.Now()
	latency := end.Sub(start)

	clientIP := c.ClientIP()
	method := c.Request.Method
        status := c.Writer.Status()
	xreq := c.Request.Header.Get(XRequestHeader)

	errors := make([]common.Error, 0)
	if status != http.StatusOK {
		e, exists := c.Get(common.ErrorKey)
		if exists {
			errors = e.([]common.Error)
		}
	}

	glog.Infof("xreq: %s, status: %3d, duration: %13v, client: %s, method: %s, path: %s, errors: %s",
		xreq,
		status,
		latency,
		clientIP,
		method,
		path,
		errors,
	)
    }
}
