package aggregator

import (
	"fmt"
	"github.com/bioothod/apparat/services/common"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

type Forwarder struct {
	Addr		string
}

func (f *Forwarder) Send(c *gin.Context) (*http.Response, error) {
	method := c.Request.Method

	url := c.Request.URL
	url.Host = f.Addr
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

func (f *Forwarder) Flush(c *gin.Context, resp *http.Response) {
	for k, v := range resp.Header {
		for _, hv := range v {
			c.Writer.Header().Add(k, hv)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (f *Forwarder) Forward(c *gin.Context) {
	resp, err := f.Send(c)
	if err != nil {
		estr := fmt.Sprintf("could not forward request: destination: method: %s, addres: %s, path: %s, error: %v",
			c.Request.Method,
			f.Addr,
			c.Request.URL.Path,
			err)
		common.NewErrorString(c, "forward", estr)
		c.JSON(http.StatusInternalServerError, gin.H {
			"operation": "forward",
			"error": estr,
		})
		return
	}
	defer resp.Body.Close()

	f.Flush(c, resp)
}

