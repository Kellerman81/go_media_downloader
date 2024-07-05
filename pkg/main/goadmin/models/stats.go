package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/parameter"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func getStatsTable(ctx *context.Context) (userTable table.Table) {
	userTable = table.NewDefaultTable(ctx, table.Config{})
	userTable.GetOnlyInfo()
	//typ #List #count #missing #reached

	var stats []map[string]any
	id := 0
	lists := database.GetrowsN[string](false, 5, "select distinct listname from movies where length(listname) >= 1")
	for idx := range lists {
		all := database.GetdatarowN[int](false, "select count(*) from movies where listname = ? COLLATE NOCASE", &lists[idx])
		missing := database.GetdatarowN[int](false, "select count(*) from movies where listname = ? COLLATE NOCASE and missing=1", &lists[idx])
		reached := database.GetdatarowN[int](false, "select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=1", &lists[idx])
		upgrade := database.GetdatarowN[int](false, "select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=0 and missing=0", &lists[idx])
		stats = append(stats, map[string]any{"id": id, "typ": "movies", "list": lists[idx], "total": all, "missing": missing, "finished": reached, "upgrade": upgrade})
		id++
	}
	lists = database.GetrowsN[string](false, 5, "select distinct listname from series where length(listname) >= 1")
	for idx := range lists {
		all := database.GetdatarowN[int](false, "select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &lists[idx])
		missing := database.GetdatarowN[int](false, "select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and missing=1", &lists[idx])
		reached := database.GetdatarowN[int](false, "select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=1", &lists[idx])
		upgrade := database.GetdatarowN[int](false, "select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=0 and missing=0", &lists[idx])
		stats = append(stats, map[string]any{"id": id, "typ": "episodes", "list": lists[idx], "total": all, "missing": missing, "finished": reached, "upgrade": upgrade})
		id++
	}
	info := userTable.GetInfo().SetDefaultPageSize(100)
	info.HideDeleteButton().HideDetailButton().HideEditButton().HideExportButton().HideFilterArea().HideFilterButton().HideNewButton().HidePagination()
	info.AddField("ID", "id", db.Numeric)
	info.AddField("Typ", "typ", db.Varchar)
	info.AddField("List", "list", db.Varchar)
	info.AddField("Total", "total", db.Int)
	info.AddField("Missing", "missing", db.Int)
	info.AddField("Finished", "finished", db.Int)
	info.AddField("Upgradable", "upgrade", db.Int)

	info.SetTable("Stats").
		SetTitle("Stats").
		SetDescription("Stats").
		SetGetDataFn(func(param parameter.Parameters) (data []map[string]any, size int) {
			param.PK()
			return stats, len(stats)
		})
	return userTable
}
