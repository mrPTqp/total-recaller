package main

import (
	"github.com/mrPTqp/total-recaller/pkg/logger"
)

func main() {
	log := logger.NewLogger()
	defer func() {
		_ = log.Sync()
	}()
}
