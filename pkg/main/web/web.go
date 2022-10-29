package web

import "github.com/maxence-charriere/go-app/v9/pkg/app"

type Home struct {
	app.Compo
}

type bla struct {
	Str string
	Num int
}

func (h *Home) Render() app.UI {
	a := []bla{}
	a = append(a, bla{Str: "ttt", Num: 1})
	a = append(a, bla{Str: "sss", Num: 2})
	return app.Table().Body(
		app.THead().Body(
			app.Tr().Aria("role", "row").Body(
				app.Th().Aria("role", "columnheader").Scope("col").Body(
					app.I().Class("fas fa-plug pf-u-mr-xs").Aria("hidden", false),
					app.Text("String"),
				),
				app.Th().Aria("role", "columnheader").Scope("col").Body(
					app.I().Class("fas fa-plug pf-u-mr-xs").Aria("hidden", false),
					app.Text("ID"),
				),
			),
		),
		app.TBody().Class("pf-x-u-border-t-0").Aria("role", "rowgroup").Body(
			app.Range(a).Slice(func(i int) app.UI {
				return app.Tr().
					Class(func() string {
						classes := "pf-m-hoverable"
						return classes
					}()).
					Aria("role", "row").
					Body(
						app.Td().
							Aria("role", "cell").
							DataSet("label", "String").
							Body(
								app.Text(a[i].Str),
							),
						app.Td().
							Aria("role", "cell").
							DataSet("label", "ID").
							Body(
								app.Text(a[i].Num),
							),
					)
			}),
		),
	)
}
