package main

import (
    "github.com/woodada/ioutil/ini"
    "github.com/woodada/woow"
    "time"
)

func loadConfig(dict ini.Dict) woow.Config {
    cfg := woow.Config{}

    // 加载账号密码配置
    {
        usernames := dict.GetKeys("accounts")
        if len(usernames) > 0 {
            m := make(map[string]string)
            for _, name := range usernames {
                m[name], _ = dict.GetString("accounts", name)
            }
            cfg.Accounts = woow.NewAccountStore(m)
        }
    }

    {
        cfg.ListenAddress, _ = dict.GetString("server", "listen_addr")
        v, ok := dict.GetInt("server", "pre_read_timeout")
        if ok {
            cfg.PreReadTimeout = time.Duration(v) * time.Second
        } else {
            cfg.PreReadTimeout = 3 * time.Second
        }

        v, ok = dict.GetInt("server", "keep_alive")
        if ok {
            cfg.KeepAlive = time.Duration(v) * time.Second
        } else {
            cfg.KeepAlive = 300 * time.Second
        }

        cfg.SSLEnabled, _ = dict.GetBool("server", "ssl_enabled")
        cfg.SSLKey, _ = dict.GetString("server", "ssl_key")
        cfg.SSLCert, _ = dict.GetString("server", "ssl_cert")
    }

    // tunnel
    {
        v, ok := dict.GetInt("tunnel", "timeout")
        if ok {
            cfg.TunnelTimeout = time.Duration(v) * time.Second
        } else {
            cfg.TunnelTimeout = 600 * time.Second
        }
    }

    return cfg
}
