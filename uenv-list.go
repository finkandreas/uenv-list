package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/mux"

    "cscs.ch/uenv-list/handler"
    "cscs.ch/uenv-list/util"
)

func main() {
    var configpath string
    flag.StringVar(&configpath, "config", "config.yaml", "Path to config YAML file")
    flag.Parse()
    config := util.ReadConfig(configpath)

    reqHandler := mux.NewRouter()
    reqHandler.HandleFunc("/list", handler.GetListHandler(config))
    reqHandler.PathPrefix("/").Handler(handler.CatchAllHandler{})

    listenAddress := fmt.Sprintf("%v:%v", config.Server.Address, config.Server.Port)
    server := &http.Server{
        Addr:              listenAddress,
        ReadHeaderTimeout: 1 * time.Second,
        Handler: reqHandler,
    }
    log.Printf("Starting server on %v", listenAddress)
    log.Fatalf("Server stopped. err=%v", server.ListenAndServe())
}
