package parser

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

type MediaInfoJson struct {
	Media MediaInfoJsonMedia `json:"media"`
}
type MediaInfoJsonMedia struct {
	Track []Track `json:"track"`
}

// track represents a media track from the JSON response
type Track struct {
	// Type is the JSON key "@type"
	Type string `json:"@type"`

	// Format is the JSON key "Format"
	Format string `json:"Format"`

	// Duration is the JSON key "Duration"
	Duration string `json:"Duration"`

	// CodecID is the JSON key "CodecID"
	CodecID string `json:"CodecID,omitempty"`

	// Width is the JSON key "Width"
	Width string `json:"Width,omitempty"`

	// Height is the JSON key "Height"
	Height string `json:"Height,omitempty"`

	// Language is the JSON key "Language"
	Language string `json:"Language,omitempty"`
}

// type track struct {
// 	Type string `json:"@type"`
// 	//ID         string `json:"ID"`
// 	//VideoCount string `json:"VideoCount,omitempty"`
// 	//AudioCount string `json:"AudioCount,omitempty"`
// 	//FileExtension                  string     `json:"FileExtension,omitempty"`
// 	Format string `json:"Format"`
// 	//FileSize                       string     `json:"FileSize,omitempty"`
// 	Duration string `json:"Duration"`
// 	//OverallBitRateMode             string     `json:"OverallBitRate_Mode,omitempty"`
// 	//OverallBitRate                 string     `json:"OverallBitRate,omitempty"`
// 	//FrameRate                      string     `json:"FrameRate"`
// 	//FrameCount                     string     `json:"FrameCount,omitempty"`
// 	//FileModifiedDate               string     `json:"File_Modified_Date,omitempty"`
// 	//FileModifiedDateLocal          string     `json:"File_Modified_Date_Local,omitempty"`
// 	//Extra                          TrackExtra `json:"extra,omitempty"`
// 	//StreamOrder                    string     `json:"StreamOrder,omitempty"`
// 	//MenuID                         string     `json:"MenuID,omitempty"`
// 	//FormatProfile                  string     `json:"Format_Profile,omitempty"`
// 	//FormatLevel                    string     `json:"Format_Level,omitempty"`
// 	//FormatSettingsCABAC            string     `json:"Format_Settings_CABAC,omitempty"`
// 	//FormatSettingsRefFrames        string     `json:"Format_Settings_RefFrames,omitempty"`
// 	CodecID string `json:"CodecID,omitempty"`
// 	//BitRateMode                    string     `json:"BitRate_Mode,omitempty"`
// 	//BitRateNominal                 string     `json:"BitRate_Nominal,omitempty"`
// 	//BitRateMaximum                 string     `json:"BitRate_Maximum,omitempty"`
// 	Width  string `json:"Width,omitempty"`
// 	Height string `json:"Height,omitempty"`
// 	//StoredHeight                   string     `json:"Stored_Height,omitempty"`
// 	//SampledWidth                   string     `json:"Sampled_Width,omitempty"`
// 	//SampledHeight                  string     `json:"Sampled_Height,omitempty"`
// 	//PixelAspectRatio               string     `json:"PixelAspectRatio,omitempty"`
// 	//DisplayAspectRatio             string     `json:"DisplayAspectRatio,omitempty"`
// 	//ColorSpace                     string     `json:"ColorSpace,omitempty"`
// 	//ChromaSubsampling              string     `json:"ChromaSubsampling,omitempty"`
// 	//BitDepth                       string     `json:"BitDepth,omitempty"`
// 	//ScanType                       string     `json:"ScanType,omitempty"`
// 	//Delay                          string     `json:"Delay,omitempty"`
// 	//EncodedLibrary                 string     `json:"Encoded_Library,omitempty"`
// 	//EncodedLibraryName             string     `json:"Encoded_Library_Name,omitempty"`
// 	//EncodedLibraryVersion          string     `json:"Encoded_Library_Version,omitempty"`
// 	//EncodedLibrarySettings         string     `json:"Encoded_Library_Settings,omitempty"`
// 	//BufferSize                     string     `json:"BufferSize,omitempty"`
// 	//ColourDescriptionPresent       string     `json:"colour_description_present,omitempty"`
// 	//ColourDescriptionPresentSource string     `json:"colour_description_present_Source,omitempty"`
// 	//ColourRange                    string     `json:"colour_range,omitempty"`
// 	//ColourRangeSource              string     `json:"colour_range_Source,omitempty"`
// 	//ColourPrimaries                string     `json:"colour_primaries,omitempty"`
// 	//ColourPrimariesSource          string     `json:"colour_primaries_Source,omitempty"`
// 	//TransferCharacteristics        string     `json:"transfer_characteristics,omitempty"`
// 	//TransferCharacteristicsSource  string     `json:"transfer_characteristics_Source,omitempty"`
// 	//MatrixCoefficients             string     `json:"matrix_coefficients,omitempty"`
// 	//MatrixCoefficientsSource       string     `json:"matrix_coefficients_Source,omitempty"`
// 	//FormatVersion                  string     `json:"Format_Version,omitempty"`
// 	//FormatAdditionalFeatures       string     `json:"Format_AdditionalFeatures,omitempty"`
// 	//MuxingMode                     string     `json:"MuxingMode,omitempty"`
// 	//Channels                       string     `json:"Channels,omitempty"`
// 	//ChannelPositions               string     `json:"ChannelPositions,omitempty"`
// 	//ChannelLayout                  string     `json:"ChannelLayout,omitempty"`
// 	//SamplesPerFrame                string     `json:"SamplesPerFrame,omitempty"`
// 	//SamplingRate                   string     `json:"SamplingRate,omitempty"`
// 	//SamplingCount                  string     `json:"SamplingCount,omitempty"`
// 	//CompressionMode                string     `json:"Compression_Mode,omitempty"`
// 	//DelaySource                    string     `json:"Delay_Source,omitempty"`
// 	Language string `json:"Language,omitempty"`
// }

func (t *MediaInfoJson) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	clear(t.Media.Track)
	t.Media.Track = nil
	*t = MediaInfoJson{}
}

// getmediainfoFilename returns the path to the mediainfo executable.
// It first checks if a custom path has been set in mediainfopath.
// If not, it constructs the default path based on the OS and the
// config.SettingsGeneral.MediainfoPath setting.
func getmediainfoFilename() string {
	if mediainfopath != "" {
		return mediainfopath
	}

	if runtime.GOOS == "windows" {
		mediainfopath = filepath.Join(config.SettingsGeneral.MediainfoPath, "mediainfo.exe")
	} else {
		mediainfopath = filepath.Join(config.SettingsGeneral.MediainfoPath, "mediainfo")
	}
	// if !filepath.IsAbs(mediainfopath) {
	// 	mediainfopath, _ = filepath.Abs(mediainfopath)
	// }
	return mediainfopath
}

// parsemediainfo parses the output of mediainfo for the given file.
// It extracts information about the media tracks, resolution, languages,
// codecs etc. and populates the FileParser struct with the extracted data.
// It handles calling mediainfo and parsing the JSON output.
func parsemediainfo(m *database.ParseInfo, file string, qualcfg *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}
	data, errstr, err := ExecCmd(getmediainfoFilename(), file, "mediainfo")
	if err != nil {
		return errors.New("error running mediainfo [" + errstr + "] " + err.Error() + " [" + logger.BytesToString2(data) + "]")
	}

	if errstr != "" {
		return errors.New("mediainfo error: " + errstr)
	}
	var info MediaInfoJson
	err = json.Unmarshal(data, &info)
	if err != nil {
		return err
	}
	//clear(data)
	defer info.Close()

	if len(info.Media.Track) == 0 {
		return logger.ErrTracksEmpty
	}
	var redetermineprio bool
	var n int
	for idx := range info.Media.Track {
		if info.Media.Track[idx].Type == "Audio" && info.Media.Track[idx].Language != "" {
			n++
		}
	}
	if n > 0 {
		m.Languages = make([]string, 0, n)
	}
	for idx := range info.Media.Track {
		if info.Media.Track[idx].Type == "Audio" {
			if info.Media.Track[idx].Language != "" {
				m.Languages = append(m.Languages, info.Media.Track[idx].Language)
			}
			if m.Audio == "" || (info.Media.Track[idx].Format != "" && !strings.EqualFold(info.Media.Track[idx].CodecID, m.Audio)) {
				m.Audio = info.Media.Track[idx].Format
				m.AudioID = database.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if info.Media.Track[idx].Type != "video" {
			continue
		}

		if (info.Media.Track[idx].Format == "mpeg4" || strings.EqualFold(info.Media.Track[idx].Format, "mpeg4")) && (info.Media.Track[idx].CodecID == "xvid" || strings.EqualFold(info.Media.Track[idx].CodecID, "xvid")) {
			info.Media.Track[idx].Format = info.Media.Track[idx].CodecID
		}
		if m.Codec == "" || (info.Media.Track[idx].Format != "" && !strings.EqualFold(info.Media.Track[idx].Format, m.Codec)) {
			m.Codec = info.Media.Track[idx].Format
			m.CodecID = database.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		m.Height = logger.StringToInt(info.Media.Track[idx].Height)
		m.Width = logger.StringToInt(info.Media.Track[idx].Width)
		info.Media.Track[idx].Duration = splitByFullP(info.Media.Track[idx].Duration, '.')
		m.Runtime = logger.StringToInt(info.Media.Track[idx].Duration)

		if m.Height > m.Width {
			m.Height, m.Width = m.Width, m.Height
		}
		getreso := m.Parseresolution()

		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = database.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}
	if redetermineprio {
		intid := Findpriorityidxwanted(m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, qualcfg)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}
