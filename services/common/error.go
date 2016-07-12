package common

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

const ErrorKey string = "error"

type Error struct {
	Operation		string		`json:"operation"`
	ErrorString		string		`json:"error"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("operation: %s, error: %s", e.Operation, e.ErrorString)
}

func NewError(c *gin.Context, operation string, err error) {
	e, exists := c.Get(ErrorKey)
	if !exists {
		e = make([]Error, 0, 1)
	}

	err_ctx := Error {
		Operation:		operation,
		ErrorString:		err.Error(),
	}

	err_slice := e.([]Error)

	err_slice = append(err_slice, err_ctx)
	c.Set(ErrorKey, err_slice)
}
