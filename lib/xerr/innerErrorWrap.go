package xerr

import (
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type Error struct {
	e error
}

var NilXerr zError = nil

type zError interface {
	XError() Error
	Error() string
}

func (xerr *Error) XError() Error {
	return *xerr
}

func (xerr *Error) Error() string {
	return xerr.String()
	// return xerr.error.Error()
}

func (xerr *Error) String() string {
	return fmt.Sprintf("%+v", xerr.e)
}

func AnyErr(results map[string]error) (string, error) {
	for k,v := range results {
		if v != nil {
			return k, v
		}
	}
	return "", nil
}

func NewXError(err error, msgArr ...string) error {
	if err == nil {
		return nil
	}
	if err == redis.Nil {
		return redis.Nil
	}
	msg := ""
	if len(msgArr) > 0 {
		msg = strings.Join(msgArr, "\n")
	}
	xerr, ok := err.(zError)
	if ok && msg == "" {
		return xerr
	}
	if !ok {
		// new XError
		_err := errors.Wrap(err, "\n"+msg)
		xerr = &Error{e: _err}
		return xerr
	}
	// add message to bottom error
	_err := xerr.XError().e
	_err = errors.Wrap(_err, "\n"+msg)
	xerr = &Error{e: _err}
	return xerr
}
func NewError(content string, msgArr ...string) error {
	err := errors.New(content)
	msg := ""
	if len(msgArr) > 0 {
		msg = strings.Join(msgArr, "\n")
	}
	xerr, ok := err.(zError)
	if ok && msg == "" {
		return xerr
	}
	if !ok {
		// new XError
		_err := errors.Wrap(err, "\n"+msg)
		xerr = &Error{e: _err}
		return xerr
	}
	// add message to bottom error
	_err := xerr.XError().e
	_err = errors.Wrap(_err, "\n"+msg)
	xerr = &Error{e: _err}
	return xerr
}
