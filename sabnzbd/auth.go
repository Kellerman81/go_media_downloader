package sabnzbd

import (
	"fmt"
	"mime/multipart"
	"net/url"
)

// authenticator adds authentication parameters to a url
type authenticator interface {
	Authenticate(url.Values)
	AuthenticateMultipart(*multipart.Writer) error
	fmt.Stringer
}

type noneAuth struct{}

func (a noneAuth) Authenticate(v url.Values)                       {}
func (a noneAuth) AuthenticateMultipart(m *multipart.Writer) error { return nil }
func (a noneAuth) String() string                                  { return "None" }

type apikeyAuth struct {
	Apikey string
}

func (a apikeyAuth) Authenticate(v url.Values) {
	v.Set("apikey", a.Apikey)
}

func (a apikeyAuth) AuthenticateMultipart(m *multipart.Writer) error {
	return m.WriteField("apikey", a.Apikey)
}

func (a apikeyAuth) String() string { return "apikey" }

type loginAuth struct {
	Username string
	Password string
}

func (a loginAuth) Authenticate(v url.Values) {
	v.Set("ma_username", a.Username)
	v.Set("ma_password", a.Password)
}

func (a loginAuth) AuthenticateMultipart(m *multipart.Writer) error {
	if err := m.WriteField("ma_username", a.Username); err != nil {
		return err
	}
	return m.WriteField("ma_password", a.Password)
}

func (a loginAuth) String() string { return "login" }
