package sabnzbd

import "net/http"

// Source: https://github.com/mrobinsn/go-sabnzbd - fixed:add category.
type Option func(*Sabnzbd) error

func UseHTTP() Option {
	return func(s *Sabnzbd) error {
		s.useHTTP()
		return nil
	}
}

func UseHTTPS() Option {
	return func(s *Sabnzbd) error {
		s.useHTTPS()
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
		s.useInsecureHTTP()
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
