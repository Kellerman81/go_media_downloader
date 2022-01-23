package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/icon"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
	"github.com/GoAdminGroup/go-admin/template/types/form"
	"github.com/Kellerman81/go_media_downloader/config"
)

func GetDbmoviesTable(ctx *context.Context) table.Table {

	//dbmovies := table.NewDefaultTable(table.DefaultConfig().SetConnection("media"))
	//dbmovies := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))
	dbmovies := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := dbmovies.GetDetail()
	detail.AddField("Id", "id", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Created_at", "created_at", db.Datetime)
	detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Year", "year", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Adult", "adult", db.Numeric)
	detail.AddField("Budget", "budget", db.Integer)
	detail.AddField("Genres", "genres", db.Text)
	detail.AddField("Original_language", "original_language", db.Text)
	detail.AddField("Original_title", "original_title", db.Text)
	detail.AddField("Overview", "overview", db.Text)
	detail.AddField("Popularity", "popularity", db.Real)
	detail.AddField("Revenue", "revenue", db.Integer)
	detail.AddField("Runtime", "runtime", db.Integer)
	detail.AddField("Spoken_languages", "spoken_languages", db.Text)
	detail.AddField("Status", "status", db.Text)
	detail.AddField("Tagline", "tagline", db.Text)
	detail.AddField("Vote_average", "vote_average", db.Real).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Vote_count", "vote_count", db.Integer).FieldSortable()
	detail.AddField("Moviedb_id", "moviedb_id", db.Integer)
	detail.AddField("Imdb_id", "imdb_id", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	detail.AddField("Freebase_id", "freebase_id", db.Text)
	detail.AddField("Facebook_id", "facebook_id", db.Text)
	detail.AddField("Instagram_id", "instagram_id", db.Text)
	detail.AddField("Twitter_id", "twitter_id", db.Text)
	detail.AddField("Url", "url", db.Text)
	detail.AddField("Backdrop", "backdrop", db.Text)
	detail.AddField("Poster", "poster", db.Text)
	detail.AddField("Slug", "slug", db.Text)
	detail.AddField("Trakt_id", "trakt_id", db.Integer)
	detail.AddField("Release_date", "release_date", db.Datetime)
	detail.AddColumnButtons("Details", types.GetColumnButton("Titles", icon.Info,
		action.PopUpWithIframe("/admin/info/dbmovie_titles", "see more", action.IframeData{Src: "/admin/info/dbmovie_titles", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Movies", icon.Info,
		action.PopUpWithIframe("/admin/info/movies", "see more", action.IframeData{Src: "/admin/info/movies", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")))
	detail.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	info := dbmovies.GetInfo().HideFilterArea()
	info.AddField("Id", "id", db.Integer).FieldFilterable().FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Year", "year", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Adult", "adult", db.Numeric)
	// info.AddField("Budget", "budget", db.Integer)
	info.AddField("Genres", "genres", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Original_language", "original_language", db.Text)
	// info.AddField("Original_title", "original_title", db.Text)
	// info.AddField("Overview", "overview", db.Text)
	// info.AddField("Popularity", "popularity", db.Real)
	// info.AddField("Revenue", "revenue", db.Integer)
	// info.AddField("Runtime", "runtime", db.Integer)
	// info.AddField("Spoken_languages", "spoken_languages", db.Text)
	// info.AddField("Status", "status", db.Text)
	// info.AddField("Tagline", "tagline", db.Text)
	info.AddField("Vote_average", "vote_average", db.Real).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Vote_count", "vote_count", db.Integer).FieldSortable()
	//info.AddField("Moviedb_id", "moviedb_id", db.Integer)
	info.AddField("Imdb_id", "imdb_id", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	// info.AddField("Freebase_id", "freebase_id", db.Text)
	// info.AddField("Facebook_id", "facebook_id", db.Text)
	// info.AddField("Instagram_id", "instagram_id", db.Text)
	// info.AddField("Twitter_id", "twitter_id", db.Text)
	// info.AddField("Url", "url", db.Text)
	// info.AddField("Backdrop", "backdrop", db.Text)
	// info.AddField("Poster", "poster", db.Text)
	// info.AddField("Slug", "slug", db.Text)
	// info.AddField("Trakt_id", "trakt_id", db.Integer)
	// info.AddField("Release_date", "release_date", db.Datetime)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	info.AddColumnButtons("Details", types.GetColumnButton("Titles", icon.Info,
		action.PopUpWithIframe("/admin/info/dbmovie_titles", "see more", action.IframeData{Src: "/admin/info/dbmovie_titles", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Movies", icon.Info,
		action.PopUpWithIframe("/admin/info/movies", "see more", action.IframeData{Src: "/admin/info/movies", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Refresh", icon.Refresh,
		action.PopUpWithIframe("/admin/info/refresh", "see more", action.IframeData{Src: "/api/movies/refresh/{{.Id}}?apikey=" + cfg_general.WebApiKey}, "900px", "560px")))
	info.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	formList := dbmovies.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Year", "year", db.Integer, form.Number)
	formList.AddField("Adult", "adult", db.Numeric, form.Number)
	formList.AddField("Budget", "budget", db.Integer, form.Number)
	formList.AddField("Genres", "genres", db.Text, form.Text)
	formList.AddField("Original_language", "original_language", db.Text, form.Text)
	formList.AddField("Original_title", "original_title", db.Text, form.Text)
	formList.AddField("Overview", "overview", db.Text, form.RichText)
	formList.AddField("Popularity", "popularity", db.Real, form.Text)
	formList.AddField("Revenue", "revenue", db.Integer, form.Number)
	formList.AddField("Runtime", "runtime", db.Integer, form.Number)
	formList.AddField("Spoken_languages", "spoken_languages", db.Text, form.Text)
	formList.AddField("Status", "status", db.Text, form.Text)
	formList.AddField("Tagline", "tagline", db.Text, form.RichText)
	formList.AddField("Vote_average", "vote_average", db.Real, form.Text)
	formList.AddField("Vote_count", "vote_count", db.Integer, form.Number)
	formList.AddField("Moviedb_id", "moviedb_id", db.Integer, form.Number)
	formList.AddField("Imdb_id", "imdb_id", db.Text, form.Text)
	formList.AddField("Freebase_m_id", "freebase_m_id", db.Text, form.Text)
	formList.AddField("Freebase_id", "freebase_id", db.Text, form.Text)
	formList.AddField("Facebook_id", "facebook_id", db.Text, form.Text)
	formList.AddField("Instagram_id", "instagram_id", db.Text, form.Text)
	formList.AddField("Twitter_id", "twitter_id", db.Text, form.Text)
	formList.AddField("Url", "url", db.Text, form.Text)
	formList.AddField("Backdrop", "backdrop", db.Text, form.Text)
	formList.AddField("Poster", "poster", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Trakt_id", "trakt_id", db.Integer, form.Number)
	formList.AddField("Release_date", "release_date", db.Datetime, form.Datetime)

	formList.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	return dbmovies
}
