package logging

import (
	"os"

	"github.com/rs/zerolog"
)

type ZerologAdapter struct {
	logger zerolog.Logger
}

func NewZerologAdapter() *ZerologAdapter {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	return &ZerologAdapter{
		logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

func (z *ZerologAdapter) Trace(msg string) {
	z.logger.Trace().Msg(msg)
}

func (z *ZerologAdapter) Debug(msg string) {
	z.logger.Debug().Msg(msg)
}

func (z *ZerologAdapter) Info(msg string) {
	z.logger.Info().Msg(msg)
}

func (z *ZerologAdapter) Error(msg string) {
	z.logger.Error().Msg(msg)
}

func (z *ZerologAdapter) Fatal(msg string) {
	z.logger.Fatal().Msg(msg)
}
