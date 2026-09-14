package apiexternal

// Nzb and Nzbwithprio (formerly defined here) were superseded package-wide by
// apiexternal_v2.Nzb / apiexternal_v2.Nzbwithprio and have been removed. The
// only remaining reference was api/all.go's Swagger-doc-only Jsonresults
// struct, which now uses apiexternal_v2.Nzbwithprio instead.

// saveAttributes populates the fields of the NZB struct from
// the name/value pairs passed in. It handles translating the
// values to the appropriate types for the NZB struct fields.
// func (n *Nzb) saveAttributes(name, value string) {
// 	switch name {
// 	case strtitle:
// 		n.Title = value
// 	case strlink, "url":
// 		n.DownloadURL = value
// 	case strguid:
// 		n.ID = value
// 	case "tvdbid":
// 		n.TVDBID = logger.StringToInt(value)
// 	case logger.StrImdb:
// 		n.IMDBID = logger.AddImdbPrefix(value)
// 	case "season":
// 		n.Season = value
// 	case "episode":
// 		n.Episode = value
// 	case strsize:
// 		n.Size = logger.StringToInt64(value)
// 	}
// }

// setfield sets the corresponding field in the Nzb struct based on the provided field name and value.
// If the field is already set, it will not be overwritten.
// The supported fields are:
// - Title
// - DownloadURL
// - ID
// - Size
// - IMDBID
// - TVDBID
// - Season
// - Episode.
// func (n *Nzb) setfield(field string, value []byte) {
// 	var shouldSet bool
// 	switch field {
// 	case strtitle:
// 		shouldSet = n.Title == ""
// 	case strlink, "url":
// 		shouldSet = n.DownloadURL == ""
// 	case strguid:
// 		shouldSet = n.ID == ""
// 	case strsize, "length":
// 		shouldSet = n.Size == 0
// 	case logger.StrImdb:
// 		shouldSet = n.IMDBID == ""
// 	case "tvdbid":
// 		shouldSet = n.TVDBID == 0
// 	case "season":
// 		shouldSet = n.Season == ""
// 	case "episode":
// 		shouldSet = n.Episode == ""
// 	default:
// 		return
// 	}
// 	if shouldSet {
// 		n.saveAttributes(field, string(value))
// 	}
// }

// setfieldstr sets the specified field of the Nzb struct to the provided value,
// but only if the field is not already set. The supported fields are:
// - Title
// - DownloadURL
// - ID
// - Size
// - IMDBID
// - TVDBID
// - Season
// - Episode.
// func (n *Nzb) setfieldstr(field string, value string) {
// 	var shouldSet bool
// 	switch field {
// 	case strtitle:
// 		shouldSet = n.Title == ""
// 	case strlink, "url":
// 		shouldSet = n.DownloadURL == ""
// 	case strguid:
// 		shouldSet = n.ID == ""
// 	case strsize, "length":
// 		shouldSet = n.Size == 0
// 	case logger.StrImdb:
// 		shouldSet = n.IMDBID == ""
// 	case "tvdbid":
// 		shouldSet = n.TVDBID == 0
// 	case "season":
// 		shouldSet = n.Season == ""
// 	case "episode":
// 		shouldSet = n.Episode == ""
// 	default:
// 		return
// 	}
// 	if shouldSet {
// 		n.saveAttributes(field, value)
// 	}
// }

// generateIdentifierStringFromInt generates a season/episode identifier string
// from the given season and episode integers. It pads each number with leading
// zeros to ensure a consistent format like "S01E02". This is intended to generate
// identifiers for public display/logging.
func generateIdentifierStringFromInt(season, episode *int) string {
	return ("S" + padNumberWithZero(season) + "E" + padNumberWithZero(episode))
}
