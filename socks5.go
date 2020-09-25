package woow

import (
    "encoding/binary"
    "github.com/pkg/errors"
    "io"
    "net"
    "strconv"
    "time"

    "github.com/woodada/ioutil/smartnet"
)

const (
    // client to server
    SOCKS5_IPV4             = 0x01
    SOCKS5_DOMAIN_NAME      = 0x03
    SOCKS5_IPV6             = 0x04
    SOCKS5_CONNECT_METHOD   = 0x01
    SOCKS5_BIND_METHOD      = 0x02
    SOCKS5_ASSOCIATE_METHOD = 0x03
    // The maximum packet size of any udp Associate packet, based on ethernet's max size,
    // minus the IP and UDP headers. IPv4 has a 20 byte header, UDP adds an
    // additional 4 bytes.  This is a total overhead of 24 bytes.  Ethernet's
    // max packet size is 1500 bytes,  1500 - 24 = 1476.
    SOCKS5_MAX_UDP_PACKET_SIZE = 1476
    // server to client
    SOCKS5_NO_NEED_AUTH      = 0x00
    SOCKS5_USER_PASS_AUTH    = 0x02
    SOCKS5_USER_AUTH_VERSION = 0x01
    SOCKS5_AUTH_SUCCESS      = 0x00
    SOCKS5_AUTH_FAILURE      = 0x01

    SOCKS5_VERSION                    = 0x05
    SOCKS5_CONNECT_SUCC               = 0x00
    SOCKS5_NETWORK_UNREACHABLE        = 0x03
    SOCKS5_ADDRESS_TYPE_NOT_SUPPORTED = 0x08 // Address type not supported

)

func (this *Server) proxySOCKS5(conn net.Conn, callback smartnet.StopCallBackFunc) {
    if err := this.doSocks5HandShake(conn); err != nil {
        conn.Close()
        callback(err, nil)
        return
    }

    cmd, err := this.doSocks5Header(conn)
    if err != nil {
        conn.Close()
        callback(err, nil)
        return
    }

    var remote net.Conn = nil
    switch cmd {
    case SOCKS5_CONNECT_METHOD:
        remote, err = this.doSocks5Connect(conn)
    case SOCKS5_ASSOCIATE_METHOD:
        err = this.doSocks5Associate(conn)
    }

    if err != nil {
        if remote != nil {
            remote.Close()
        }
        conn.Close()
        callback(err, nil)
        return
    }

    cfg := smartnet.DefaultTunnelConfig()
    cfg.StopCallback = callback
    cfg.B2AConfig.WriteTimeout = 10 * time.Second
    cfg.B2AConfig.ReadTimeout = this.config.TunnelTimeout
    cfg.A2BConfig.ReadTimeout = this.config.TunnelTimeout
    cfg.A2BConfig.WriteTimeout = 10 * time.Second
    smartnet.NewRateTunnel(conn, remote, cfg)
}

// 握手包括鉴权
func (this *Server) doSocks5HandShake(conn net.Conn) error {
    /*
       VERSION	METHODS_COUNT	METHODS…
       1字节	1字节	1到255字节，长度由METHODS_COUNT值决定
       0x05	0x03	0x00 0x01 0x02
    */
    var buf [150]byte
    conn.SetReadDeadline(time.Now().Add(1 * time.Second))
    n, err := io.ReadFull(conn, buf[0:2])
    if err != nil {
        return errors.WithMessage(err, "read header")
    }
    if n != 2 {
        return errors.Errorf("%d expect 2,%s", n, "read header")
    }
    if buf[0] != 5 {
        return errors.Errorf("only support socks5")
    }

    methodNums := int(buf[1])
    if methodNums > len(buf)-2 {
        return errors.Errorf("method nums: %d too large", methodNums)
    }

    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    n, err = io.ReadFull(conn, buf[2:2+methodNums])
    if err != nil {
        return errors.WithMessage(err, "read method")
    } else if n != methodNums {
        return errors.Errorf("%d expect %d,%s", n, methodNums, "read method")
    }

    if this.accounts.Size() > 0 {
        buf[1] = SOCKS5_USER_PASS_AUTH // UserPassAuth
        conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
        if _, err = conn.Write(buf[0:2]); err != nil {
            return errors.WithMessage(err, "send auth request")
        }

        if err = this.doSocks5Auth(conn); err != nil {
            conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
            conn.Write(_socks5AuthFail)
            return errors.WithMessage(err, "auth fail")
        }
        conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
        _, err = conn.Write(_socks5AuthSucc)
        if err != nil {
            return errors.WithMessage(err, "send auth resp")
        }
    } else {
        buf[1] = SOCKS5_NO_NEED_AUTH
        conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
        if _, err = conn.Write(buf[0:2]); err != nil {
            return errors.WithMessage(err, "send auth request")
        }
    }
    return nil
}

var _socks5AuthSucc = []byte{SOCKS5_USER_AUTH_VERSION, SOCKS5_AUTH_SUCCESS}
var _socks5AuthFail = []byte{SOCKS5_USER_AUTH_VERSION, SOCKS5_AUTH_FAILURE}

func (this *Server) doSocks5Auth(conn net.Conn) error {
    /*
       账号密码认证
       VERSION	METHOD
       1字节	1字节
       0x05	0x02
       客户端发送账号密码
       服务端返回的认证方法为0x02(账号密码认证)时，客户端会发送账号密码数据给代理服务器
       VERSION	USERNAME_LENGTH	USERNAME	PASSWORD_LENGTH	PASSWORD
       1字节	1字节	1-255字节	1字节	1-255字节
       0x01	0x01	0x0a	0x01	0x0a
    */
    var header [257]byte
    conn.SetReadDeadline(time.Now().Add(3 * time.Second))
    if _, err := io.ReadFull(conn, header[0:2]); err != nil {
        return errors.WithMessage(err, "read header")
    }

    if header[0] != SOCKS5_USER_AUTH_VERSION {
        return errors.Errorf("not supported version: %d", header[0])
    }

    userLen := int(header[1])
    conn.SetReadDeadline(time.Now().Add(1 * time.Second))
    // 多读一个字节 把密码的长度读取出来
    if _, err := io.ReadFull(conn, header[0:userLen+1]); err != nil {
        return errors.WithMessage(err, "read username")
    }

    username := string(header[0:userLen]) //string copy一份
    passLen := int(header[userLen])
    conn.SetReadDeadline(time.Now().Add(1 * time.Second))
    if _, err := io.ReadFull(conn, header[0:passLen]); err != nil {
        return errors.WithMessage(err, "read password")
    }
    passwd := string(header[0:passLen]) //string copy一份

    if code := this.accounts.Compare(username, passwd); code != 0 {
        if code == 1 {
            return errors.Errorf("not found username: %s passwd: %s", username, passwd)
        } else {
            return errors.Errorf("invalid username: %s passwd: %s", username, passwd)
        }
    }
    return nil
}

//  正常情况返回命令字 有任何异常返回错误
func (this *Server) doSocks5Header(conn net.Conn) (byte, error) {
    /*
    	The SOCKS request is formed as follows:
    	+----+-----+-------+
    	|VER | CMD |  RSV  |
    	+----+-----+-------+
    	| 1  |  1  | X'00' |
    	+----+-----+-------+
    */
    var header [3]byte
    conn.SetReadDeadline(time.Now().Add(1 * time.Second))
    _, err := io.ReadFull(conn, header[:])
    if err != nil {
        return 0, errors.WithMessage(err, "read header")
    }

    if header[0] != SOCKS5_VERSION {
        return 0, errors.Errorf("not supported version: %d", header[0])
    }
    cmd := header[1]
    if cmd == SOCKS5_CONNECT_METHOD ||
        cmd == SOCKS5_ASSOCIATE_METHOD {
        // 这些都是支持的协议
        return cmd, nil
    } else {
        // not support SOCKS5_BIND_METHOD
        return 0, errors.Errorf("not supported method: %d", cmd)
    }
}

// 正常返回远程连接 异常会返回错误 异常情况下本接口不关闭远程连接
func (this *Server) doSocks5Connect(conn net.Conn) (net.Conn, error) {
    host, port, err := getSocks5Addr(conn)
    if err != nil {
        return nil, errors.WithMessage(err, "getSocks5Addr")
    }

    remoteAddr := host + ":" + strconv.Itoa(port)
    remote, err := net.Dial("tcp", remoteAddr)
    conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
    if err != nil {
        conn.Write(_socks5NetwordUnreachable)
        return remote, errors.WithMessagef(err, "dial remote addr %s", remoteAddr)
    }

    _, err = conn.Write(_socks5ConnectSucc)
    if err != nil {
        return remote, errors.WithMessage(err, "write connect succ resp")
    }

    return remote, nil
}

// connect resp
var _socks5NotSupportedAddr = []byte{SOCKS5_VERSION, SOCKS5_ADDRESS_TYPE_NOT_SUPPORTED, 0, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
var _socks5ConnectSucc = []byte{SOCKS5_VERSION, SOCKS5_CONNECT_SUCC, 0, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
var _socks5NetwordUnreachable = []byte{SOCKS5_VERSION, SOCKS5_NETWORK_UNREACHABLE, 0, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}

// Network unreachable

// 获取被代理的地址
// 正常情况返回 host port nil  异常情况返回 空串 错误信息
func getSocks5Addr(conn net.Conn) (string, int, error) {
    var buf [257]byte
    conn.SetReadDeadline(time.Now().Add(3 * time.Second))
    _, err := conn.Read(buf[0:1])
    if err != nil {
        return "", -1, errors.WithMessage(err, "read addr type")
    }

    // 地址类型
    addrType := buf[0]

    var host string
    // 地址
    switch addrType {
    case SOCKS5_IPV4:
        _, err = io.ReadFull(conn, buf[0:net.IPv4len])
        if err != nil {
            return "", -1, errors.WithMessage(err, "read ipv4")
        }
        host = net.IP(buf[0:net.IPv4len]).String()
    case SOCKS5_IPV6:
        _, err = io.ReadFull(conn, buf[0:net.IPv6len])
        if err != nil {
            return "", -1, errors.WithMessage(err, "read ipv6")
        }
        host = net.IP(buf[0:net.IPv6len]).String()
    case SOCKS5_DOMAIN_NAME:
        var domainLen int
        _, err = conn.Read(buf[0:1])
        if err != nil {
            return "", -1, errors.WithMessage(err, "read domain length")
        }
        domainLen = int(buf[0])
        _, err = io.ReadFull(conn, buf[0:domainLen])
        if err != nil {
            return "", -1, errors.WithMessage(err, "read domain")
        }
        host = string(buf[0:domainLen])
    default:
        conn.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
        conn.Write(_socks5NotSupportedAddr)
        return "", -1, errors.Errorf("not support addr type: %d", addrType)
    }

    // 读取端口
    _, err = io.ReadFull(conn, buf[0:2])
    if err != nil {
        return "", -1, errors.WithMessage(err, "read port")
    }

    port := int(binary.BigEndian.Uint16(buf[0:2]))
    return host, port, nil
}

func (this *Server) doSocks5Associate(conn net.Conn) error {
    // TODO
    return nil
}
