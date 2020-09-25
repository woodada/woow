package woow

import (
    "bufio"
    "bytes"
    "encoding/base64"
    "fmt"
    "github.com/pkg/errors"
    "io"
    "net"
    "net/http"
    "net/url"

    "strings"
    "time"

    "github.com/woodada/ioutil/smartnet"
)

func (this *Server) proxyHTTP(c net.Conn, callback smartnet.StopCallBackFunc) {
    // 函数包装使变量及时释放
    f := func(c net.Conn) (net.Conn, error) {
        // 设定取第一个请求必须在3秒内接收完成
        err := c.SetReadDeadline(time.Now().Add(3 * time.Second))
        if err != nil {
            return nil, errors.WithMessage(err, "c.SetReadDeadline")
        }

        var buff [2000]byte // TODO reuse buffer
        headerLen, buffLen, err := readAtLeastDoubleBreakLine(c, buff[:])
        if err != nil {
            return nil, errors.WithMessage(err, "readAtLeastDoubleBreakLine")
        }

        this.logger.Printf("[request raw]%s", string(buff[0:buffLen]))
        // 取第一个请求 https 或者http2必须会发CONNECT http明文的不一定 但一定能Read成功
        req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buff[:buffLen])))
        if err != nil {
            return nil, err
        }

        // 拿到需要连接的远程地址
        addr, err := getRemoteAddress(req.Host)
        if err != nil {
            return nil, errors.WithMessage(err, "getRemoteAddress("+req.Host+")")
        }
        this.logger.Printf("remote addr %s", addr)

        if this.accounts.Size() > 0 {
            username, password, k, err := getProxyAuth(req)
            if err != nil {
                return nil, errors.WithMessage(err, "getProxyAuth")
            }

            code := this.accounts.Compare(username, password)
            if code != 0 {
                this.logger.Printf("auth fail! request user: %s pass: %s ", username, password)
                c.SetWriteDeadline(time.Now().Add(1 * time.Second))
                c.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
                return nil, fmt.Errorf("Forbidden")
            }

            // 删除鉴权信息，不往后透传
            lp := bytes.Index(buff[:headerLen], []byte("\r\n"+k))
            if lp <= 0 {
                return nil, fmt.Errorf("[BUG]index proxy header! %s %s", k, string(buff[:headerLen]))
            }

            lp += 2

            var rp int
            var find bool
            for i := lp; i+1 < headerLen; i++ {
                if buff[i] == '\r' && buff[i+1] == '\n' {
                    find = true
                    rp = i + 1
                    break
                }
            }
            if !find {
                return nil, fmt.Errorf("[BUG]find next line! %s", string(buff[:headerLen]))
            }

            copy(buff[lp:buffLen], buff[rp+1:buffLen])
            buffLen = buffLen - (rp - lp + 1)
            headerLen = headerLen - (rp - lp + 1)
            this.logger.Printf("[request replace]%s", string(buff[0:buffLen]))
        }

        if req.Method == http.MethodConnect {
            err = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
            if err != nil {
                return nil, err
            }
            _, err = c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
            if err != nil {
                return nil, errors.WithMessage(err, "connect response fail")
            }
        }

        remote, err := net.DialTimeout("tcp", addr, 3*time.Second) // TODO connect 超时时间
        if err != nil {
            return remote, errors.WithMessage(err, addr)
        }

        if req.Method != http.MethodConnect {
            // TODO 指定本地网卡
            err = remote.SetWriteDeadline(time.Now().Add(3 * time.Second))
            if err != nil {
                return remote, errors.WithMessage(err, "remote.SetWriteDeadline")
            }
            n, err := remote.Write(buff[0:buffLen])
            if err != nil {
                return remote, errors.WithMessage(err, "remote.Write")
            }
            if n != buffLen {
                return remote, fmt.Errorf("incomplete send %d expect %d", n, buffLen)
            }
        }
        return remote, nil
    }

    remote, err := f(c)
    if err != nil {
        c.Close()
        if remote != nil {
            remote.Close()
        }
        callback(err, nil)
        return
    }

    cfg := smartnet.DefaultTunnelConfig()
    cfg.StopCallback = callback
    cfg.B2AConfig.WriteTimeout = 10 * time.Second
    cfg.B2AConfig.ReadTimeout = this.config.TunnelTimeout
    cfg.A2BConfig.ReadTimeout = this.config.TunnelTimeout
    cfg.A2BConfig.WriteTimeout = 10 * time.Second
    smartnet.NewRateTunnel(c, remote, cfg)
}

func getProxyAuth(r *http.Request) (user, passwd, key string, err error) {
    key = "Proxy-Authorization"
    s := strings.SplitN(r.Header.Get(key), " ", 2)
    if len(s) != 2 {
        key = "IP-Proxy-Authorization" // 自定义的Header
        s = strings.SplitN(r.Header.Get(key), " ", 2)
        if len(s) != 2 {
            err = fmt.Errorf("鉴权header不存在")
            return
        }
    }

    b, err := base64.StdEncoding.DecodeString(s[1])
    if err != nil {
        err = errors.WithMessage(err, "base64.StdEncoding.DecodeString")
        return
    }

    pair := strings.SplitN(string(b), ":", 2)
    if len(pair) != 2 {
        err = fmt.Errorf("格式不正确")
        return
    }
    user = pair[0]
    passwd = pair[1]
    return
}

var doubleBreakLine = []byte{'\r', '\n', '\r', '\n'}

// 从c中至少读取下两个连在一起的换行符 限制不超出buff空间
// 依次返回 \r\n\r\n后的位置 读取总共长度 异常错误
func readAtLeastDoubleBreakLine(c net.Conn, buff []byte) (int, int, error) {
    var maxsize int = cap(buff)
    var rn int
    var lp int // 位置

    for {
        n, err := c.Read(buff[rn:])
        if n > 0 {
            rn += n
        }

        if err != nil {
            return -1, rn, err
        }

        if n <= 0 {
            return -1, rn, io.ErrUnexpectedEOF
        }

        // logger.Printf("read from %s buff lp: %d len: %d rn: %d %s", c.RemoteAddr(), lp, rn-lp, rn, string(buff[rn-n:rn]))
        if idx := bytes.Index(buff[lp:rn], doubleBreakLine); idx >= 0 {
            return lp + idx + 4, rn, nil
        }

        if rn >= maxsize {
            return -1, rn, io.ErrShortBuffer
        }

        lp = rn - 4
        if lp < 0 {
            lp = 0
        }
    }

    panic("[BUG]")
}

// u: url
func getRemoteAddress(u string) (string, error) {
    hostPortURL, err := url.Parse(u)
    if err != nil {
        return "", err
    }
    var addr string
    if hostPortURL.Opaque == "443" {
        if strings.Index(u, ":") == -1 {
            addr = u + ":443"
        } else {
            addr = u
        }
    } else {
        if strings.Index(u, ":") == -1 {
            addr = u + ":80"
        } else {
            addr = u
        }
    }
    return addr, nil
}
