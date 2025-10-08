package status

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
)

type Status struct {
	code codes.Code
	msg  string
}

func (s *Status) Code() codes.Code {
	if s == nil {
		return codes.OK
	}
	return s.code
}

func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return s.msg
}

func Errorf(c codes.Code, format string, args ...interface{}) error {
	return &Status{code: c, msg: fmt.Sprintf(format, args...)}
}

func (s *Status) Error() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("rpc error: code = %s desc = %s", s.code.String(), s.msg)
}

// Code returns the canonical gRPC status code for the provided error.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	var coder interface{ Code() codes.Code }
	if errors.As(err, &coder) {
		return coder.Code()
	}
	return codes.Unknown
}
