package main

import (
	"net"

	"golang.org/x/net/proxy"
)

func GetDialer(isSocks5 bool) proxy.Dialer {
    if !isSocks5 { // If SOCKS5 proxy is not enabled in the config, return direct dialer
        return &net.Dialer{}
    }

    // If proxy username or password is configured, create an auth object.
    // If both are empty, auth remains nil and proxy.SOCKS5 will not use authentication.
    var auth *proxy.Auth
    if cfg.SocksUsername != "" || cfg.SocksPassword != "" {
        auth = &proxy.Auth{
            User:     cfg.SocksUsername,
            Password: cfg.SocksPassword,
        }
    }
    proxyDialer, err := proxy.SOCKS5("tcp", cfg.SocksAddr, auth, proxy.Direct)
    if err != nil { // On error, fall back to direct dialer
        return &net.Dialer{}
    }
    return proxyDialer
}
