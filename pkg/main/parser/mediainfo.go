package parser

import (
	"path/filepath"
	"runtime"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

type mediaInfoJSON struct {
	Media struct {
		Track []struct {
			Type     string `json:"@type"`
			Format   string `json:"Format"`
			Duration string `json:"Duration"`
			CodecID  string `json:"CodecID,omitempty"`
			Width    string `json:"Width,omitempty"`
			Height   string `json:"Height,omitempty"`
			Language string `json:"Language,omitempty"`
		} `json:"track"`
	} `json:"media"`
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

// getmediainfoFilename returns the path to the mediainfo executable.
// It first checks if a custom path has been set in mediainfopath.
// If not, it constructs the default path based on the OS and the
// config.GetSettingsGeneral().MediainfoPath setting.
func getmediainfoFilename() string {
	if mediainfopath != "" {
		return mediainfopath
	}

	if runtime.GOOS == "windows" {
		mediainfopath = filepath.Join(config.GetSettingsGeneral().MediainfoPath, "mediainfo.exe")
	} else {
		mediainfopath = filepath.Join(config.GetSettingsGeneral().MediainfoPath, "mediainfo")
	}

	return mediainfopath
}
