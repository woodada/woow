高并发的代理服务，采用go语言开发，同时支持http/https/socks5。且只需要监听一个端口，可以自动识别并正确代理。
取名woow因为看起来像一头猪。

1.编译
```
go get gopkg.in/natefinch/lumberjack.v2
go get github.com/woodada/ioutil
go get github.com/woodada/woow
cd github.com/woodada/woow/cmd/woow
go build -ldflags "-w -s" -o woow
```

2.查看版本号
```
proxy -version or proxy --version or proxy --v or proxy -v
```

3.生成ssl证书自签发
```
proxy --gencert or proxy -gencert
```

4.查看配置
```
proxy --config or proxy -config or proxy --c or proxy -c

导出默认配置
proxy --config > xxx.ini
```

默认配置如下
```
# woow config file

[server]
# 代理服务监听地址
listen_addr=:12345

# 日志路径 路径为空默认不打印日志
log_path=/tmp/woow.log

# 连接建立后第一批读超时时间 单位秒
pre_read_timeout = 3

# 是否启用https_proxy 可以改为真实有效的签发证书
ssl_enabled=true
ssl_key="./gencert.key"
ssl_cert="./gencert.cert"

[admin]
# 内置管理页面监听地址
listen_addr=127.0.0.1:54321

[tunnel]
# 空闲时间 单位秒 
timeout= 600

# 鉴权配置 无任何账号配置即可免Auth代理
# 已知在Mac下即使配置了代理的账号密码，但是实际浏览器请求也是不带鉴权的。将[accounts]下配置为空，即可关闭代理服务的鉴权。
[accounts]
woo=123456
lucy=123456
```

5.运行
```
proxy xxx.ini
```

6.测试

```
curl -k -v -x 'http://woo:123456@127.0.0.1:12345' https://www.baidu.com
curl -k -v -x 'socks5://woo:123456@127.0.0.1:12345' https://www.baidu.com
curl -k  --proxy-insecure -v -x 'https://woo:123456@127.0.0.1:12345' https://www.baidu.com
```
只需要一个端口，同时支持http socks5 https 三种代理；
使用https代理时加上--proxy-insecure选项，这是因为这是自己签发的证书。

## 管理页面

```
http://127.0.0.1:54321//debug/pprof/
http://127.0.0.1:54321/status
```

## 开发

采用模块开发,在代码中Run一个Server即可，Server间独立，可以启动多个。

```
type Config struct {
    Logger         smartnet.Logger // 日志对象 默认不打印日志
    Accounts       *AccountStore   // 账号存储 线程安全 可运行用增删改账号信息
    SSLEnabled     bool            // 启用https代理
    SSLKey         string          // key 
    SSLCert        string          // cert
    PreReadTimeout time.Duration   // 第一批字节读取超时时间
    TunnelTimeout  time.Duration   // 隧道空闲断开时间
    KeepAlive      time.Duration   // TCP keep alive
    ListenAddress  string          // 服务监听地址
}

s := woow.NewServer(cfg)
s.Run()
```


