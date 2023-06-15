package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

type MediaInfoJson struct {
	Media struct {
		Track []Track `json:"track"`
	} `json:"media"`
}

type Track struct {
	Type                  string `json:"@type"`
	ID                    string `json:"ID"`
	VideoCount            string `json:"VideoCount,omitempty"`
	AudioCount            string `json:"AudioCount,omitempty"`
	FileExtension         string `json:"FileExtension,omitempty"`
	Format                string `json:"Format"`
	FileSize              string `json:"FileSize,omitempty"`
	Duration              string `json:"Duration"`
	OverallBitRateMode    string `json:"OverallBitRate_Mode,omitempty"`
	OverallBitRate        string `json:"OverallBitRate,omitempty"`
	FrameRate             string `json:"FrameRate"`
	FrameCount            string `json:"FrameCount,omitempty"`
	FileModifiedDate      string `json:"File_Modified_Date,omitempty"`
	FileModifiedDateLocal string `json:"File_Modified_Date_Local,omitempty"`
	Extra                 struct {
		OverallBitRatePrecisionMin string `json:"OverallBitRate_Precision_Min"`
		OverallBitRatePrecisionMax string `json:"OverallBitRate_Precision_Max"`
	} `json:"extra,omitempty"`
	StreamOrder                    string `json:"StreamOrder,omitempty"`
	MenuID                         string `json:"MenuID,omitempty"`
	FormatProfile                  string `json:"Format_Profile,omitempty"`
	FormatLevel                    string `json:"Format_Level,omitempty"`
	FormatSettingsCABAC            string `json:"Format_Settings_CABAC,omitempty"`
	FormatSettingsRefFrames        string `json:"Format_Settings_RefFrames,omitempty"`
	CodecID                        string `json:"CodecID,omitempty"`
	BitRateMode                    string `json:"BitRate_Mode,omitempty"`
	BitRateNominal                 string `json:"BitRate_Nominal,omitempty"`
	BitRateMaximum                 string `json:"BitRate_Maximum,omitempty"`
	Width                          string `json:"Width,omitempty"`
	Height                         string `json:"Height,omitempty"`
	StoredHeight                   string `json:"Stored_Height,omitempty"`
	SampledWidth                   string `json:"Sampled_Width,omitempty"`
	SampledHeight                  string `json:"Sampled_Height,omitempty"`
	PixelAspectRatio               string `json:"PixelAspectRatio,omitempty"`
	DisplayAspectRatio             string `json:"DisplayAspectRatio,omitempty"`
	ColorSpace                     string `json:"ColorSpace,omitempty"`
	ChromaSubsampling              string `json:"ChromaSubsampling,omitempty"`
	BitDepth                       string `json:"BitDepth,omitempty"`
	ScanType                       string `json:"ScanType,omitempty"`
	Delay                          string `json:"Delay,omitempty"`
	EncodedLibrary                 string `json:"Encoded_Library,omitempty"`
	EncodedLibraryName             string `json:"Encoded_Library_Name,omitempty"`
	EncodedLibraryVersion          string `json:"Encoded_Library_Version,omitempty"`
	EncodedLibrarySettings         string `json:"Encoded_Library_Settings,omitempty"`
	BufferSize                     string `json:"BufferSize,omitempty"`
	ColourDescriptionPresent       string `json:"colour_description_present,omitempty"`
	ColourDescriptionPresentSource string `json:"colour_description_present_Source,omitempty"`
	ColourRange                    string `json:"colour_range,omitempty"`
	ColourRangeSource              string `json:"colour_range_Source,omitempty"`
	ColourPrimaries                string `json:"colour_primaries,omitempty"`
	ColourPrimariesSource          string `json:"colour_primaries_Source,omitempty"`
	TransferCharacteristics        string `json:"transfer_characteristics,omitempty"`
	TransferCharacteristicsSource  string `json:"transfer_characteristics_Source,omitempty"`
	MatrixCoefficients             string `json:"matrix_coefficients,omitempty"`
	MatrixCoefficientsSource       string `json:"matrix_coefficients_Source,omitempty"`
	FormatVersion                  string `json:"Format_Version,omitempty"`
	FormatAdditionalFeatures       string `json:"Format_AdditionalFeatures,omitempty"`
	MuxingMode                     string `json:"MuxingMode,omitempty"`
	Channels                       string `json:"Channels,omitempty"`
	ChannelPositions               string `json:"ChannelPositions,omitempty"`
	ChannelLayout                  string `json:"ChannelLayout,omitempty"`
	SamplesPerFrame                string `json:"SamplesPerFrame,omitempty"`
	SamplingRate                   string `json:"SamplingRate,omitempty"`
	SamplingCount                  string `json:"SamplingCount,omitempty"`
	CompressionMode                string `json:"Compression_Mode,omitempty"`
	DelaySource                    string `json:"Delay_Source,omitempty"`
	Language                       string `json:"Language,omitempty"`
}

var mediainfopath string

func getmediainfoFilename() string {
	if mediainfopath != "" {
		return mediainfopath
	}

	if runtime.GOOS == "windows" {
		mediainfopath = logger.PathJoin(config.SettingsGeneral.MediainfoPath, "mediainfo.exe")
	} else {
		mediainfopath = logger.PathJoin(config.SettingsGeneral.MediainfoPath, "mediainfo")
	}
	return mediainfopath
}
func parsemediainfo(m *apiexternal.ParseInfo, file *string, qualityTemplate string) error {
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	//cmd := exec.CommandContext(ctx, getmediainfoFilename(), "--Output=JSON", *file)
	var outputBuf, stdErr bytes.Buffer
	err := ExecCmd(getmediainfoFilename(), file, "mediainfo", &outputBuf, &stdErr)
	defer outputBuf.Reset()
	defer stdErr.Reset()
	//cmd := exec.Command(getmediainfoFilename(), "--Output=JSON", *file)
	//var outputBuf, stdErr bytes.Buffer

	//cmd.Stdout = &outputBuf
	//cmd.Stderr = &stdErr
	//err := cmd.Run()
	if err != nil {
		return errors.New("error running mediainfo [" + stdErr.String() + "] " + err.Error() + " [" + outputBuf.String() + "]")
	}
	//cancel()
	//logger.ClearVar(cmd)

	if stdErr.Len() > 0 {
		return errors.New("mediainfo error: " + stdErr.String())
	}
	var info MediaInfoJson
	err = json.NewDecoder(&outputBuf).Decode(&info)
	if err != nil {
		return err
	}

	if len(info.Media.Track) == 0 {
		return errors.New("no tracks")
	}
	defer logger.ClearVar(&info)
	var redetermineprio bool
	var getreso string
	for idxstream := range info.Media.Track {
		if info.Media.Track[idxstream].Type == "Audio" {
			if info.Media.Track[idxstream].Language != "" {
				m.Languages = append(m.Languages, info.Media.Track[idxstream].Language)
			}
			if m.Audio == "" || (!strings.EqualFold(info.Media.Track[idxstream].CodecID, m.Audio) && info.Media.Track[idxstream].Format != "") {
				m.Audio = info.Media.Track[idxstream].Format
				m.AudioID = gettypeids(logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if info.Media.Track[idxstream].Type != "video" {
			continue
		}

		if strings.EqualFold(info.Media.Track[idxstream].Format, "mpeg4") && strings.EqualFold(info.Media.Track[idxstream].CodecID, "xvid") {
			info.Media.Track[idxstream].Format = info.Media.Track[idxstream].CodecID
		}
		if m.Codec == "" || (!strings.EqualFold(info.Media.Track[idxstream].Format, m.Codec) && info.Media.Track[idxstream].Format != "") {
			m.Codec = info.Media.Track[idxstream].Format
			m.CodecID = gettypeids(logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		m.Height = logger.StringToInt(info.Media.Track[idxstream].Height)
		m.Width = logger.StringToInt(info.Media.Track[idxstream].Width)
		m.Runtime = logger.StringToInt(logger.SplitByRet(info.Media.Track[idxstream].Duration, '.'))

		if m.Height > m.Width {
			m.Height, m.Width = m.Width, m.Height
		}
		getreso = parseresolution(m.Height, m.Width)

		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}

	if redetermineprio {
		//allQualityPrioritiesMu.Lock()
		cfgname := config.SettingsQuality["quality_"+qualityTemplate].Name
		intid := -1
		for idxi := range allQualityPrioritiesWantedT {
			if strings.EqualFold(allQualityPrioritiesWantedT[idxi].QualityGroup, cfgname) && allQualityPrioritiesWantedT[idxi].ResolutionID == m.ResolutionID && allQualityPrioritiesWantedT[idxi].QualityID == m.QualityID && allQualityPrioritiesWantedT[idxi].CodecID == m.CodecID && allQualityPrioritiesWantedT[idxi].AudioID == m.AudioID {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&allQualityPrioritiesWantedT, func(e Prioarr) bool {
		//	return strings.EqualFold(e.QualityGroup, cfgname) && e.ResolutionID == m.ResolutionID && e.QualityID == m.QualityID && e.CodecID == m.CodecID && e.AudioID == m.AudioID
		//})
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}
