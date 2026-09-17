package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// dns.go ——— 解决 Android 上 Go 纯解析器读取损坏 /etc/resolv.conf 的问题。
//
// 背景：Android 的真实 DNS 由 netd 管理，/etc/resolv.conf 往往缺失或指向
// 未监听的环回地址(如 [::1]:53)，导致 Go 默认解析器失败。
//
// 方案：完全自建 DNS 客户端，绕过 /etc/resolv.conf 与 net.## 内部解析器。
// 逐服务器发送标准 DNS 查询(AAAA 优先，兼容仅有 IPv6 的网络)，自己解析应答，
// 一个服务器超时/失败就换下一个，实现可靠的逐服务器重试。
//
// resolveHost → dialContext 供 http.Transport 使用：
// 解析出 IP 后直接按 IP 建连，TLS 握手仍用原始域名做 SNI/证书校验。

var (
	dnsMu      sync.RWMutex
	dnsServers []string
)

// loadSystemDNS 收集 DNS 服务器列表 = 系统下发的 + 公共 DNS（始终追加，保证兜底）。
func loadSystemDNS() {
	list := make([]string, 0, 8)
	for _, k := range []string{"net.dns1", "net.dns2", "net.dns3", "net.dns4"} {
		if b, err := exec.Command("getprop", k).Output(); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				list = append(list, net.JoinHostPort(s, "53"))
			}
		}
	}
	// 公共 DNS 始终追加（含 IPv6）
	list = append(list,
		"[2400:3200::1]:53", // 阿里 IPv6 DNS
		"[2400:da00::6666]:53",
		"223.5.5.5:53", // 阿里 DNS
		"119.29.29.29:53",
		"8.8.8.8:53",
		"1.1.1.1:53",
	)
	dnsMu.Lock()
	dnsServers = dedup(list)
	dnsMu.Unlock()
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func dnsServersSnapshot() []string {
	dnsMu.RLock()
	defer dnsMu.RUnlock()
	out := make([]string, len(dnsServers))
	copy(out, dnsServers)
	return out
}

// ---- DNS 报文构建/解析（纯 Go，不依赖系统解析器）----

func dnsEncodeName(host string) []byte {
	var b []byte
	for _, l := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if l == "" {
			continue
		}
		b = append(b, byte(len(l)))
		b = append(b, l...)
	}
	return append(b, 0)
}

func dnsBuildQuery(id uint16, host string, qtype uint16) []byte {
	b := make([]byte, 0, 40)
	// header
	b = append(b, byte(id>>8), byte(id)) // ID
	b = append(b, 0x01, 0x00)            // flags: RD
	b = append(b, 0, 1)                  // QDCOUNT=1
	b = append(b, 0, 0, 0, 0, 0, 0)      // AN NS AR = 0
	// question
	b = append(b, dnsEncodeName(host)...)
	b = append(b, byte(qtype>>8), byte(qtype)) // QTYPE
	b = append(b, 0, 1)                        // QCLASS IN
	return b
}

// dnsSkipName 跳过报文中的 NAME（支持压缩指针）。
func dnsSkipName(pkt []byte, start int) int {
	idx := start
	for {
		if idx >= len(pkt) {
			return start
		}
		b := pkt[idx]
		if b&0xC0 == 0xC0 { // 压缩指针，占 2 字节
			return idx + 2
		}
		if b == 0 {
			return idx + 1
		}
		idx += 1 + int(b)
	}
}

// dnsParseResponse 解析应答，返回 (v4, v6)。
func dnsParseResponse(pkt []byte, id uint16) (net.IP, net.IP) {
	if len(pkt) < 12 || pkt[0] != byte(id>>8) || pkt[1] != byte(id&0xff) {
		return nil, nil
	}
	ancount := int(pkt[6])<<8 | int(pkt[7])
	idx := 12
	idx = dnsSkipName(pkt, idx)
	idx += 4 // QTYPE + QCLASS
	var v4, v6 net.IP
	for a := 0; a < ancount; a++ {
		idx = dnsSkipName(pkt, idx) // answer name
		if idx+10 > len(pkt) {
			break
		}
		rtype := int(pkt[idx])<<8 | int(pkt[idx+1])
		rdlen := int(pkt[idx+8])<<8 | int(pkt[idx+9])
		rd := idx + 10
		if rd+rdlen > len(pkt) {
			break
		}
		if rtype == 1 && rdlen == 4 {
			v4 = net.IP(pkt[rd : rd+4]).To4()
		} else if rtype == 28 && rdlen == 16 {
			v6 = net.IP(pkt[rd : rd+16])
		}
		idx = rd + rdlen
	}
	return v4, v6
}

// rawLookup 向指定 DNS 服务器查询 host，返回 (v4, v6)。v6 优先。
func rawLookup(host string, server string, deadline time.Time) (net.IP, net.IP, error) {
	// 依次尝试 AAAA、A（面板可能仅有 IPv6）
	for _, qtype := range []uint16{28, 1} {
		id := uint16(rand.Uint32())
		payload := dnsBuildQuery(id, host, qtype)

		conn, err := net.DialTimeout("udp", server, time.Until(deadline))
		if err != nil {
			return nil, nil, err
		}
		conn.SetDeadline(deadline)
		if _, err := conn.Write(payload); err != nil {
			conn.Close()
			return nil, nil, err
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		conn.Close()
		if err != nil {
			continue // 此类型超时，尝试下一个 qtype
		}
		v4, v6 := dnsParseResponse(buf[:n], id)
		if qtype == 28 && v6 != nil {
			return nil, v6, nil
		}
		if qtype == 1 && v4 != nil {
			return v4, nil, nil
		}
	}
	return nil, nil, errors.New("DNS 无应答/无地址")
}

// resolveHost 依次尝试所有 DNS 服务器，返回 IPv6(优先)或 IPv4。
func resolveHost(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}
	if len(dnsServersSnapshot()) == 0 {
		loadSystemDNS()
	}
	var lastErr error = errors.New("无可用 DNS 服务器")
	for _, srv := range dnsServersSnapshot() {
		// 每个服务器独立短超时，防止单个挂起的 DNS 拖垮整体
		deadline := time.Now().Add(4 * time.Second)
		v4, v6, err := rawLookup(host, srv, deadline)
		if err == nil {
			if v6 != nil {
				return v6, nil
			}
			if v4 != nil {
				return v4, nil
			}
		}
		lastErr = err
	}
	return nil, fmt.Errorf("DNS 解析失败: %w", lastErr)
}

// dialContext 是 http.Transport 的 DialContext：解析出 IP 后直接按 IP 建连。
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip, err := resolveHost(ctx, host)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: 8 * time.Second}
	return d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}
