package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gitlab.com/scooter-phd/explan/explan"
	"gitlab.com/scooter-phd/explan/userdb"
)

func main() {
	var exitStatus = 0

	if explanConfig, err := explan.GetExplanConfig(); err == nil {
		if authStorer, err := userdb.OpenUserDB(explanConfig.SQLSchemaSig); err == nil {
			defer authStorer.Close()
			if router, err := setupRouter(explanConfig); err == nil {
				router.Run(explanConfig.ListenAddr)
			}
		}
	} else {
		exitStatus = 1
	}

	os.Exit(exitStatus)
}

func setupRouter(cfg *explan.ConfigData) (*gin.Engine, error) {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.StaticFS("/", http.Dir("content"))

	return router, nil
}
