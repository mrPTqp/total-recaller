package app

import (
	"net/http"
	"sync"
)

type App struct {
	c        *AppComponents
	server   *http.Server
	shutdown sync.Once
}

func NewApp(components *AppComponents) *App {

}
