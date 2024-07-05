package goadmin

import (
	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	"github.com/GoAdminGroup/go-admin/engine"
	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/sqlite" // sql driver
	"github.com/GoAdminGroup/go-admin/modules/language"
	"github.com/GoAdminGroup/themes/adminlte"
	"github.com/Kellerman81/go_media_downloader/pkg/main/goadmin/goadmindb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/goadmin/models"
	"github.com/Kellerman81/go_media_downloader/pkg/main/goadmin/pages"
	"github.com/gin-gonic/gin"
)

func Init(router *gin.Engine) {
	//template.AddComp(chartjs.NewChart())
	eng := engine.Default()
	acfg := config.Config{
		Databases: config.DatabaseList{
			"default": {Driver: "sqlite", File: "./databases/admin.db"},
		},
		AppID:                    "PPbLwfSG2Cwa",
		Theme:                    "adminlte",
		UrlPrefix:                "/admin",
		Env:                      config.EnvLocal,
		Debug:                    true,
		Language:                 language.EN,
		Title:                    "Go Media Downloader",
		LoginTitle:               "Go Media Downloader",
		Logo:                     "<b>Go</b> Media Downloader",
		FooterInfo:               "Go Media Downloader by Kellerman81",
		IndexUrl:                 "/",
		GoModFilePath:            "",
		BootstrapFilePath:        "",
		LoginUrl:                 "/login",
		AssetRootPath:            "",
		AssetUrl:                 "",
		ColorScheme:              adminlte.ColorschemeSkinBlack,
		AccessLogPath:            "./logs/access.log",
		ErrorLogPath:             "./logs/error.log",
		InfoLogPath:              "./logs/info.log",
		HideConfigCenterEntrance: true,
		HideAppInfoEntrance:      true,
		HideToolEntrance:         true,
		HidePluginEntrance:       true,
		NoLimitLoginIP:           true,
	}
	if err := eng.AddConfig(&acfg).AddPlugins(eng.AdminPlugin()).AddGenerators(models.Generators).
		Use(router); err != nil {
		panic(err)
	}
	eng.HTML("GET", "/admin", pages.GetDashBoard)
	eng.HTML("GET", "/", pages.GetDashBoard)
	eng.HTML("GET", "/actions", pages.GetActionsPage)
	eng.Services["sqlite"] = goadmindb.GetSqliteDB().InitDB(map[string]config.Database{
		"default": {Driver: "sqlite", File: "./databases/admin.db"},
		"media":   {Driver: "sqlite"}})
	eng.Adapter.SetConnection(db.GetConnection(eng.Services))
	router.Static("/admin/uploads", "./temp")
}
