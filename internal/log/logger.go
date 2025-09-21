// placeholder logger.go
package log

import "go.uber.org/zap"

func New(env string) *zap.SugaredLogger {
	var lg *zap.Logger
	var err error
	if env == "dev" {
		lg, err = zap.NewDevelopment()
	} else {
		lg, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	return lg.Sugar()
}
