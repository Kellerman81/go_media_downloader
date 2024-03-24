package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/pool"
	"github.com/goccy/go-json"
)

type mediaInfoJson struct {
	Media mediaInfoJsonMedia `json:"media"`
}
type mediaInfoJsonMedia struct {
	Track []track `json:"track"`
}

// track represents a media track from the JSON response
type track struct {
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

var plmediainfo = pool.NewPool(100, 0, func(b *mediaInfoJson) {}, func(b *mediaInfoJson) { *b = mediaInfoJson{} })

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
	return mediainfopath
}

// parsemediainfo parses the output of mediainfo for the given file.
// It extracts information about the media tracks, resolution, languages,
// codecs etc. and populates the FileParser struct with the extracted data.
// It handles calling mediainfo and parsing the JSON output.
func parsemediainfo(m *apiexternal.FileParser, file string, qualcfg *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}
	out := ExecCmd(getmediainfoFilename(), file, "mediainfo")
	defer out.Close()
	if out.Err != nil {
		return fmt.Errorf("error running mediainfo [%s] %s [%s]", out.Outerror, out.Err.Error(), out.Out)
	}

	if out.Outerror != "" {
		return errors.New("mediainfo error: " + out.Outerror)
	}
	info := plmediainfo.Get()
	defer plmediainfo.Put(info)
	err := json.Unmarshal(out.Out, info)
	if err != nil {
		return err
	}

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
	m.M.Languages = make([]string, 0, n)
	for _, track := range info.Media.Track {
		if track.Type == "Audio" {
			if track.Language != "" {
				m.M.Languages = append(m.M.Languages, track.Language)
			}
			if m.M.Audio == "" || (track.Format != "" && !strings.EqualFold(track.CodecID, m.M.Audio)) {
				m.M.Audio = track.Format
				m.M.AudioID = gettypeids(m.M.Audio, database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if track.Type != "video" {
			continue
		}

		if strings.EqualFold(track.Format, "mpeg4") && strings.EqualFold(track.CodecID, "xvid") {
			track.Format = track.CodecID
		}
		if m.M.Codec == "" || (track.Format != "" && !strings.EqualFold(track.Format, m.M.Codec)) {
			m.M.Codec = track.Format
			m.M.CodecID = gettypeids(m.M.Codec, database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		m.M.Height = logger.StringToInt(track.Height)
		m.M.Width = logger.StringToInt(track.Width)
		track.Duration = logger.SplitByFullP(track.Duration, '.')
		m.M.Runtime = logger.StringToInt(track.Duration)

		if m.M.Height > m.M.Width {
			m.M.Height, m.M.Width = m.M.Width, m.M.Height
		}
		getreso := parseresolution(&m.M)

		if getreso != "" && (m.M.Resolution == "" || !strings.EqualFold(getreso, m.M.Resolution)) {
			m.M.Resolution = getreso
			m.M.ResolutionID = gettypeids(m.M.Resolution, database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}

	if redetermineprio {
		intid := findpriorityidxwanted(m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, qualcfg)
		if intid != -1 {
			m.M.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}
