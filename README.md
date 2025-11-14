# frnimh/SNI-Proxy

[![Go Version](https://img.shields.io/github/go-mod/go-version/frnimh/SNI-Proxy.svg?style=flat-square&label=Go&color=00ADD8&logo=go)](https://github.com/frnimh/SNI-Proxy/)
[![Release Version](https://img.shields.io/github/v/release/frnimh/SNI-Proxy.svg?style=flat-square&label=Release&color=00ADD8&logo=github)](https://github.com/frnimh/SNI-Proxy/releases/latest)
[![GitHub license](https://img.shields.io/github/license/frnimh/SNI-Proxy.svg?style=flat-square&label=License&color=00ADD8&logo=github)](https://github.com/frnimh/SNI-Proxy/)
[![GitHub Star](https://img.shields.io/github/stars/frnimh/SNI-Proxy.svg?style=flat-square&label=Star&color=00ADD8&logo=github)](https://github.com/frnimh/SNI-Proxy/)
[![GitHub Fork](https://img.shields.io/github/forks/frnimh/SNI-Proxy.svg?style=flat-square&label=Fork&color=00ADD8&logo=github)](https://github.com/frnimh/SNI-Proxy/)

🧷 A simple SNIProxy for personal use, shared for those with similar needs. If it’s useful, please **give it a ⭐ for support~**

SNIProxy is a tool that automatically forwards incoming traffic to the source server of a domain based on the domain name (SNI). It’s commonly used for **load balancing** across multiple website servers. Since it obtains the target domain through plaintext SNI, it **does not require SSL decryption and re-encryption**, greatly improving forwarding speed and efficiency.

> [!IMPORTANT]
> Note! SNIProxy only functions as a **port forwarder and load balancer**. Simply put, all data received by SNIProxy is **forwarded intact** to the target source server (including plaintext SNI domain information, visible to any third party such as the firewall). Therefore, SNIProxy **cannot be used for bypassing firewalls** (otherwise it’s pointless!). Firewalls can already block domains (SNI) directly.


## Software Introduction

- **Supports** all platforms and systems (Go language feature)
- **Supports** Socks5 upstream proxy (e.g., can use WARP so SNIProxy’s exit IP is Cloudflare’s)
- **Supports** forwarding all domains or only specified domains (including the domain itself and all subdomains)
- **Supports** listening and forwarding IPv4 and IPv6 traffic separately or simultaneously
- **Supports** redirecting HTTP to HTTPS
- **Supports** automatically forwarding traffic to the source server of a domain based on plaintext SNI (domain/hostname), without encrypting/decrypting traffic or needing SSL keys/certificates

> [!WARNING]
> Note! SNIProxy is personally written for my own use. Its **reliability and stability** are **not comparable to professional software (like Nginx or HAProxy)**. Therefore, it is **not recommended for production environments**, and this project assumes no responsibility for any losses under GPL-3.0 (use at your own risk).

> [!NOTE]
> Also, web traffic is constantly evolving. The popular **HTTP/2** can multiplex multiple hostnames over a single TCP stream, making it **impossible for SNIProxy to correctly forward traffic by hostname**. **HTTP/3 (QUIC)** uses UDP instead of TCP. **SNIProxy does not support these protocols** and will not add support, as it would make the project overly complex (this was written just for personal convenience, the code is only a few hundred lines).

---

### SNIProxy Workflow

1. Parse the TLS/SSL handshake message from the incoming connection to get the **SNI domain**.
2. Check if the domain is in the allowed list (or if `allow_all_hosts` is enabled). If not, terminate the connection; otherwise, continue.
3. Use system DNS to resolve the SNI domain to an IP address (the source server IP of that domain).
4. Forward the received data intact to the source **IP:443**, establishing a “bridge” between the client and the source server for continuous bidirectional data transfer (TCP relay/port forwarding).

```javascript
// Point example.com to the SNIProxy server IP, then:
Access example.com <=> SNIProxy(parse SNI for target domain) <=> Source(example.com)

// More detailed workflow:
Access example.com <=> SNIProxy [ Parse SNI => example.com <=> Check if domain is allowed <=> DNS resolve domain to IP ] <=> Source(example.com)

// Multiple SNIProxy servers can be used for load balancing:
// DNS can point visitors to different server IPs to achieve load distribution.
// DNS can also direct visitors from different regions/ISPs to the nearest, fastest server IP for improved access speed and user experience.

// If SNIProxy uses an upstream proxy:
Access example.com <=> SNIProxy <=> Socks5(parse SNI for target domain) <=> Source(example.com)
```

> [!TIP]
> SNIProxy is essentially a **port forwarding (relay)** tool, but unlike traditional port forwarding which can only specify a **fixed target IP**, SNIProxy can resolve the incoming domain via DNS to obtain a **flexible target IP** (different domains can route to different target IPs, **coexisting simultaneously without interference**).

---

## Usage

<details>
<summary><code><strong>「 Click to view Linux usage example 」</strong></code></summary>

---

The following commands are examples; check [**Releases**](https://github.com/frnimh/SNI-Proxy/releases) for version numbers and filenames.

```bash
# If using for the first time, it's recommended to create a new folder (skip this step for future updates)
mkdir sniproxy

# Enter the folder (for future updates, just repeat the download and extract steps)
cd sniproxy

# Download the sniproxy archive (replace [version] and [filename] as needed)
wget -N https://github.com/frnimh/SNI-Proxy/releases/download/v1.0.6/sniproxy_linux_amd64.tar.gz

# If download fails, try removing the -N option (if updating, remember to delete the old archive rm sniproxy_linux_amd64.tar.gz)

# Extract (no need to delete old files; it will overwrite automatically; replace filename as needed)
tar -zxf sniproxy_linux_amd64.tar.gz

# Make executable
chmod +x sniproxy

# Edit configuration file (customize content according to config file instructions, save with Ctrl+X then press Enter twice)
nano config.yaml

# Run (without parameters)
./sniproxy

# Run (with configuration)
./sniproxy -c "config.yaml"

# Run in background
nohup ./sniproxy -c "config.yaml" > "sni.log" 2>&1 &
```

> Additionally, it is strongly recommended to increase the [system file descriptor limit](https://github.com/frnimh/SNI-Proxy#-increase-system-file-descriptor-limit-to-avoid-too-many-open-files-error) to avoid encountering **too many open files** errors.

> Also, if you want **startup at boot, daemon mode (auto-restart on crash), background execution, and easy management**, you can [register it as a system service](https://github.com/frnimh/SNI-Proxy).

</details>

---

<details>
<summary><code><strong>「 Click to view Windows usage example 」</strong></code></summary>

---

### Download

Download the precompiled executable and extract:

1. [GitHub Releases](https://github.com/frnimh/SNI-Proxy/releases)

### Configuration

Right-click the `config.yaml` file → Open with → Notepad.

Customize according to the [config file instructions](https://github.com/frnimh/SNI-Proxy#-configuration-file-instructions-configyaml) and save.

### Run

Double-click `sniproxy.exe` or run in CMD by navigating to the software directory:

```powershell
# In CMD, navigate to the extracted sniproxy program directory (remember to modify the path below)
cd /d C:\xxx\sniproxy

# Run (without parameters)
sniproxy.exe

# Run (with parameters example)
sniproxy.exe -c "config.yaml"
```


```bash
# Navigate to the directory containing the sniproxy archive via terminal (remember to modify the path below)
cd /xxx/xxx

# Extract (no need to delete old files, it will overwrite directly; replace filenames as needed)
tar -zxf sniproxy_linux_amd64.tar.gz

# Grant execute permission
chmod a+x sniproxy

# Edit the configuration file (customize according to the config file instructions below and save by pressing Ctrl+X then Enter twice)
nano config.yaml

# Run (without parameters)
./sniproxy

# Run (with parameters example)
./sniproxy -c "config.yaml"
```

</details>

---

```text
home@srv:~# ./sniproxy -h

SNIProxy vX.X.X
https://github.com/frnimh/SNI-Proxy

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
```

---

## Other Notes

#### Configuration File (config.yaml)

<details>
<summary><code><strong>「 Click to expand to view content 」</strong></code></summary>

---

> **Note:** The configuration file uses YAML format, which relies on indentation (number of spaces at the beginning of each line) to determine hierarchy. If unsure, follow the format in the default configuration file. Lines starting with `#` are comments and will be ignored. Unnecessary configurations can be commented out.

Currently, there are only a few configuration options in the file:


```yaml
# Listening port (quotes are required). Common examples:
# ":443"            Only specify port, listens on all IPv4+IPv6 addresses on port 443
# "0.0.0.0:443"     Listens on all IPv4 addresses on port 443
# "127.0.0.1:443"   Listens on local IPv4 only (accessible only locally)
# "[::]:443"        Listens on all IPv6 addresses on port 443
# "[::1]:443"       Listens on local IPv6 only (accessible only locally)
# You can also replace the IP with your public IP, so it can only be accessed from that IP
listen_addr: ":443"

# Optional: Redirect incoming HTTP to HTTPS (fixed port 443, unrelated to above setting)
# If not set, HTTP connections will not be listened to or handled
listen_addr_http: ":80"

# Optional: Enable Socks5 upstream proxy
# (Before: client <=> SNIProxy <=> target server)
# (After:  client <=> SNIProxy <=> Socks5 <=> target server)
# (Example: using WARP: client <=> SNIProxy <=> WARP <=> target server)
enable_socks5: true
# Optional: Socks5 proxy address
socks_addr: 127.0.0.1:40000
# Optional: Socks5 username and password (if required)
socks_username: admin
socks_password: abc123

# Either/or: Allow all hosts (if enabled, rules list below is ignored)
allow_all_hosts: true

# Either/or: Only allow specified hosts (choose one with allow_all_hosts)
# Specifying domains allows the domain itself and all subdomains to access the service
# Example: √ = allowed, × = blocked
# Non-allowed domain access will be logged in red to help check leaks or scans
# 2025/06/16 10:44:55 Domain xxx not in allowed rules, ignored (http/https noted)...
rules:
  - example.com      # example.com √ , a.example.com √ , a.a.example.com √
  - b.example2.com   # example2.com × , b.example2.com √ , c.b.example2.com √
```

---

Some examples:

1. Allow access to all domains

```yaml
listen_addr: ":443"
listen_addr_http: ":80"

allow_all_hosts: true
```

> Note: Enabling `allow_all_hosts` may expose your server to scanning and misuse by others. Please be aware!
> It is recommended to apply some restrictions, such as using IPv6 only (`"[::]:443"`) or limiting access to port 443 via firewall.

2. Allow only specified domains

```yaml
listen_addr: ":443"
listen_addr_http: ":80"

rules:
  - example.com
  - b.example2.com
```

3. Allow all domains + Enable upstream proxy

```yaml
listen_addr: ":443"
listen_addr_http: ":80"

enable_socks5: true
socks_addr: 127.0.0.1:40000

allow_all_hosts: true
```

4. Only allow specified domains + Enable upstream proxy + Proxy username and password

```yaml
listen_addr: ":443"
listen_addr_http: ":80"

enable_socks5: true
socks_addr: 127.0.0.1:40000
socks_username: admin
socks_password: abc123

rules:
  - example.com
  - b.example2.com
```

</details>

---

#### Linux: Configure as a system service (systemd)

<details>
<summary><code><strong>"Click to expand and view"</strong></code></summary>

---

Create an empty system service file named **sniproxy**:

```bash
nano /etc/systemd/system/sniproxy.service
```

After modifying the following content (the program path and parameters after `ExecStart=`), paste it into the file:

```ini
[Unit]
Description=SNI Proxy
After=network.target

[Service]
ExecStart=/home/sniproxy/sniproxy -c /home/sniproxy/config.yaml -l /home/sniproxy/sni.log
Restart=on-failure

[Install]
WantedBy=multi-user.target
```
> If you need detailed logs (debug mode), you can add the `-d` parameter at the end of the `ExecStart=...` line (make sure to add a space before it).

> `Restart=on-failure` means that if the program exits abnormally, it will automatically restart, functioning as a daemon.

Enable **sniproxy** to start on boot and start it immediately:

```bash
# Enable the system service and allow it to start on boot
systemctl enable sniproxy

# Start immediately
systemctl start sniproxy
```

Other commands you might need:

```bash
# Stop the service
systemctl stop sniproxy

# Restart the service
systemctl restart sniproxy

# Check the service status
systemctl status sniproxy

# View the full log
cat /home/sniproxy/sni.log

# Follow the log in real-time (displays the latest log entries as they appear)
tail -f /home/sniproxy/sni.log

# If you modified the /etc/systemd/system/sniproxy.service file, you need to reload the systemd configuration before starting/restarting the sniproxy service
systemctl daemon-reload
```
</details>

---

#### IPv4 vs IPv6 preference when forwarding

<details>
<summary><code><strong>「 Click to expand 」</strong></code></summary>

---

First, it's important to understand that whether SNIProxy forwards traffic to the target domain server via IPv4 or IPv6 is ***independent*** of whether you access the SNIProxy service via IPv4 or IPv6. The two stages, `"You ↔ SNIProxy"` and `"SNIProxy ↔ Target Server"`, are **separate and do not affect each other**.

SNIProxy relies on the system DNS service for DNS resolution. Therefore, whether it forwards traffic via IPv4 or IPv6 to the target server depends on:

1. Whether the server running SNIProxy has an **IPv4 or IPv6 address** (or both, i.e., dual-stack server).
2. The system DNS priority configured on the server (IPv4-first or IPv6-first, usually default is IPv6-first).
3. Whether the target domain has **IPv4 or IPv6 records** (i.e., A and AAAA records).

In short, if your server has an IPv6 address and the system defaults to IPv6-first, then regardless of whether you access SNIProxy via IPv4 or IPv6, SNIProxy will forward traffic via IPv6 to the target domain server if the domain has an IPv6 record.

```javascript
Access example.com <=IPv4=> SNIProxy <=prefers IPv6=> System DNS resolves the domain's IP address <=IPv6=> Origin server (example.com)

Access example.com <=IPv6=> SNIProxy <=prefers IPv6=> System DNS resolves the domain's IP address <=IPv6=> Origin server (example.com)
```

If the target domain only has an IPv6 address, and your local machine only has an IPv4 address, but your server has both IPv4 and IPv6, you can access SNIProxy via IPv4, and then SNIProxy will forward the traffic to the target domain's origin server via IPv6.

```javascript
Access example.com (IPv4 only) <=IPv4=> SNIProxy (supports IPv4+IPv6) <=IPv6=> Origin server (example.com IPv6 only)
```

---

The IPv4/IPv6 priority of the system DNS service can be adjusted (the following changes the default from IPv6-preferred to IPv4-preferred):

Open and edit the file (you can also use vim to edit):

```bash
nano /etc/gai.conf
```


Find the following line:

```yaml
#precedence ::ffff:0:0/96  100
```
Remove the `#` comment character at the beginning of the line to make it:

> If you can't find it, you can directly add the following line at the end of the file.

```yaml
precedence ::ffff:0:0/96  100
```
Press `Ctrl+O` and Enter to save the file, then press `Ctrl+X` to exit the nano editor.

Now, try `ping` any domain that has both IPv4 and IPv6 addresses to check if it resolves to an IPv4 address.

> Additionally, after changing the system DNS priority, you may need to clear the server's DNS cache and restart the SNIProxy service.

</details>

---

#### Increase System File Descriptor Limit (to avoid "too many open files" error)

<details>
<summary><code><strong>「 Click to Expand 」</strong></code></summary>

---

On Linux systems, some users may encounter the following error in the logs:

```
Error accepting connection request: accept tcp [::]:443: accept4: too many open files
```

This happens because the system has run out of file descriptors (default is 1024). Increasing the system file descriptor limit can effectively alleviate this issue (it won’t completely prevent it, because theoretically, if enough files or connections are opened, it will eventually be exhausted. In general, this adjustment is necessary whether acting as a proxy or running a website).

- **Temporarily increase** (resets to 1024 after reboot):

```bash
ulimit -n 65535
```

- **Permanently increase** (remains 65535 after reboot; manually closing files will reduce the count):

```bash
echo "* soft nofile 65535
* hard nofile 65535
root soft nofile 65535
root hard nofile 65535" >> /etc/security/limits.conf
```


After running the above commands, you need to restart SNIProxy for the changes to take effect. If it still doesn’t work, try rebooting the system:

```bash
systemctl restart sniproxy
```

</details>

---

## Manual Compilation

<details>
<summary><code><strong>「 Click to Expand 」</strong></code></summary>

---

For convenience, I wrote the version number into the `version` variable in the code during compilation.
Therefore, when you manually compile, you need to add the `-ldflags` parameter after the `go build` command to specify the version number:

```bash
go build -ldflags "-s -w -X main.version=v1.0.6"
# Run this command in the SNIProxy directory via the command line (e.g., CMD, Bat script)
# to compile a binary that can run on the same system, bitness, and architecture as your current device
# (Go will automatically detect your system's bitness and architecture) with the version number set to v1.0.6
```

If you want to cross-compile on 64-bit Windows for a different OS/architecture/bitness, set the GOOS and GOARCH variables.

For example, on Windows, compile a 64-bit Linux AMD binary:

```bat
SET GOOS=linux
SET GOARCH=amd64
go build -ldflags "-s -w -X main.version=v1.0.6"
```


For example, on a Linux system, compile a binary program for **Windows OS, AMD architecture, 32-bit**:

```bash
GOOS=windows
GOARCH=386
go build -ldflags "-s -w -X main.version=v1.0.6"
```

> You can run `go tool dist list` to see which OS/architecture combinations your Go toolchain supports.

---

Of course, for convenience when compiling in batches, I usually define a variable for the version number, so subsequent builds can just reference that variable.
Also, when compiling in batches, you need to place outputs in separate folders (or use different filenames), which requires the `-o` parameter to specify the output path.

```bat
:: On Windows:
SET version=v1.0.6
SET GOOS=linux
SET GOARCH=amd64
go build -o Releases\sniproxy_linux_amd64\sniproxy -ldflags "-s -w -X main.version=%version%"
```

```bash
# On Linux:
version=v1.0.6
GOOS=windows
GOARCH=386
go build -o Releases/sniproxy_windows_386/sniproxy.exe -ldflags "-s -w -X main.version=${version}"
```

</details>

---

## License

The GPL-3.0 License.
