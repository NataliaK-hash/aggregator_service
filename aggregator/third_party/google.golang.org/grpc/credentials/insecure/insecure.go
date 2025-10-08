package insecure

import (
	"context"
	"net"

	"google.golang.org/grpc/credentials"
)

// NewCredentials returns transport credentials that use an insecure connection.
func NewCredentials() credentials.TransportCredentials {
	return insecureTC{}
}

type insecureTC struct{}

type insecureAuthInfo struct{}

func (insecureTC) ClientHandshake(ctx context.Context, addr string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, insecureAuthInfo{}, nil
}

func (insecureTC) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, insecureAuthInfo{}, nil
}

func (insecureTC) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "insecure"}
}

func (insecureTC) Clone() credentials.TransportCredentials {
	return insecureTC{}
}

func (insecureTC) OverrideServerName(string) error {
	return nil
}

func (insecureAuthInfo) AuthType() string {
	return "insecure"
}
