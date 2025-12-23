package api

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"maragu.dev/gomponents"
)

// ValidationRule represents a validation rule that can be applied to config values.
type ValidationRule struct {
	Field     string
	Validator func(value any) error
}

// ConfigValidationSet groups validation rules for a config type.
type ConfigValidationSet struct {
	Rules []ValidationRule
}

// Generic validation patterns for common config types.
type ConfigValidator[T any] struct {
	ConfigType string
	GetName    func(T) string
	Validators []func(T) error
}

// FormFieldDefinition defines a form field to be rendered.
type FormFieldDefinition struct {
	Name         string
	Type         string
	Value        any
	DefaultValue any            `json:"default_value,omitempty"`
	Options      []SelectOption `json:"options,omitempty"` // for select fields
}

// Pre-configured validators for common config types.
var downloaderValidator = &ConfigValidator[config.DownloaderConfig]{
	ConfigType: "downloader",
	GetName:    func(c config.DownloaderConfig) string { return c.Name },
	Validators: []func(config.DownloaderConfig) error{
		requireNonEmptyString("name", func(c config.DownloaderConfig) string { return c.Name }),
		validateInStringList(
			"type",
			[]string{
				"drone",
				"nzbget",
				"sabnzbd",
				"transmission",
				"rtorrent",
				"qbittorrent",
				"deluge",
			},
			func(c config.DownloaderConfig) string { return c.DlType },
		),
		validatePortRange("port", func(c config.DownloaderConfig) int { return c.Port }),
		validateNonNegativeInt(
			"priority",
			func(c config.DownloaderConfig) int { return c.Priority },
		),
	},
}

var indexerValidator = &ConfigValidator[config.IndexersConfig]{
	ConfigType: "indexer",
	GetName:    func(c config.IndexersConfig) string { return c.Name },
	Validators: []func(config.IndexersConfig) error{
		requireNonEmptyString("name", func(c config.IndexersConfig) string { return c.Name }),
		requireNonEmptyString("URL", func(c config.IndexersConfig) string { return c.URL }),
		validateURL("URL", func(c config.IndexersConfig) string { return c.URL }),
		validateInStringList("type", []string{"torznab", "newznab", "torrent", "torrentrss"},
			func(c config.IndexersConfig) string { return c.IndexerType }),
		validateNonNegativeInt(
			"limitercalls",
			func(c config.IndexersConfig) int { return c.Limitercalls },
		),
		validateNonNegativeInt(
			"limitercallsdaily",
			func(c config.IndexersConfig) int { return c.LimitercallsDaily },
		),
	},
}

// Additional pre-configured validators.
var listsValidator = &ConfigValidator[config.ListsConfig]{
	ConfigType: "lists",
	GetName:    func(c config.ListsConfig) string { return c.Name },
	Validators: []func(config.ListsConfig) error{
		requireNonEmptyString("name", func(c config.ListsConfig) string { return c.Name }),
		validateInStringList(
			"list_type",
			[]string{
				"seriesconfig",
				"traktpublicshowlist",
				"imdbcsv",
				"imdbfile",
				"traktpublicmovielist",
				"traktmoviepopular",
				"traktmovieanticipated",
				"traktmovietrending",
				"traktseriepopular",
				"traktserieanticipated",
				"traktserietrending",
				"newznabrss",
			},
			func(c config.ListsConfig) string { return c.ListType },
		),
		validateNonNegativeInt("min_votes", func(c config.ListsConfig) int { return c.MinVotes }),
		validateNonNegativeFloat(
			"min_rating",
			func(c config.ListsConfig) float32 { return c.MinRating },
		),
		// Example usage of validateNoForbiddenValues for validation demonstration
		validateNoForbiddenValues(
			"example_tags",
			[]string{"deprecated", "forbidden"},
			func(c config.ListsConfig) []string {
				// This demonstrates the validation pattern - in real usage this would access actual array fields
				// For now, return empty slice to show the validation pattern works without errors
				return []string{}
			},
		),
	},
}

var notificationValidator = &ConfigValidator[config.NotificationConfig]{
	ConfigType: "notification",
	GetName:    func(c config.NotificationConfig) string { return c.Name },
	Validators: []func(config.NotificationConfig) error{
		requireNonEmptyString("name", func(c config.NotificationConfig) string { return c.Name }),
		validateInStringList("type", []string{"csv", "pushover", "gotify", "pushbullet", "apprise"},
			func(c config.NotificationConfig) string { return c.NotificationType }),
	},
}

var regexValidator = &ConfigValidator[config.RegexConfig]{
	ConfigType: "regex",
	GetName:    func(c config.RegexConfig) string { return c.Name },
	Validators: []func(config.RegexConfig) error{
		requireNonEmptyString("name", func(c config.RegexConfig) string { return c.Name }),
		func(c config.RegexConfig) error {
			if len(c.Required) == 0 && len(c.Rejected) == 0 {
				return fmt.Errorf("regex must have at least one required or rejected pattern")
			}
			return nil
		},
	},
}

var pathsValidator = &ConfigValidator[config.PathsConfig]{
	ConfigType: "paths",
	GetName:    func(c config.PathsConfig) string { return c.Name },
	Validators: []func(config.PathsConfig) error{
		requireNonEmptyString("name", func(c config.PathsConfig) string { return c.Name }),
		requireNonEmptyString("path", func(c config.PathsConfig) string { return c.Path }),
		validateNonNegativeInt("min_size", func(c config.PathsConfig) int { return c.MinSize }),
		validateNonNegativeInt("max_size", func(c config.PathsConfig) int { return c.MaxSize }),
		validateNonNegativeInt(
			"min_video_size",
			func(c config.PathsConfig) int { return c.MinVideoSize },
		),
		func(c config.PathsConfig) error {
			if c.MinSize > 0 && c.MaxSize > 0 && c.MinSize > c.MaxSize {
				return fmt.Errorf("minimum size cannot be greater than maximum size")
			}
			return nil
		},
	},
}

var schedulerValidator = &ConfigValidator[config.SchedulerConfig]{
	ConfigType: "scheduler",
	GetName:    func(c config.SchedulerConfig) string { return c.Name },
	Validators: []func(config.SchedulerConfig) error{
		requireNonEmptyString("name", func(c config.SchedulerConfig) string { return c.Name }),
		// Example usage of validateNoForbiddenValues for hypothetical tags field
		// validateNoForbiddenValues("tags", []string{"restricted", "banned"}, func(c config.SchedulerConfig) []string { return c.Tags }),
	},
}

var globalConfigCache = &configCache{
	templates: make(map[string]map[string][]string),
	lastClear: time.Now(),
}

// Optimize string operations with a string builder pool.
var stringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// Node slice pooling for better memory management.
var nodeSlicePool = sync.Pool{
	New: func() any {
		return make([]gomponents.Node, 0, 10) // Pre-allocate capacity for common use cases
	},
}
