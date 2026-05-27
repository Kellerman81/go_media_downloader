package apiexternal

import "errors"

var (
	errServerURLEmpty        = errors.New("server URL empty")
	errMessageEmpty          = errors.New("message empty")
	errNotificationURLsEmpty = errors.New("notification URLs empty")
	errClientEmpty           = errors.New("client empty")
	errTokenEmpty            = errors.New("token empty")
	errMessageTooLong        = errors.New("message too long")
	errTitleTooLong          = errors.New("title too long")
	errAPIKeyEmpty           = errors.New("apikey empty")
	errNoClient              = errors.New("no client")
	errNoClientReturned      = errors.New("no Client returned")
	errTraktNotInit          = errors.New("trakt provider not initialized")
)
