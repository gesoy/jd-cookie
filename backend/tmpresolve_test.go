package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

// 最小 DNS 应答：把查询的 question 原样回显，并附带一条与 QTYPE 一致的记录
func startFakeDNS(t *testing.T) net.Addr {
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 512)
		for {
			n, peer, err := ln.ReadFrom(buf)
			if err != nil {
				return
			}
			req := buf[:n]
			if n < 12 {
				continue
			}
			// 解析 QNAME 结束位
			idx := 12
			qtIdx := -1
			for idx < n {
				ln := int(req[idx])
				idx++
				if ln == 0 {
					qtIdx = idx
					break
				}
				idx += ln
			}
			if qtIdx < 0 || qtIdx+4 > n {
				continue
			}
			qtype := binary.BigEndian.Uint16(req[qtIdx : qtIdx+2])
			question := req[12:n]

			var rdata []byte
			var tl uint16
			switch qtype {
			case 1: // A
				rdata = net.IPv4(127, 0, 0, 1).To4()
				tl = 1
			case 28: // AAAA
				rdata = net.ParseIP("::1").To16()
				tl = 28
			default:
				continue
			}
			resp := make([]byte, 0, 64)
			hdr := []byte{0, 0, 0x81, 0x80, 0, 1, 0, 1, 0, 0, 0, 0}
			copy(hdr[0:2], req[0:2]) // 回显 transaction ID
			resp = append(resp, hdr...)
			resp = append(resp, question...)
			resp = append(resp, 0xC0, 0x0C)                       // pointer to QNAME
			resp = append(resp, byte(tl>>8), byte(tl), 0, 1)       // type, class IN
			resp = append(resp, 0, 0, 0, 60)                       // TTL 60
			resp = append(resp, byte(len(rdata)>>8), byte(len(rdata))) // rdlength
			resp = append(resp, rdata...)
			ln.WriteTo(resp, peer)
		}
	}()
	return ln.LocalAddr()
}

// 静默 DNS 服务器：收到即丢弃，不回复（模拟"连得上但无响应"）
func startSilentDNS(t *testing.T) net.Addr {
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 512)
		for {
			if _, _, err := ln.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	return ln.LocalAddr()
}

func TestResolveHostSkipsDeadDNS(t *testing.T) {
	good := startFakeDNS(t)
	silent := startSilentDNS(t)
	// 显式注入 DNS 列表：第一个是"无响应"的死服务器，第二个是正常服务器
	dnsServers = []string{silent.String(), good.String()}
	fmt.Println("DNS 列表:", dnsServers)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 单独验证 good 服务器本身可解析
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, good.String())
	}}
	gctx, gcancel := context.WithTimeout(context.Background(), 5*time.Second)
	ips, gerr := r.LookupIP(gctx, "ip", "example.invalid")
	gcancel()
	fmt.Println("good 服务器直查:", ips, "err=", gerr)

	ip, err := resolveHost(ctx, "example.invalid")
	if err != nil {
		t.Fatalf("resolveHost 失败: %v", err)
	}
	fmt.Printf("解析结果: %v (期望 ::1，优先 IPv6)
", ip)
	if !ip.Equal(net.ParseIP("::1")) {
		t.Fatalf("期望 ::1，得到 %v", ip)
	}
}
