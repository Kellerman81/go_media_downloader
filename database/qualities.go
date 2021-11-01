package database

import (
	"time"
)

//type 1 reso 2 qual 3 codec 4 audio
type Qualities struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Type      int
	Name      string
	Regex     string
	Strings   string
	Priority  int
}

var ListResolutions = []Qualities{
	{Type: 1, Name: "360p", Priority: 10000, Regex: "(\\b|_)360p(\\b|_)", Strings: "360p,360i"},
	{Type: 1, Name: "368p", Priority: 20000, Regex: "(\\b|_)368p(\\b|_)", Strings: "368p,368i"},
	{Type: 1, Name: "480p", Priority: 30000, Regex: "(\\b|_)480p(\\b|_)", Strings: "480p,480i"},
	{Type: 1, Name: "576p", Priority: 40000, Regex: "(\\b|_)576p(\\b|_)", Strings: "576p,576i"},
	{Type: 1, Name: "720p", Priority: 50000, Regex: "(\\b|_)(1280x)?720(i|p)(\\b|_)", Strings: "720p,720i"},
	{Type: 1, Name: "1080p", Priority: 60000, Regex: "(\\b|_)(1920x)?1080(i|p)(\\b|_)", Strings: "1080p,1080i"},
	{Type: 1, Name: "2160p", Priority: 70000, Regex: "(\\b|_)((3840x)?2160p|4k)(\\b|_)", Strings: "2160p,2160i"}}

var ListQualities = []Qualities{
	{Type: 2, Name: "workprint", Priority: 1000, Regex: "(\\b|_)workprint(\\b|_)", Strings: "workprint"},
	{Type: 2, Name: "cam", Priority: 1300, Regex: "(\\b|_)(?:web)?cam(\\b|_)", Strings: "webcam,cam"},
	{Type: 2, Name: "ts", Priority: 2000, Regex: "(\\b|_)(?:hd)?ts|telesync(\\b|_)", Strings: "hdts,ts,telesync"},
	{Type: 2, Name: "tc", Priority: 2300, Regex: "(\\b|_)(tc|telecine)(\\b|_)", Strings: "tc,telecine"},
	{Type: 2, Name: "r5", Priority: 3000, Regex: "(\\b|_)r[2-8c](\\b|_)", Strings: "r5,r6"},
	{Type: 2, Name: "hdrip", Priority: 3300, Regex: "(\\b|_)hd[^a-zA-Z0-9]?rip(\\b|_)", Strings: "hdrip,hd.rip,hd rip,hd-rip,hd_rip"},
	{Type: 2, Name: "ppvrip", Priority: 4000, Regex: "(\\b|_)ppv[^a-zA-Z0-9]?rip(\\b|_)", Strings: "ppvrip,ppv.rip,ppv rip,ppv-rip,ppv_rip"},
	{Type: 2, Name: "preair", Priority: 4300, Regex: "(\\b|_)preair(\\b|_)", Strings: "preair"},
	{Type: 2, Name: "tvrip", Priority: 5000, Regex: "(\\b|_)tv[^a-zA-Z0-9]?rip(\\b|_)", Strings: "tvrip,tv.rip,tv rip,tv-rip,tv_rip"},
	{Type: 2, Name: "dsr", Priority: 5300, Regex: "(\\b|_)(dsr|ds)[^a-zA-Z0-9]?rip(\\b|_)", Strings: "dsrip,ds.rip,ds rip,ds-rip,ds_rip,dsrrip,dsr.rip,dsr rip,dsr-rip,dsr_rip"},
	{Type: 2, Name: "sdtv", Priority: 6000, Regex: "(\\b|_)(?:[sp]dtv|dvb)(?:[^a-zA-Z0-9]?rip)?(\\b|_)", Strings: "sdtv,pdtv,dvb,sdtvrip,sdtv.rip,sdtv rip,sdtv-rip,sdtv_rip,pdtvrip,pdtv.rip,pdtv rip,pdtv-rip,pdtv_rip,dvbrip,dvb.rip,dvb rip,dvb-rip,dvb_rip"},
	{Type: 2, Name: "dvdscr", Priority: 6300, Regex: "(\\b|_)(?:(?:dvd|web)[^a-zA-Z0-9]?)?scr(?:eener)?(\\b|_)", Strings: "webscr,webscreener,web.scr,web.screener,web-scr,web-screener,web scr,web screener,web_scr,web_screener,dvdscr,dvdscreener,dvd.scr,dvd.screener,dvd-scr,dvd-screener,dvd scr,dvd screener,dvd_scr,dvd_screener"},
	{Type: 2, Name: "bdscr", Priority: 7000, Regex: "(\\b|_)bdscr(?:eener)?(\\b|_)", Strings: "bdscr,bdscreener"},
	{Type: 2, Name: "webrip", Priority: 7300, Regex: "(\\b|_)web[^a-zA-Z0-9]?rip(\\b|_)", Strings: "webrip,web.rip,web rip,web-rip,web_rip"},
	{Type: 2, Name: "hdtv", Priority: 8000, Regex: "(\\b|_)a?hdtv(?:[^a-zA-Z0-9]?rip)?(\\b|_)", Strings: "hdtv,hdtvrip,hdtv.rip,hdtv rip,hdtv-rip,hdtv_rip"},
	{Type: 2, Name: "webdl", Priority: 8300, Regex: "(\\b|_)web(?:[^a-zA-Z0-9]?(dl|hd))?(\\b|_)", Strings: "webdl,web dl,web.dl,web-dl,web_dl,webhd,web.hd,web hd,web-hd,web_hd"},
	{Type: 2, Name: "dvdrip", Priority: 9000, Regex: "(\\b|_)(dvd[^a-zA-Z0-9]?rip|hddvd)(\\b|_)", Strings: "dvdrip,dvd.rip,dvd rip,dvd-rip,dvd_rip,hddvd,dvd"},
	{Type: 2, Name: "remux", Priority: 9100, Regex: "(\\b|_)remux(\\b|_)", Strings: "remux"},
	{Type: 2, Name: "bluray", Priority: 9300, Regex: "(\\b|_)(?:b[dr][^a-zA-Z0-9]?rip|blu[^a-zA-Z0-9]?ray(?:[^a-zA-Z0-9]?rip)?)(\\b|_)", Strings: "bdrip,bd.rip,bd rip,bd-rip,bd_rip,brrip,br.rip,br rip,br-rip,br_rip,bluray,blu ray,blu.ray,blu-ray,blu_ray"}}
var ListCodecs = []Qualities{
	{Type: 3, Name: "divx", Priority: 100, Regex: "(\\b|_)divx(\\b|_)", Strings: "divx"},
	{Type: 3, Name: "xvid", Priority: 200, Regex: "(\\b|_)xvid(\\b|_)", Strings: "xvid"},
	{Type: 3, Name: "h264", Priority: 300, Regex: "(\\b|_)(h|x)264(\\b|_)", Strings: "h264,x264"},
	{Type: 3, Name: "vp9", Priority: 400, Regex: "(\\b|_)vp9(\\b|_)", Strings: "vp9"},
	{Type: 3, Name: "h265", Priority: 500, Regex: "(\\b|_)((h|x)265|hevc)(\\b|_)", Strings: "h265,x265,hevc"},
	{Type: 3, Name: "10bit", Priority: 600, Regex: "(\\b|_)(10bit|hi10p)(\\b|_)", Strings: "10bit,hi10p"}}
var ListAudio = []Qualities{
	{Type: 4, Name: "mp3", Priority: 10, Regex: "(\\b|_)mp3(\\b|_)", Strings: "mp3"},
	{Type: 4, Name: "aac", Priority: 20, Regex: "(\\b|_)aac(s)?(\\b|_)", Strings: "aac,aacs"},
	{Type: 4, Name: "dd5.1", Priority: 30, Regex: "(\\b|_)dd[0-9\\.]+(\\b|_)"},
	{Type: 4, Name: "ac3", Priority: 40, Regex: "(\\b|_)ac3(s)?(\\b|_)", Strings: "ac3,ac3s"},
	{Type: 4, Name: "dd+5.1", Priority: 50, Regex: "(\\b|_)dd[p+][0-9\\.]+(\\b|_)"},
	{Type: 4, Name: "flac", Priority: 60, Regex: "(\\b|_)flac(s)?(\\b|_)", Strings: "flac,flacs"},
	{Type: 4, Name: "dtshd", Priority: 70, Regex: "(\\b|_)dts[^a-zA-Z0-9]?hd(?:[^a-zA-Z0-9]?ma)?(s)?(\\b|_)"},
	{Type: 4, Name: "dts", Priority: 80, Regex: "(\\b|_)dts(s)?(\\b|_)", Strings: "dts,dtss"},
	{Type: 4, Name: "truehd", Priority: 90, Regex: "(\\b|_)truehd(s)?(\\b|_)", Strings: "truehd"}}
