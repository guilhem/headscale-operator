package utils

import (
	"net"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func SliptHostPort(hostport string) (string, int, error) {
	host, sListenPort, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", 0, err
	}
	listenPort, err := strconv.Atoi(sListenPort)
	if err != nil {
		return "", 0, err
	}
	return host, listenPort, nil
}

func IgnoreNotFound(err error) error {
	status, ok := status.FromError(err)
	if !ok {
		return err
	}

	if (status.Code() == codes.NotFound) || strings.Contains(status.Message(), "not found") {
		return nil
	}

	return err
}
