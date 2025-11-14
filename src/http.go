package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

// Handle HTTP redirect to HTTPS
func handleHTTPRedirect(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		serviceLogger(fmt.Sprintf("Error reading (HTTP) connection request: %v", err), 31, false)
		return
	}
	data := string(buf[:n])
	lines := strings.Split(data, "\r\n")
	if len(lines) == 0 {
		serviceLogger("Not an HTTP/1.1 message", 31, true)
		return
	}
	// Parse request line to get path
	reqLine := strings.Fields(lines[0])
	if len(reqLine) < 2 {
		serviceLogger("Failed to parse HTTP/1.1 request", 31, true)
		return
	}
	path := reqLine[1]
	// Parse Host to get domain
	var host string
	for _, line := range lines[1:] {
		if strings.HasPrefix(strings.ToLower(line), "host:") {
			host = strings.TrimSpace(line[5:])
			break
		}
	}
	// If Host is missing or empty, do not redirect; just close
	if host == "" {
		serviceLogger("Domain not found (HTTP), ignoring...", 31, true)
		return
	}

	if cfg.AllowAllHosts { // If allow_all_hosts is true, no need to check HTTP domain
		sendHTTPRedirect(conn, host, path)
		return
	}
	for _, rule := range cfg.ForwardRules { // Iterate over whitelisted domains in Rules
		if strings.Contains(host, rule) { // If HTTP host contains a whitelisted domain (e.g., www.aa.com contains aa.com), redirect to HTTPS
			sendHTTPRedirect(conn, host, path)
			return
		} else {
			serviceLogger(fmt.Sprintf("Domain %s not in allowed domain list, ignoring (HTTP)...", host), 31, false)
		}
	}
}

// Send a 301 redirect response
func sendHTTPRedirect(conn net.Conn, host, path string) {
	// Build 301 redirect response
	location := "https://" + host + path
	resp := fmt.Sprintf("HTTP/1.1 301 Moved Permanently\r\nLocation: %s\r\nConnection: close\r\nContent-Length: 0\r\n\r\n", location)
	serviceLogger(fmt.Sprintf("Redirect to: %s", location), 34, false)
	// Send response
	_, err := conn.Write([]byte(resp))
	if err != nil {
		serviceLogger(fmt.Sprintf("Unable to forward to backend (HTTP): %v", err), 31, false)
	}
}

// Handle new HTTPS connection
func serve(conn net.Conn, raddr string) {
	defer conn.Close()

	buf := make([]byte, 2048)
	var fullHeader []byte
	const maxAttempts = 2
	attempts := 0

	for {
		n, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			serviceLogger(fmt.Sprintf("Error reading (HTTPS) connection request: %v", err), 31, false)
			return
		}
		fullHeader = append(fullHeader, buf[:n]...)

		if len(fullHeader) < 5 { // Determine whether it's a TLS handshake message
			serviceLogger("Not a TLS handshake message", 31, true)
			return
		} else if IsCompleteHandshake(fullHeader) { // Check if a complete TLS handshake message has been received
			break // If complete, break the loop
		}

		attempts++
		if attempts >= maxAttempts {
			serviceLogger("Handshake message receive timeout.", 31, false)
			return // Exceeded maximum wait attempts; exit
		}
	}
	if attempts > 0 { // If > 0, handshake was fragmented or lost and required retries
		serviceLogger("Complete handshake message received.", 32, true)
	}

	ServerName := getSNIServerName(fullHeader) // Extract SNI server name

	if ServerName == "" {
		serviceLogger("SNI domain not found (HTTPS), ignoring...", 31, true)
		return
	}

	if cfg.AllowAllHosts { // If allow_all_hosts is true, skip SNI domain checks
		serviceLogger(fmt.Sprintf("Forwarding target: %s:%d", ServerName, ForwardPort), 32, false)
		forward(conn, fullHeader, fmt.Sprintf("%s:%d", ServerName, ForwardPort), raddr)
		return
	}

	for _, rule := range cfg.ForwardRules { // Iterate over whitelisted domains in Rules
		if strings.Contains(ServerName, rule) { // If SNI contains a whitelisted domain (e.g., www.aa.com contains aa.com), forward connection
			serviceLogger(fmt.Sprintf("Forwarding target: %s:%d", ServerName, ForwardPort), 32, false)
			forward(conn, fullHeader, fmt.Sprintf("%s:%d", ServerName, ForwardPort), raddr)
		} else {
			serviceLogger(fmt.Sprintf("Domain %s not in allowed domain list, ignoring (HTTPS)...", ServerName), 31, false)
		}
	}
}

// Determine whether the handshake message is complete
func IsCompleteHandshake(header []byte) bool {
	Length := binary.BigEndian.Uint16(header[3:5])
	return len(header) >= int(Length)+5
}

