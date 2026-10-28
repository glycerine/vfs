package oserror

import (
	"errors"
	"io/fs"
)

var (
	ErrExist      = fs.ErrExist
	ErrNotExist   = fs.ErrNotExist
	ErrPermission = fs.ErrPermission
	ErrInvalid    = fs.ErrInvalid
)

func IsExist(err error) bool      { return errors.Is(err, fs.ErrExist) }
func IsNotExist(err error) bool   { return errors.Is(err, fs.ErrNotExist) }
func IsPermission(err error) bool { return errors.Is(err, fs.ErrPermission) }
