package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

var (
    version string // Version number injected at build time

    ConfigFilePath string // Configuration file
    LogFilePath    string // Log file
    EnableDebug    bool   // Debug mode (verbose logs)

    ForwardPort = 443       // Target port to forward to
    cfg         configModel // Configuration file structure
)

// Configuration file structure
type configModel struct {
	ForwardRules   []string `yaml:"rules,omitempty"`
	ListenAddr     string   `yaml:"listen_addr,omitempty"`
	ListenAddrHTTP string   `yaml:"listen_addr_http,omitempty"`
	EnableSocks    bool     `yaml:"enable_socks5,omitempty"`
	SocksAddr      string   `yaml:"socks_addr,omitempty"`
	SocksUsername  string   `yaml:"socks_username,omitempty"`
	SocksPassword  string   `yaml:"socks_password,omitempty"`
	AllowAllHosts  bool     `yaml:"allow_all_hosts,omitempty"`
}

func init() {
	var printVersion bool
	var help = `
SNIProxy ` + version + `
https://github.com/frnimh/SNIProxy

Parameters:
    -c config.yaml
        Configuration file (default: config.yaml)
    -l sni.log
        Log file (default: none)
    -d
        Debug mode (default: off)
    -v
        Program version
    -h
        Help information
`
	flag.StringVar(&ConfigFilePath, "c", "config.yaml", "Configuration file")
	flag.StringVar(&LogFilePath, "l", "", "Log file")
	flag.BoolVar(&EnableDebug, "d", false, "Debug mode")
	flag.BoolVar(&printVersion, "v", false, "Program version")
	flag.Usage = func() { fmt.Print(help) }
	flag.Parse()
	if printVersion {
		fmt.Printf("frnimh/SNIProxy %s\n", version)
		os.Exit(0)
	}
}

func main() {
    data, err := os.ReadFile(ConfigFilePath) // Read configuration file
	if err != nil {
		serviceLogger(fmt.Sprintf("Failed to read configuration file: %v", err), 31, false)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		serviceLogger(fmt.Sprintf("Failed to parse configuration file: %v", err), 31, false)
		os.Exit(1)
	}
    if len(cfg.ForwardRules) <= 0 && !cfg.AllowAllHosts { // If rules is empty and allow_all_hosts is not true
		serviceLogger("The rules field in the configuration file cannot be empty (unless allow_all_hosts is set to true)!", 31, false)
		os.Exit(1)
	}
    if len(cfg.ListenAddr) <= 0 {
        cfg.ListenAddr = ":443" // If listen_addr is not specified, default to port 443
    }
    for _, rule := range cfg.ForwardRules { // Print all domains in rules
        serviceLogger(fmt.Sprintf("Loading rules: %v", rule), 32, false)
    }
	serviceLogger(fmt.Sprintf("Debug mode: %v", EnableDebug), 32, false)
	serviceLogger(fmt.Sprintf("Upstream proxy: %v", cfg.EnableSocks), 32, false)
	serviceLogger(fmt.Sprintf("Domain names: %v", cfg.AllowAllHosts), 32, false)

    startSniProxy() // Start SNI Proxy
}

// Start SNI Proxy
func startSniProxy() {
    _, cancel := context.WithCancel(context.Background())
    defer cancel()

    // If listen_addr_http is configured, start an HTTP listener (for 301 redirect to HTTPS)
    if cfg.ListenAddrHTTP != "" {
        listenerHTTP, err := net.Listen("tcp", cfg.ListenAddrHTTP)
        if err != nil {
            serviceLogger(fmt.Sprintf("Failed to listen (HTTP) on %s: %v", cfg.ListenAddrHTTP, err), 31, false)
            os.Exit(1)
        }
        serviceLogger(fmt.Sprintf("Listening on: %v", listenerHTTP.Addr()), 0, false)

        go func(listenerHTTP net.Listener) {
            defer listenerHTTP.Close()
            for {
                conn, err := listenerHTTP.Accept()
                if err != nil {
                    serviceLogger(fmt.Sprintf("Error while accepting (HTTP) connection: %v", err), 31, false)
                    continue
                }
                raddr := conn.RemoteAddr().(*net.TCPAddr)
                serviceLogger("Connection from: "+raddr.String(), 32, false)
                go handleHTTPRedirect(conn) // New connection; handle in a new goroutine
            }
        }(listenerHTTP)
    }

    // Start HTTPS listener
    listener, err := net.Listen("tcp", cfg.ListenAddr)
    if err != nil {
            serviceLogger(fmt.Sprintf("Failed to listen (HTTPS) on %s: %v", cfg.ListenAddrHTTP, err), 31, false)
        os.Exit(1)
    }
    serviceLogger(fmt.Sprintf("Listening on: %v", listener.Addr()), 0, false)

    go func(listener net.Listener) {
        defer listener.Close()
        for {
            conn, err := listener.Accept()
            if err != nil {
                serviceLogger(fmt.Sprintf("Error while accepting (HTTPS) connection: %v", err), 31, false)
                continue
            }
            raddr := conn.RemoteAddr().(*net.TCPAddr)
            serviceLogger("Connection from: "+raddr.String(), 32, false)
            go serve(conn, raddr.String()) // New connection; handle in a new goroutine
        }
    }(listener)

    // Exit when a termination signal is received
    ch := make(chan os.Signal, 2)
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    s := <-ch
    cancel()
    fmt.Printf("\nSignal received %s, Exit.\n", s)
}

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

// Extract SNI domain
func getSNIServerName(buf []byte) string {
    n := len(buf)
    if n < 5 {
        serviceLogger("Not a TLS handshake message", 31, true)
        return ""
    }

    // TLS record type
    if recordType(buf[0]) != recordTypeHandshake {
        serviceLogger("Not TLS", 31, true)
        return ""
    }

    // TLS major version
    if buf[1] != 3 {
        serviceLogger("TLS versions < 3 are not supported", 31, true)
        return ""
    }

    // payload length
    //l := int(buf[3])<<16 + int(buf[4])
    //log.Printf("length: %d, got: %d", l, n)

    // Handshake message type
    if buf[5] != typeClientHello {
        serviceLogger("Not a Client Hello message", 31, true)
        return ""
    }

    // Parse Client Hello message
    msg := &clientHelloMsg{}
    // Client Hello does not include the 5-byte TLS record header
    ret := msg.unmarshal(buf[5:n])
    if !ret {
        serviceLogger("Failed to parse Client Hello message", 31, true)
        return ""
    }
    return msg.serverName
}

// forward forwards initial data to dst and then pipes traffic between client and backend.
// It dials dst using GetDialer, writes data to backend, and connects streams via ioReflector.
func forward(conn net.Conn, data []byte, dst string, raddr string) {
    backend, err := GetDialer(cfg.EnableSocks).Dial("tcp", dst)
    if err != nil {
        serviceLogger(fmt.Sprintf("Unable to connect to backend (HTTPS): %v", err), 31, false)
        return
    }

	defer backend.Close()

    if _, err = backend.Write(data); err != nil {
        serviceLogger(fmt.Sprintf("Unable to forward to backend (HTTPS): %v", err), 31, false)
        return
    }
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    conChk := make(chan struct{})
    go ioReflector(ctx, backend, conn, false, conChk, raddr, dst)
    go ioReflector(ctx, conn, backend, true, conChk, raddr, dst)
    <-conChk
    // Cancel context to notify the other ioReflector to exit
	cancel()
}

// ioReflector 函数接收一个 io.WriteCloser 类型的写入对象 dst、一个 io.Reader 类型的读取对象 src、一个 bool 类型的 isToClient、一个 chan struct{} 类型的 conChk，以及两个字符串类型的 raddr 和 dsts
// 该函数使用 io.Copy 函数将 src 中读取到的数据流复制到 dst 中，然后将转发的字节数写入日志
// 最后，该函数关闭 dst 连接，并向 conChk 通道发送一个信号以表示连接已关闭。
func ioReflector(ctx context.Context, dst io.WriteCloser, src io.Reader, isToClient bool, conChk chan struct{}, raddr string, dsts string) {
	// 将 IO 流反映到另一个
	defer onDisconnect(dst, conChk)

	done := make(chan struct{})
	go func() {
		written, _ := io.Copy(dst, src)
		if isToClient {
			serviceLogger(fmt.Sprintf("[%v] -> [%v] %d bytes", dsts, raddr, written), 33, true)
		} else {
			serviceLogger(fmt.Sprintf("[%v] -> [%v] %d bytes", raddr, dsts, written), 33, true)
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		// 上下文取消，退出
	case <-done:
		// 复制完成，退出
	}
}

// onDisconnect 函数接收一个 io.WriteCloser 类型的写入对象 dst 和一个 chan struct{} 类型的 conChk
// 该函数在 dst 连接关闭时被调用，并向 conChk 通道发送一个信号以表示连接已关闭
func onDisconnect(dst io.WriteCloser, conChk chan struct{}) {
	// 关闭时 -> 强制断开另一对连接
	dst.Close()
	select {
	case conChk <- struct{}{}:
	default:
	}
}

// 解析 Client Hello 消息
func (m *clientHelloMsg) unmarshal(data []byte) bool {
	if len(data) < 42 {
		return false
	}
	m.raw = data
	m.vers = uint16(data[4])<<8 | uint16(data[5])
	m.random = data[6:38]
	sessionIDLen := int(data[38])
	if sessionIDLen > 32 || len(data) < 39+sessionIDLen {
		return false
	}
	m.sessionID = data[39 : 39+sessionIDLen]
	data = data[39+sessionIDLen:]
	if len(data) < 2 {
		return false
	}
	// cipherSuiteLen 是密码套件编号的字节数。由于是 uint16，因此数字必须是偶数
	cipherSuiteLen := int(data[0])<<8 | int(data[1])
	if cipherSuiteLen%2 == 1 || len(data) < 2+cipherSuiteLen {
		return false
	}
	numCipherSuites := cipherSuiteLen / 2
	m.cipherSuites = make([]uint16, numCipherSuites)
	for i := 0; i < numCipherSuites; i++ {
		m.cipherSuites[i] = uint16(data[2+2*i])<<8 | uint16(data[3+2*i])
		if m.cipherSuites[i] == scsvRenegotiation {
			m.secureRenegotiationSupported = true
		}
	}
	data = data[2+cipherSuiteLen:]
	if len(data) < 1 {
		return false
	}
	compressionMethodsLen := int(data[0])
	if len(data) < 1+compressionMethodsLen {
		return false
	}
	m.compressionMethods = data[1 : 1+compressionMethodsLen]

	data = data[1+compressionMethodsLen:]

	m.nextProtoNeg = false
	m.serverName = ""
	m.ocspStapling = false
	m.ticketSupported = false
	m.sessionTicket = nil
	m.signatureAndHashes = nil
	m.alpnProtocols = nil
	m.scts = false

	if len(data) == 0 {
		// ClientHello 后面可选地跟着扩展数据
		return true
	}
	if len(data) < 2 {
		return false
	}

	extensionsLength := int(data[0])<<8 | int(data[1])
	data = data[2:]
	if extensionsLength != len(data) {
		return false
	}

	for len(data) != 0 {
		if len(data) < 4 {
			return false
		}
		extension := uint16(data[0])<<8 | uint16(data[1])
		length := int(data[2])<<8 | int(data[3])
		data = data[4:]
		if len(data) < length {
			return false
		}

		switch extension {
		case extensionServerName:
			d := data[:length]
			if len(d) < 2 {
				return false
			}
			namesLen := int(d[0])<<8 | int(d[1])
			d = d[2:]
			if len(d) != namesLen {
				return false
			}
			for len(d) > 0 {
				if len(d) < 3 {
					return false
				}
				nameType := d[0]
				nameLen := int(d[1])<<8 | int(d[2])
				d = d[3:]
				if len(d) < nameLen {
					return false
				}
        				if nameType == 0 {
        					m.serverName = string(d[:nameLen])
        					// SNI value must not end with a dot
        					if strings.HasSuffix(m.serverName, ".") {
						return false
					}
					break
				}
				d = d[nameLen:]
			}
		case extensionNextProtoNeg:
			if length > 0 {
				return false
			}
			m.nextProtoNeg = true
		case extensionStatusRequest:
			m.ocspStapling = length > 0 && data[0] == statusTypeOCSP
		case extensionSupportedCurves:
			// http://tools.ietf.org/html/rfc4492#section-5.5.1
			if length < 2 {
				return false
			}
			l := int(data[0])<<8 | int(data[1])
			if l%2 == 1 || length != l+2 {
				return false
			}
			numCurves := l / 2
			m.supportedCurves = make([]CurveID, numCurves)
			d := data[2:]
			for i := 0; i < numCurves; i++ {
				m.supportedCurves[i] = CurveID(d[0])<<8 | CurveID(d[1])
				d = d[2:]
			}
		case extensionSupportedPoints:
			// http://tools.ietf.org/html/rfc4492#section-5.5.2
			if length < 1 {
				return false
			}
			l := int(data[0])
			if length != l+1 {
				return false
			}
			m.supportedPoints = make([]uint8, l)
			copy(m.supportedPoints, data[1:])
		case extensionSessionTicket:
			// http://tools.ietf.org/html/rfc5077#section-3.2
			m.ticketSupported = true
			m.sessionTicket = data[:length]
		case extensionSignatureAlgorithms:
			// https://tools.ietf.org/html/rfc5246#section-7.4.1.4.1
			if length < 2 || length&1 != 0 {
				return false
			}
			l := int(data[0])<<8 | int(data[1])
			if l != length-2 {
				return false
			}
			n := l / 2
			d := data[2:]
			m.signatureAndHashes = make([]signatureAndHash, n)
			for i := range m.signatureAndHashes {
				m.signatureAndHashes[i].hash = d[0]
				m.signatureAndHashes[i].signature = d[1]
				d = d[2:]
			}
		case extensionRenegotiationInfo:
			if length == 0 {
				return false
			}
			d := data[:length]
			l := int(d[0])
			d = d[1:]
			if l != len(d) {
				return false
			}

			m.secureRenegotiation = d
			m.secureRenegotiationSupported = true
		case extensionALPN:
			if length < 2 {
				return false
			}
			l := int(data[0])<<8 | int(data[1])
			if l != length-2 {
				return false
			}
			d := data[2:length]
			for len(d) != 0 {
				stringLen := int(d[0])
				d = d[1:]
				if stringLen == 0 || stringLen > len(d) {
					return false
				}
				m.alpnProtocols = append(m.alpnProtocols, string(d[:stringLen]))
				d = d[stringLen:]
			}
		case extensionSCT:
			m.scts = true
			if length != 0 {
				return false
			}
		}
		data = data[length:]
	}

	return true
}

// Log output
func serviceLogger(log string, color int, isDebug bool) {
	if isDebug && !EnableDebug {
		return
	}
	log = strings.Replace(log, "\n", "", -1)
	log = strings.Join([]string{time.Now().Format("2006/01/02 15:04:05"), " ", log}, "")
	if color == 0 {
		fmt.Printf("%s\n", log)
	} else {
		fmt.Printf("%c[1;0;%dm%s%c[0m\n", 0x1B, color, log, 0x1B)
	}
	if LogFilePath != "" {
		fd, _ := os.OpenFile(LogFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		fdContent := strings.Join([]string{log, "\n"}, "")
		buf := []byte(fdContent)
		fd.Write(buf)
		fd.Close()
	}
}
