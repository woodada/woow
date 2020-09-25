# 说明

woow是用go开发的高并发的代理服务，目前支持http https socks5代理。
并且只需要监听一个端口，可以自动识别并正确代理。
取名woow因为看起来像一头猪。

## 使用

编译
```
go get gopkg.in/natefinch/lumberjack.v2
go get github.com/woodada/ioutil
go get github.com/woodada/woow
cd github.com/woodada/woow/cmd/woow
go build -ldflags "-w -s" -o woow
```


# 使用说明
1.查看版本号
```
proxy -version
proxy --version
proxy --v
proxy -v
```

2.生成ssl证书
```
proxy --gencert
proxy -gencert
```

3.查看配置
```
proxy --config
proxy -config
proxy --c
proxy -c

导出默认配置
proxy --config > xxx.ini
```

默认配置如下
```cassandraql
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

# 鉴权配置 无任何配置代理将不需要鉴权
[accounts]
woo=123456
lucy=123456
```

4.运行
```
proxy xxx.ini
```

5.测试

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

## 其它
已知在Mac下配置的代理auth信息将不生效，也就是说系统即使配置了代理的账号密码，但是实际浏览器请求不带鉴权。
将[accounts]下配置为空，即可关闭鉴权。

