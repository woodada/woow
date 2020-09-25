package woow

import (
    "crypto/tls"
    "net"
    "sync/atomic"
    "time"

    "github.com/woodada/ioutil/smartnet"
)

type Config struct {
    Logger         smartnet.Logger
    Accounts       *AccountStore
    SSLEnabled     bool
    SSLKey         string
    SSLCert        string
    PreReadTimeout time.Duration
    TunnelTimeout  time.Duration
    KeepAlive      time.Duration
    ListenAddress  string
}

type Server struct {
    l        *smartnet.SmartListener
    logger   smartnet.Logger
    accounts *AccountStore
    config   Config
    connNums int64
}

func NewServer(c Config) *Server {
    if c.Logger == nil {
        c.Logger = smartnet.NewEmptyLogger()
    }

    if c.PreReadTimeout <= 0 {
        c.PreReadTimeout = 3 * time.Second
    }

    if c.TunnelTimeout <= 0 {
        c.TunnelTimeout = 300 * time.Second
    }

    if c.KeepAlive <= 0 {
        c.KeepAlive = 290 * time.Second
    }

    if c.Accounts == nil {
        c.Accounts = NewAccountStore(nil)
    }

    s := &Server{
        config:   c,
        logger:   c.Logger,
        accounts: c.Accounts,
    }

    return s
}

func (this *Server) Run() error {
    c := smartnet.Config{PreReadTimeout: this.config.PreReadTimeout,
        ListenConfig: &net.ListenConfig{KeepAlive: this.config.KeepAlive}}

    l, err := smartnet.NewSmartListener("tcp", this.config.ListenAddress, c)
    if err != nil {
        return err
    }

    this.l = l
    if this.config.SSLEnabled {
        cert, err := tls.LoadX509KeyPair(
            this.config.SSLCert,
            this.config.SSLKey,
        )
        if err != nil {
            return err
        }

        sc := &tls.Config{
            MinVersion:   tls.VersionTLS12,
            Certificates: []tls.Certificate{cert},
        }
        sl := tls.NewListener(l.ExtListener(), sc)
        go this.handleListen("https", sl, this.proxyHTTP)
    }
    go this.handleListen("http", l.HttpListener(), this.proxyHTTP)
    go this.handleListen("socks5", l.Socks5Listener(), this.proxySOCKS5)
    this.logger.Printf("listen http: %v socks5: %v ext: %v", l.HttpEnabled(), l.Socks5Enabled(), l.ExtEnabled())

    return nil
}

func (this *Server) Runables() int64 {
    return this.l.ConnRunables()
}

func (this *Server) Connections() int64 {
    return atomic.LoadInt64(&this.connNums)
}

func (this *Server) handleListen(name string, l net.Listener, proxyFunc ProxyFunc) {
    this.logger.Printf("%s %s listening", name, l.Addr())
    defer this.logger.Printf("%s %s stop listen", name, l.Addr())

    for {
        c, err := l.Accept()
        if err != nil {
            this.logger.Printf("%s %s accept err: %s", name, l.Addr(), err)
            return
        }
        go this.proxyWarp(name, c, proxyFunc)
    }
}

// callback写法有点丑 但能有效减少协程数 FIXME 用hub+channel方式 避免跨线程调用风险
type ProxyFunc func(conn net.Conn, callback smartnet.StopCallBackFunc)

func (this *Server) proxyWarp(name string, c net.Conn, f ProxyFunc) {
    atomic.AddInt64(&this.connNums, 1)
    this.logger.Printf("%s %s -> %s come in", name, c.RemoteAddr(), c.LocalAddr())
    f(c, func(e1, e2 error) {
        atomic.AddInt64(&this.connNums, -1)
        this.logger.Printf("%s %s -> %s closed cause %v", name, c.RemoteAddr(), c.LocalAddr(), e1)
    })
}
