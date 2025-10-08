package credentials

import (
	"context"
	"net"
)

// AuthInfo represents the auth information for a connection.
type AuthInfo interface {
	AuthType() string
}

// ProtocolInfo contains information about the security protocol in use.
type ProtocolInfo struct {
	SecurityProtocol string
	SecurityVersion  string
	ServerName       string
}

// TransportCredentials defines the interface for transport security implementation.
type TransportCredentials interface {
	ClientHandshake(context.Context, string, net.Conn) (net.Conn, AuthInfo, error)
	ServerHandshake(net.Conn) (net.Conn, AuthInfo, error)
	Info() ProtocolInfo
	Clone() TransportCredentials
	OverrideServerName(string) error
}
