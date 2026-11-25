package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

// @Summary      Get the whole configuration
// @Description  Returns every configuration section, keyed by name (general, imdb, quality_SD, downloader_sab, ...). Passwords and API keys are redacted.
// @Tags         config
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=Jsonconfigall}  "All configuration entries (keys shown are examples)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/all [get].
func apiConfigAll(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// configRedacted replaces secret config values in API responses.
const configRedacted = "***REDACTED***"

// isSecretConfigField reports whether a config struct field name (as
// encoded to JSON, e.g. "WebAPIKey", "Password", "PlexToken") holds a
// credential.
func isSecretConfigField(name string) bool {
	n := strings.ToLower(name)

	return strings.Contains(n, "password") || strings.Contains(n, "apikey") ||
		strings.Contains(n, "api_key") || strings.Contains(n, "token") ||
		strings.Contains(n, "secret") || strings.HasSuffix(n, "key")
}

// redactConfigSecrets returns a JSON-shaped copy of v with every non-empty
// secret field replaced, so config read endpoints don't hand out the web
// API key, download-client passwords or indexer keys.
func redactConfigSecrets(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}

	redactSecretsWalk(out)

	return out
}

func redactSecretsWalk(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if s != "" && isSecretConfigField(k) {
					t[k] = configRedacted
				}

				continue
			}

			redactSecretsWalk(val)
		}

	case []any:
		for _, val := range t {
			redactSecretsWalk(val)
		}
	}
}

// restoreRedactedSecrets puts the stored value back into every field of dst
// that still holds configRedacted, so a client that read a config through the
// redacting endpoints and posts it back does not overwrite real secrets.
func restoreRedactedSecrets(dst any, name string) {
	stored, ok := config.GetCfgAll()[name]
	if !ok {
		return
	}

	var cur, old any

	b, err := json.Marshal(dst)
	if err != nil || json.Unmarshal(b, &cur) != nil {
		return
	}

	b, err = json.Marshal(stored)
	if err != nil || json.Unmarshal(b, &old) != nil {
		return
	}

	if !mergeRedacted(cur, old) {
		return
	}

	if b, err = json.Marshal(cur); err == nil {
		_ = json.Unmarshal(b, dst)
	}
}

// mergeRedacted replaces configRedacted values in cur with the value at the
// same position in old and reports whether anything was replaced.
func mergeRedacted(cur, old any) bool {
	changed := false

	switch c := cur.(type) {
	case map[string]any:
		o, _ := old.(map[string]any)
		for k, v := range c {
			if s, ok := v.(string); ok && s == configRedacted {
				if ov, ok := o[k]; ok {
					c[k] = ov
				} else {
					c[k] = ""
				}

				changed = true

				continue
			}

			if mergeRedacted(v, o[k]) {
				changed = true
			}
		}

	case []any:
		o, _ := old.([]any)
		for i, v := range c {
			var ov any
			if i < len(o) {
				ov = o[i]
			}

			if mergeRedacted(v, ov) {
				changed = true
			}
		}
	}

	return changed
}

// @Summary      Reset the configuration
// @Description  Replaces the whole configuration with example entries and writes config.toml. Everything you configured is lost - make a copy of config.toml first. Returns the new configuration.
// @Tags         config
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=Jsonconfigall}  "The remaining configuration"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/clear [delete].
func apiConfigClear(ctx *gin.Context) {
	config.ClearCfg()
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      Get one configuration section
// @Description  Returns one configuration section by its name, e.g. quality_SD or downloader_sab. Passwords and API keys are redacted.
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=config.MediaTypeConfig}  "The entry; its type depends on the section (media group shown)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/get/{name} [get].
func apiConfigGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()[name]))
}

// @Summary      Delete one configuration section
// @Description  Removes one configuration section, reloads the affected parts and writes config.toml (comments in the file are lost). Returns the remaining configuration.
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=Jsonconfigall}  "The remaining configuration"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/delete/{name} [delete].
func apiConfigDelete(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	if err := config.DeleteCfgEntry(name); err == nil {
		reloadRuntimeFor(name)
	}

	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      Reload config.toml
// @Description  Reloads config.toml from disk after manual edits: quality priorities, schedules, download clients, indexers and notifications are rebuilt.
// @Tags         config
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonconfigreloaded  "Configuration reloaded from the file"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/refresh [get].
func apiConfigRefreshFile(ctx *gin.Context) {
	config.LoadCfgDB(true)
	parser.GenerateAllQualityPriorities()
	parser.GenerateCutoffPriorities()
	scheduler.Reload()
	providers.ReloadClientProviders()
	providers.RegisterNotificationProviders()
	sendSuccess(ctx, "config reloaded")
}

// reloadRuntimeFor re-applies a saved or deleted config entry to the parts of
// the running app that only read it at registration time: the download-client,
// indexer and notification providers, and the scheduler. Without the config file watcher
// nothing else would pick the change up before a restart.
func reloadRuntimeFor(name string) {
	if strings.HasPrefix(name, "downloader_") || strings.HasPrefix(name, "indexer_") {
		providers.ReloadClientProviders()
	}

	// Notification providers carry the general user agent.
	if name == "general" || strings.HasPrefix(name, "notification_") {
		providers.RegisterNotificationProviders()
	}

	if affectsScheduler(name) {
		scheduler.Reload()
	}
}

// affectsScheduler reports whether a config entry feeds the schedules: the
// scheduler templates, the media configs that reference them, and general
// (UseCronInsteadOfInterval).
func affectsScheduler(name string) bool {
	if name == "general" {
		return true
	}

	for _, p := range []string{"scheduler_", "serie_", "movie_", "book_", "audiobook_", "music_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}

	return false
}

// @Summary      Add or replace one configuration section
// @Description  Saves the JSON body as the named configuration section (general, imdb, media, downloader, indexer, list, path, notification, regex, quality, scheduler), reloads the affected parts and writes config.toml (comments in the file are lost). Redacted secrets keep their stored value.
// @Tags         config
// @Param        config  body      any  true  "The new configuration entry as JSON; its shape depends on the section in name (see /api/config/get/{name})"
// @Param        name    path      string       true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=Jsonconfigall}  "The whole configuration after the change"
// @Failure      400  {object}  Jsonbadrequest  "Unknown section or invalid entry"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/update/{name} [post].
func apiConfigUpdate(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}

	var err error

	switch left {
	case "general":
		var getcfg config.GeneralConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "downloader":
		var getcfg config.DownloaderConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case logger.StrImdb:
		var getcfg config.ImdbConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "indexer":
		var getcfg config.IndexersConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "list":
		var getcfg config.ListsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "serie", "movie":
		var getcfg config.MediaTypeConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "notification":
		var getcfg config.NotificationConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "path":
		var getcfg config.PathsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "quality":
		var getcfg config.QualityConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "regex":
		var getcfg config.RegexConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "scheduler":
		var getcfg config.SchedulerConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	default:
		sendBadRequest(ctx, "Unknown config type: "+left)
		return
	}

	if err != nil {
		sendBadRequest(ctx, "Failed to update config: "+err.Error())
		return
	}

	reloadRuntimeFor(name)
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      List configuration sections of a type
// @Description  Returns all configuration sections of one type, e.g. every quality profile or every downloader.
// @Tags         config
// @Param        type  path      string  true  "Type Name: ex. quality"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=Jsonconfigall}  "Entries of the section, keyed like /api/config/all"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/config/type/{type} [get].
func apiListConfigType(ctx *gin.Context) {
	list := make(map[string]any)

	name, ok := getParamID(ctx, "type")
	if !ok {
		return
	}

	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}

	switch left {
	case "general":
		list["general"] = config.GetSettingsGeneral()
	case logger.StrImdb:
		list[logger.StrImdb] = config.GetSettingsImdb()
	case "downloader":
		config.RangeSettingsDownloader(func(key string, cfgdata *config.DownloaderConfig) {
			if strings.HasPrefix(key, right) {
				list["downloader_"+key] = cfgdata
			}
		})

	case "indexer":
		config.RangeSettingsIndexer(func(key string, cfgdata *config.IndexersConfig) {
			if strings.HasPrefix(key, right) {
				list["indexer_"+key] = cfgdata
			}
		})

	case "list":
		config.RangeSettingsList(func(key string, cfgdata *config.ListsConfig) {
			if strings.HasPrefix(key, right) {
				list["list_"+key] = cfgdata
			}
		})

	case logger.StrSerie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) error {
			if strings.HasPrefix(key, right) {
				list["serie_"+key] = cfgdata
			}

			return nil
		})

	case logger.StrMovie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) error {
			if strings.HasPrefix(key, right) {
				list["movie_"+key] = cfgdata
			}

			return nil
		})

	case "notification":
		config.RangeSettingsNotification(func(key string, cfgdata *config.NotificationConfig) {
			if strings.HasPrefix(key, right) {
				list["notification_"+key] = cfgdata
			}
		})

	case "path":
		config.RangeSettingsPath(func(key string, cfgdata *config.PathsConfig) {
			if strings.HasPrefix(key, right) {
				list["path_"+key] = cfgdata
			}
		})

	case "quality":
		config.RangeSettingsQuality(func(key string, cfgdata *config.QualityConfig) {
			if strings.HasPrefix(key, right) {
				list["quality_"+key] = cfgdata
			}
		})

	case "regex":
		config.RangeSettingsRegex(func(key string, cfgdata *config.RegexConfig) {
			if strings.HasPrefix(key, right) {
				list["regex_"+key] = cfgdata
			}
		})

	case "scheduler":
		config.RangeSettingsScheduler(func(key string, cfgdata *config.SchedulerConfig) {
			if strings.HasPrefix(key, right) {
				list["scheduler_"+key] = cfgdata
			}
		})
	}

	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(list))
}
