package dashboard_overview

import (
	"github.com/a-h/templ"
	dashboard_overview_ui "github.com/yazmeyaa/hosthalla/ui/widgets/dashboard_overview/ui"
)

type DashboardOverviewProps = dashboard_overview_ui.DashboardOverviewProps

var DashboardOverview = dashboard_overview_ui.DashboardOverview

func CSSClasses() []templ.CSSClass {
	return dashboard_overview_ui.CSSClasses()
}
