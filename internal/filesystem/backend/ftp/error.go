package ftp

import (
	"net/textproto"

	"github.com/jlaffaye/ftp"
)

func isBadCommand(err error) bool {
	return isProtoCodeErr(err, ftp.StatusBadCommand)
}

func isNotImplementedErr(err error) bool {
	return isProtoCodeErr(err, ftp.StatusNotImplemented)
}

func isFileUnavailableErr(err error) bool {
	return isProtoCodeErr(err, ftp.StatusFileUnavailable)
}

func isProtoCodeErr(err error, code int) bool {
	protoErr, ok := err.(*textproto.Error)
	if !ok {
		return false
	}

	return protoErr.Code == code
}
