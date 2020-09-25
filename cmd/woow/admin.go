package main

import (
    "fmt"
    "net/http"
    "net/http/pprof"
    "os"

    "github.com/woodada/ioutil/ini"
    "github.com/woodada/woow"
)

func Admin(svr *woow.Server, dict ini.Dict) {
    accounts := dict.GetKeys("accounts")
    mux := http.NewServeMux()

    mux.HandleFunc("/debug/pprof/", AuthFilter(pprof.Index))
    mux.HandleFunc("/debug/pprof/cmdline", AuthFilter(pprof.Cmdline))
    mux.HandleFunc("/debug/pprof/profile", AuthFilter(pprof.Profile))
    mux.HandleFunc("/debug/pprof/symbol", AuthFilter(pprof.Symbol))
    mux.HandleFunc("/debug/pprof/trace", AuthFilter(pprof.Trace))
    mux.HandleFunc("/status", AuthFilter(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(fmt.Sprint(
            "Version: ", version,
            "\n", "Runables: ", svr.Runables(),
            "\n", "Connections: ", svr.Connections(),
            "\n", "Accounts: ", accounts,
            "\n",
        )))
    }))

    addr, ok := dict.GetString("admin", "listen_addr")
    if !ok {
        addr = "127.0.0.1:54321"
    }
    fmt.Println("admin listen at", addr)
    err := http.ListenAndServe(addr, mux)
    if err != nil {
        fmt.Fprintln(os.Stderr, "webui serv fail! err:", err)
        os.Exit(1)
    }
}

func AuthFilter(handlerFunc http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // TODO 鉴权
        handlerFunc(w, r)
    }
}
