// The "main" package/entry point for authboss-worked.
package main

/* "scooter me fecit"

Copyright 2022 B. Scott Michel

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program. If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"gitlab.com/scooter-phd/authboss-worked/abossworked"
)

func main() {
	var (
		exitStatus   = 0
		err          error
		workedConfig *abossworked.ConfigData
	)

	mainLog := log.New(os.Stdout, "main ", log.LstdFlags)
	workedConfig, err = abossworked.GetWorkedConfig()
	if err == nil {
		var authStorer *abossworked.AuthStorer

		authStorer, err = abossworked.OpenUserDB(workedConfig.WorkedRoot)
		if err == nil {
			defer authStorer.Close()

			var templates *abossworked.Templates

			templates, err = abossworked.TemplateLoader("content", "content/fragments", "master_layout.gohtml", nil,
				workedConfig)
			if err == nil {
				var router *gin.Engine

				router, err = abossworked.GinRouter(workedConfig, authStorer, templates)
				if err == nil {
					go gracefulShutdown(mainLog, authStorer)
					router.Run(workedConfig.HostPortString())
				}
			}
		}
	}

	if err != nil {
		mainLog.Fatal(fmt.Errorf("%w", err))
		// NOTREACHED
	}

	os.Exit(exitStatus)
}

func gracefulShutdown(mainLog *log.Logger, authStorer *abossworked.AuthStorer) {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	mainLog.Println("Received", sig, "shutting down and exiting.")

	// Cleanups and shutdowns.
	authStorer.Close()
	os.Exit(0)
}
