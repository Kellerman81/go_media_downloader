package sabnzbd

import "net/http"

type Option func(*Sabnzbd) error

func UseHttp() Option {
	return func(s *Sabnzbd) error {
		s.useHttp()
		return nil
	}
}

func UseHttps() Option {
	return func(s *Sabnzbd) error {
		s.useHttps()
		return nil
	}
}

func UseHTTPAuth(user, pass string) Option {
	return func(s *Sabnzbd) error {
		s.httpUser = user
		s.httpPass = pass
		return nil
	}
}

func UseInsecureHTTP() Option {
	return func(s *Sabnzbd) error {
		s.useInsecureHttp()
		return nil
	}
}

func Addr(addr string) Option {
	return func(s *Sabnzbd) error {
		return s.setAddr(addr)
	}
}

func Path(path string) Option {
	return func(s *Sabnzbd) error {
		return s.setPath(path)
	}
}

func LoginAuth(username, password string) Option {
	return func(s *Sabnzbd) error {
		return s.setAuth(loginAuth{username, password})
	}
}

func ApikeyAuth(apikey string) Option {
	return func(s *Sabnzbd) error {
		return s.setAuth(apikeyAuth{apikey})
	}
}

func NoneAuth() Option {
	return func(s *Sabnzbd) error {
		return s.setAuth(noneAuth{})
	}
}

func UseRoundTripper(rt http.RoundTripper) Option {
	return func(s *Sabnzbd) error {
		return s.setRoundTripper(rt)
	}
}
