package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"

    "github.com/woodada/ioutil/ini"
    "github.com/woodada/ioutil/sslcert"
    "github.com/woodada/woow"

    "gopkg.in/natefinch/lumberjack.v2"
)

const (
    version = "0.0.3.200925" // 版本号
)

func isInString(a string, ss ...string) bool {
    for _, s := range ss {
        if a == s {
            return true
        }
    }
    return false
}

func main() {
    preCheckArgs()
    dict, err := ini.Load(os.Args[1])
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }

    cfg := loadConfig(dict)

    logpath, _ := dict.GetString("server", "log_path")
    if len(logpath) > 0 {
        output := &lumberjack.Logger{
            Filename:   logpath, //filePath
            MaxSize:    500,     // megabytes
            MaxBackups: 10000,
            MaxAge:     1,     //days
            Compress:   false, // disabled by default
        }
        defer output.Close()
        cfg.Logger = log.New(output, "", log.LstdFlags|log.LUTC|log.Lshortfile)
    }

    s := woow.NewServer(cfg)

    err = s.Run()
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }

    fmt.Println("server listen at", cfg.ListenAddress)

    go Admin(s, dict)

    waitExit()
}

func waitExit() {
    ch := make(chan os.Signal)
    signal.Notify(ch, os.Interrupt, os.Kill)
    fmt.Fprintln(os.Stderr, <-ch)
}

func preCheckArgs() {
    for i, a := range os.Args {
        if i == 0 {
            continue
        }
        if isInString(a, "--version", "-version", "--v", "-v") {
            fmt.Println(version)
            os.Exit(1)
        }

        if isInString(a, "--config", "-config", "--c", "-c") {
            fmt.Println(defaultConfigData)
            os.Exit(1)
        }

        if isInString(a, "--gencert", "-gencert") {
            k, c, err := sslcert.GenerateCert()
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            err = ioutil.WriteFile("./gencert.key", k, os.FileMode(0644))
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            err = ioutil.WriteFile("./gencert.cert", c, os.FileMode(0644))
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            os.Exit(0)
        }

        if isInString(a, "--help", "-help", "--h", "-h") {
            fmt.Println(usage)
            os.Exit(1)
        }
    }

    if len(os.Args) != 2 {
        fmt.Println(usage)
        os.Exit(1)
    }
}

const usage = `使用说明
1. 查看版本号
proxy -version
proxy --version
proxy --v
proxy -v

2. 生成ssl证书
proxy --gencert
proxy -gencert

3. 查看配置
proxy --config
proxy -config
proxy --c
proxy -c

导出默认配置
proxy --config > xxx.ini

4. 运行
proxy xxx.ini
`

const defaultConfigData = `# woow config file

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
listen_addr=:54321

[tunnel]
# 空闲时间 单位秒 
timeout= 600

[accounts]
woo=123456
lucy=123456
`