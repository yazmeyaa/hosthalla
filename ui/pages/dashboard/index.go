package dashboard

import (
	"github.com/a-h/templ"
	dashboard_ui "github.com/yazmeyaa/hosthalla/ui/pages/dashboard/ui"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/badge"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/card"
	dashboard_hosts "github.com/yazmeyaa/hosthalla/ui/widgets/dashboard_hosts"
	dashboard_overview "github.com/yazmeyaa/hosthalla/ui/widgets/dashboard_overview"
)

type DashboardData = dashboard_ui.DashboardData
type DashboardSummary = dashboard_ui.DashboardSummary
type DashboardHostRow = dashboard_ui.DashboardHostRow
type DashboardPageProps = dashboard_ui.DashboardPageProps

var DashboardPage = dashboard_ui.DashboardPage
var DashboardPageContent = dashboard_ui.DashboardPageContent
var DashboardLiveUpdate = dashboard_ui.DashboardLiveUpdate
var DashboardGeneratedAtLiveUpdate = dashboard_ui.DashboardGeneratedAtLiveUpdate
var DashboardOverviewLiveUpdate = dashboard_ui.DashboardOverviewLiveUpdate
var DashboardHostsLiveUpdate = dashboard_ui.DashboardHostsLiveUpdate
var DashboardHostRowLiveUpdate = dashboard_ui.DashboardHostRowLiveUpdate

func CSSClasses() []templ.CSSClass {
	classes := dashboard_ui.CSSClasses()
	classes = append(classes, dashboard_overview.CSSClasses()...)
	classes = append(classes, dashboard_hosts.CSSClasses()...)
	classes = append(classes, card.CSSClasses()...)
	classes = append(classes, badge.CSSClasses()...)
	return classes
}
