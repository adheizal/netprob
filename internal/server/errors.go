package server

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

func writeInternalError(w http.ResponseWriter, err error, message string) {
	log.Error().Err(err).Msg(message)
	http.Error(w, message, http.StatusInternalServerError)
}
